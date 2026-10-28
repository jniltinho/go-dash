// Package cmd defines the dashboard command-line interface.
package cmd

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/jniltinho/go-dash/internal/config"
	"github.com/jniltinho/go-dash/internal/database"
)

// Build metadata, injected with -ldflags "-X github.com/jniltinho/go-dash/cmd.Version=...".
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// app carries state shared by all commands of one invocation.
type app struct {
	v       *viper.Viper
	cfgFile string
	assets  embed.FS
	cfg     config.Config
	logger  *slog.Logger
}

// Execute runs the CLI and returns the process exit code.
func Execute(assets embed.FS) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := newRootCmd(assets)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(root.ErrOrStderr(), "Error:", err)
		return 1
	}
	return 0
}

func newRootCmd(assets embed.FS) *cobra.Command {
	a := &app{v: viper.New(), assets: assets}

	root := &cobra.Command{
		Use:           "go-dash",
		Short:         "Link aggregator dashboard with menus, submenus and tabbed iframes",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return a.init(cmd)
		},
	}

	flags := root.PersistentFlags()
	flags.StringVar(&a.cfgFile, "config", "", "TOML config file (default ./dashboard.toml when present)")
	flags.String("db", "./dashboard.db", "SQLite database path")
	flags.String("log-level", "info", "log level: debug, info, warn, error")
	flags.String("log-format", "text", "log format: text or json")
	bindFlags(a.v, flags.Lookup("db"), flags.Lookup("log-level"), flags.Lookup("log-format"))

	root.AddCommand(
		newServeCmd(a),
		newMigrateCmd(a),
		newMenuCmd(a),
		newDashCmd(a),
		newAdminCmd(a),
		newBackupCmd(a),
		newVersionCmd(),
	)
	return root
}

func (a *app) init(cmd *cobra.Command) error {
	if err := config.Init(a.v, a.cfgFile); err != nil {
		return err
	}
	cfg, err := config.Load(a.v)
	if err != nil {
		return err
	}
	a.cfg = cfg
	a.logger = newLogger(cmd, cfg)
	slog.SetDefault(a.logger)
	return nil
}

func newLogger(cmd *cobra.Command, cfg config.Config) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(cfg.LogFormat, "json") {
		return slog.New(slog.NewJSONHandler(cmd.ErrOrStderr(), opts))
	}
	return slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), opts))
}

// openDB opens the configured database and applies pending migrations.
func (a *app) openDB(ctx context.Context) (*sql.DB, error) {
	return database.OpenAndMigrate(ctx, a.cfg.DB)
}

// distFS returns the frontend build: from disk in dev mode, embedded otherwise.
func (a *app) distFS() (fs.FS, error) {
	if a.cfg.Dev {
		return os.DirFS(a.cfg.DistDir), nil
	}
	sub, err := fs.Sub(a.assets, "web/dist")
	if err != nil {
		return nil, fmt.Errorf("opening embedded assets: %w", err)
	}
	return sub, nil
}

// bindFlags binds flags to viper keys, converting dashes to underscores
// (--public-url -> public_url -> DASHBOARD_PUBLIC_URL).
func bindFlags(v *viper.Viper, flags ...*pflag.Flag) {
	for _, f := range flags {
		if err := v.BindPFlag(strings.ReplaceAll(f.Name, "-", "_"), f); err != nil {
			panic(fmt.Sprintf("binding flag %q: %v", f.Name, err))
		}
	}
}
