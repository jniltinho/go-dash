// Package config resolves application settings with the precedence
// flags > DASHBOARD_* environment variables > TOML file > defaults.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

// EnvPrefix is the prefix for environment variables (DASHBOARD_ADDR, ...).
const EnvPrefix = "DASHBOARD"

// DefaultSandbox is the default iframe sandbox attribute.
const DefaultSandbox = "allow-scripts allow-same-origin allow-forms allow-popups"

// forbiddenSandboxTokens would let embedded pages replace the dashboard
// window or open unsandboxed popups.
var forbiddenSandboxTokens = map[string]bool{
	"allow-top-navigation":                     true,
	"allow-top-navigation-by-user-activation":  true,
	"allow-top-navigation-to-custom-protocols": true,
	"allow-popups-to-escape-sandbox":           true,
}

// Config holds every runtime setting.
type Config struct {
	Addr              string `mapstructure:"addr"`
	DB                string `mapstructure:"db"`
	PublicURL         string `mapstructure:"public_url"`
	Title             string `mapstructure:"title"`
	Dev               bool   `mapstructure:"dev"`
	DistDir           string `mapstructure:"dist_dir"`
	TLSCert           string `mapstructure:"tls_cert"`
	TLSKey            string `mapstructure:"tls_key"`
	CookieSecure      bool   `mapstructure:"cookie_secure"`
	TrustProxy        bool   `mapstructure:"trust_proxy"`
	FrameSrc          string `mapstructure:"frame_src"`
	IframeSandbox     string `mapstructure:"iframe_sandbox"`
	MaxTabs           int    `mapstructure:"max_tabs"`
	LogLevel          string `mapstructure:"log_level"`
	LogFormat         string `mapstructure:"log_format"`
	AdminPassword     string `mapstructure:"admin_password"`
	AdminPasswordHash string `mapstructure:"admin_password_hash"`
	Seed              string `mapstructure:"seed"`
	LoginRate         int    `mapstructure:"login_rate"`
	DefaultTheme      string `mapstructure:"default_theme"`
	DefaultThemeStyle string `mapstructure:"default_theme_style"`
	// BasePath is the URL prefix the application is served under behind a
	// reverse proxy; always starts and ends with "/" after Load.
	BasePath string `mapstructure:"base_path"`
}

// themeName matches theme IDs (the theme list itself lives in the frontend
// registry; unknown names fall back to the frontend default).
var themeName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,40}$`)

// SetDefaults registers default values on v.
func SetDefaults(v *viper.Viper) {
	v.SetDefault("addr", ":8080")
	v.SetDefault("db", "./dashboard.db")
	v.SetDefault("public_url", "")
	v.SetDefault("title", "Go Dash")
	v.SetDefault("dev", false)
	v.SetDefault("dist_dir", "web/dist")
	v.SetDefault("tls_cert", "")
	v.SetDefault("tls_key", "")
	v.SetDefault("cookie_secure", false)
	v.SetDefault("trust_proxy", false)
	v.SetDefault("frame_src", "http: https:")
	v.SetDefault("iframe_sandbox", DefaultSandbox)
	v.SetDefault("max_tabs", 10)
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "text")
	v.SetDefault("admin_password", "")
	v.SetDefault("admin_password_hash", "")
	v.SetDefault("seed", "")
	v.SetDefault("login_rate", 5)
	v.SetDefault("default_theme", "ocean-light")
	v.SetDefault("default_theme_style", "flat")
	v.SetDefault("base_path", "/")
}

// Init configures env binding and reads the TOML file. When file is empty,
// ./dashboard.toml is used if present; an explicit missing file is an error.
func Init(v *viper.Viper, file string) error {
	SetDefaults(v)
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()
	v.SetConfigType("toml")

	if file != "" {
		v.SetConfigFile(file)
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("reading config %q: %w", file, err)
		}
		return nil
	}

	if _, err := os.Stat("dashboard.toml"); err == nil {
		v.SetConfigFile("dashboard.toml")
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("reading dashboard.toml: %w", err)
		}
	}
	return nil
}

// Load unmarshals and validates the configuration.
func Load(v *viper.Viper) (Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decoding config: %w", err)
	}
	cfg.BasePath = NormalizeBasePath(cfg.BasePath)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks settings that would otherwise fail late.
func (c Config) Validate() error {
	if c.DB == "" {
		return errors.New("db path must not be empty")
	}
	if c.MaxTabs < 1 || c.MaxTabs > 50 {
		return fmt.Errorf("max_tabs must be between 1 and 50, got %d", c.MaxTabs)
	}
	if !themeName.MatchString(c.DefaultTheme) {
		return fmt.Errorf("default_theme %q is not a valid theme name", c.DefaultTheme)
	}
	if c.DefaultThemeStyle != "flat" && c.DefaultThemeStyle != "3d" {
		return fmt.Errorf("default_theme_style must be \"flat\" or \"3d\", got %q", c.DefaultThemeStyle)
	}
	if c.LoginRate < 1 || c.LoginRate > 1000 {
		return fmt.Errorf("login_rate must be between 1 and 1000, got %d", c.LoginRate)
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return errors.New("tls_cert and tls_key must be set together")
	}
	if strings.ContainsAny(c.FrameSrc, ";\r\n") {
		return errors.New("frame_src must not contain ';' or line breaks")
	}
	if strings.ContainsAny(c.IframeSandbox, "\"<>\r\n") {
		return errors.New("iframe_sandbox contains invalid characters")
	}
	for _, token := range strings.Fields(c.IframeSandbox) {
		if forbiddenSandboxTokens[strings.ToLower(token)] {
			return fmt.Errorf("iframe_sandbox token %q is not allowed: embedded pages must not navigate or escape the dashboard", token)
		}
	}
	if err := ValidateBasePath(c.BasePath); err != nil {
		return err
	}
	if path := publicURLPath(c.PublicURL); path != "/" && path != NormalizeBasePath(c.BasePath) {
		return fmt.Errorf("public_url path %q must equal base_path %q (or be empty)", path, c.BasePath)
	}
	return nil
}

const maxBasePathLength = 100

var basePathSegment = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)

// reservedRootNames are served by the application at its root: a base path
// starting with one of them could not be told apart from a stripped request.
// Other files of the frontend build are checked by `serve` against its assets.
var reservedRootNames = map[string]bool{
	"api": true, "admin": true, "assets": true, "dash": true, "healthz": true,
	"index.html": true, "favicon.svg": true,
}

// NormalizeBasePath adds the leading and trailing slash ("panel" -> "/panel/").
func NormalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// ValidateBasePath checks a normalized base path ("" counts as "/").
func ValidateBasePath(p string) error {
	if p == "" || p == "/" {
		return nil
	}
	if len(p) > maxBasePathLength {
		return fmt.Errorf("base_path must have at most %d characters", maxBasePathLength)
	}
	// Checked before trimming: "//host/" would make redirects and <base href>
	// point to another host.
	if strings.Contains(p, "//") {
		return fmt.Errorf("base_path %q is invalid: it must not contain \"//\"", p)
	}
	segments := strings.Split(strings.Trim(p, "/"), "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." || !basePathSegment.MatchString(seg) {
			return fmt.Errorf("base_path %q is invalid: use segments of letters, digits, '.', '_', '~' or '-' (no '.', '..', '//' or encoded characters)", p)
		}
	}
	if IsReservedRootName(segments[0]) {
		return fmt.Errorf("base_path %q must not start with %q: the application serves that name at its root", p, segments[0])
	}
	return nil
}

// IsReservedRootName reports whether name is served by the application at its root.
func IsReservedRootName(name string) bool {
	return reservedRootNames[strings.ToLower(name)]
}

// publicURLPath returns the normalized path of public_url ("/" when empty or invalid).
func publicURLPath(raw string) string {
	if raw == "" {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" || u.Path == "/" {
		return "/"
	}
	return NormalizeBasePath(u.Path)
}

// TLSEnabled reports whether the server terminates TLS itself.
func (c Config) TLSEnabled() bool {
	return c.TLSCert != "" && c.TLSKey != ""
}
