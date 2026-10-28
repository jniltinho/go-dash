package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jniltinho/go-dash/internal/auth"
	"github.com/jniltinho/go-dash/internal/config"
	"github.com/jniltinho/go-dash/internal/menu"
	"github.com/jniltinho/go-dash/internal/server"
)

func newServeCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			db, err := a.openDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			menuSvc := menu.NewService(db)
			if err := seedIfEmpty(cmd, a, menuSvc); err != nil {
				return err
			}

			mgr, err := auth.NewManager(db, auth.Options{
				EnvPasswordHash: a.cfg.AdminPasswordHash,
				EnvPassword:     a.cfg.AdminPassword,
			})
			if err != nil {
				return err
			}
			if src, err := mgr.Source(ctx); err == nil {
				switch src {
				case auth.SourceNone:
					a.logger.Warn("no admin password configured; web administration is disabled")
				case auth.SourceEnvPlain:
					a.logger.Warn("using plain-text DASHBOARD_ADMIN_PASSWORD; prefer DASHBOARD_ADMIN_PASSWORD_HASH")
				}
			}

			assets, err := a.distFS()
			if err != nil {
				return err
			}
			if err := server.CheckBasePath(assets, a.cfg.BasePath); err != nil {
				return err
			}

			srv := server.New(server.Deps{
				Config: a.cfg,
				DB:     db,
				Menu:   menuSvc,
				Auth:   mgr,
				Assets: assets,
				Logger: a.logger,
				Build:  server.BuildInfo{Version: Version, Commit: GitCommit, BuildDate: BuildDate},
			})
			return srv.Run(ctx)
		},
	}

	f := cmd.Flags()
	f.String("addr", ":8080", "listen address")
	f.String("public-url", "", "public URL of the dashboard (used for same-origin checks)")
	f.String("title", "Go Dash", "title shown in the toolbar")
	f.Bool("dev", false, "serve frontend files from --dist-dir on disk instead of the embedded build")
	f.String("dist-dir", "web/dist", "frontend build directory used with --dev")
	f.String("tls-cert", "", "TLS certificate file")
	f.String("tls-key", "", "TLS key file")
	f.Bool("cookie-secure", false, "always set the Secure flag on the session cookie")
	f.Bool("trust-proxy", false, "trust X-Forwarded-For / X-Forwarded-Proto from a reverse proxy")
	f.String("frame-src", "http: https:", "CSP frame-src sources allowed in iframes")
	f.String("iframe-sandbox", config.DefaultSandbox, "iframe sandbox tokens")
	f.Int("max-tabs", 10, "maximum number of open item tabs")
	f.Int("login-rate", 5, "login attempts allowed per minute per IP")
	f.String("default-theme", "ocean-light", "theme for visitors without a saved choice")
	f.String("default-theme-style", "flat", "style for visitors without a saved choice: flat or 3d")
	f.String("seed", "", "menu JSON (as written by `menu export`) imported on start when the menu is empty")
	f.String("base-path", "/", "URL prefix when served under a sub-path by a reverse proxy, e.g. /panel/")
	for _, name := range []string{
		"addr", "public-url", "title", "dev", "dist-dir", "tls-cert", "tls-key",
		"cookie-secure", "trust-proxy", "frame-src", "iframe-sandbox", "max-tabs", "seed", "login-rate", "default-theme", "default-theme-style",
		"base-path",
	} {
		bindFlags(a.v, f.Lookup(name))
	}
	return cmd
}

// seedIfEmpty imports the configured seed file when the default dashboard has no items yet.
func seedIfEmpty(cmd *cobra.Command, a *app, svc *menu.Service) error {
	if a.cfg.Seed == "" {
		return nil
	}
	n, err := seedMenu(cmd.Context(), svc, a.cfg.Seed, a.cliValidateOptions())
	if err != nil {
		return err
	}
	if n > 0 {
		a.logger.Info("menu seeded", "file", a.cfg.Seed, "items", n)
	}
	return nil
}

// seedMenu imports path into the default dashboard when it is empty and
// returns the number of imported items; other dashboards are not considered.
func seedMenu(ctx context.Context, svc *menu.Service, path string, opts menu.ValidateOptions) (int, error) {
	items, err := svc.List(ctx, menu.DefaultDashboardID)
	if err != nil {
		return 0, err
	}
	if len(items) > 0 {
		return 0, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading seed %q: %w", path, err)
	}
	var doc menu.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return 0, fmt.Errorf("parsing seed %q: %w", path, err)
	}
	n, err := svc.Import(ctx, menu.DefaultDashboardID, doc, menu.ImportAppend, opts)
	if err != nil {
		return 0, fmt.Errorf("importing seed %q: %w", path, err)
	}
	return n, nil
}
