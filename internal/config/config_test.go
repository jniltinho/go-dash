package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestPrecedence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "dashboard.toml")
	if err := os.WriteFile(file, []byte("addr = \":6060\"\nmax_tabs = 5\ntitle = \"From file\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DASHBOARD_ADDR", ":9090")
	t.Setenv("DASHBOARD_MAX_TABS", "7")

	v := viper.New()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("addr", ":8080", "")
	if err := flags.Parse([]string{"--addr", ":7070"}); err != nil {
		t.Fatal(err)
	}
	if err := v.BindPFlag("addr", flags.Lookup("addr")); err != nil {
		t.Fatal(err)
	}
	if err := Init(v, file); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":7070" {
		t.Errorf("flag should win: addr = %q", cfg.Addr)
	}
	if cfg.MaxTabs != 7 {
		t.Errorf("env should beat file: max_tabs = %d", cfg.MaxTabs)
	}
	if cfg.Title != "From file" {
		t.Errorf("file should beat default: title = %q", cfg.Title)
	}
	if cfg.DB != "./dashboard.db" {
		t.Errorf("default db = %q", cfg.DB)
	}
	if cfg.DefaultTheme != "ocean-light" || cfg.DefaultThemeStyle != "flat" {
		t.Errorf("default theme = %q/%q", cfg.DefaultTheme, cfg.DefaultThemeStyle)
	}
}

func TestEnvOverridesDefault(t *testing.T) {
	t.Setenv("DASHBOARD_ADDR", ":9090")
	v := viper.New()
	if err := Init(v, ""); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9090" {
		t.Fatalf("addr = %q", cfg.Addr)
	}
}

func TestValidate(t *testing.T) {
	base := Config{DB: "x.db", MaxTabs: 10, LoginRate: 5, DefaultTheme: "ocean-light", DefaultThemeStyle: "flat"}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.TLSCert = "cert.pem"
	if bad.Validate() == nil {
		t.Fatal("tls cert without key must fail")
	}
	bad = base
	bad.DefaultTheme = "<script>"
	if bad.Validate() == nil {
		t.Fatal("invalid default_theme must be rejected")
	}
	bad = base
	bad.DefaultThemeStyle = "glossy"
	if bad.Validate() == nil {
		t.Fatal("invalid default_theme_style must be rejected")
	}
	bad = base
	bad.LoginRate = 0
	if bad.Validate() == nil {
		t.Fatal("login_rate 0 must be rejected")
	}
	bad = base
	bad.IframeSandbox = "allow-scripts allow-top-navigation"
	if bad.Validate() == nil {
		t.Fatal("allow-top-navigation must be rejected")
	}
	bad = base
	bad.FrameSrc = "https:; script-src *"
	if bad.Validate() == nil {
		t.Fatal("frame_src injection must fail")
	}
}

func TestBasePath(t *testing.T) {
	tests := []struct {
		name, value, publicURL, want string
		wantErr                      bool
	}{
		{name: "default", value: "/", want: "/"},
		{name: "adds slashes", value: "panel", want: "/panel/"},
		{name: "nested", value: "/ops/dash/", want: "/ops/dash/"},
		{name: "public url with same path", value: "/panel/", publicURL: "https://host/panel", want: "/panel/"},
		{name: "public url without path", value: "/panel/", publicURL: "https://host", want: "/panel/"},
		{name: "reserved admin", value: "/admin/", wantErr: true},
		{name: "reserved api nested", value: "/api/x/", wantErr: true},
		{name: "reserved root file", value: "/favicon.svg/", wantErr: true},
		{name: "dot dot", value: "/a/../b/", wantErr: true},
		{name: "single dot", value: "/ops/./dash/", wantErr: true},
		{name: "double slash", value: "/a//b/", wantErr: true},
		{name: "protocol relative", value: "//example.com/", wantErr: true},
		{name: "leading double slash", value: "//panel", wantErr: true},
		{name: "trailing double slash", value: "/panel//", wantErr: true},
		{name: "encoded", value: "/pai%6Eel/", wantErr: true},
		{name: "query", value: "/panel?x/", wantErr: true},
		{name: "too long", value: "/" + strings.Repeat("a", 100) + "/", wantErr: true},
		{name: "public url with other path", value: "/panel/", publicURL: "https://host/other/", wantErr: true},
		{name: "public url path without base", value: "/", publicURL: "https://host/panel/", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			if err := Init(v, ""); err != nil {
				t.Fatal(err)
			}
			v.Set("base_path", tt.value)
			v.Set("public_url", tt.publicURL)
			cfg, err := Load(v)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if !strings.Contains(err.Error(), "base_path") {
					t.Fatalf("error does not name base_path: %v", err)
				}
				return
			}
			if cfg.BasePath != tt.want {
				t.Fatalf("BasePath = %q, want %q", cfg.BasePath, tt.want)
			}
		})
	}
}
