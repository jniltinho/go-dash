// Package server wires the HTTP API, security middleware and the embedded
// single-page application on top of Echo v5.
package server

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/jniltinho/go-dash/internal/auth"
	"github.com/jniltinho/go-dash/internal/config"
	"github.com/jniltinho/go-dash/internal/menu"
)

const (
	maxAdminBody     = 1 << 20 // 1 MiB
	sessionCookie    = "dashboard_session"
	csrfHeader       = "X-CSRF-Token"
	shutdownTimeout  = 10 * time.Second
	sessionPurgeTick = 15 * time.Minute
)

// BuildInfo describes the running binary.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// Deps are the server dependencies.
type Deps struct {
	Config config.Config
	DB     *sql.DB
	Menu   *menu.Service
	Auth   *auth.Manager
	// Assets is the frontend build root (contains index.html).
	Assets fs.FS
	Logger *slog.Logger
	Build  BuildInfo
}

// Server is the HTTP application.
type Server struct {
	deps    Deps
	echo    *echo.Echo
	limiter *auth.RateLimiter
	// unlockLimiter counts dashboard unlock attempts apart from admin logins.
	unlockLimiter *auth.RateLimiter
	static        *staticHandler
	// base is the normalized base path ("/" or "/panel/").
	base string
}

// New builds the Echo application with all routes and middleware.
func New(deps Deps) *Server {
	ipExtractor := echo.ExtractIPDirect()
	if deps.Config.TrustProxy {
		ipExtractor = echo.ExtractIPFromXFFHeader()
	}

	e := echo.NewWithConfig(echo.Config{
		Logger:      deps.Logger,
		IPExtractor: ipExtractor,
	})
	base := config.NormalizeBasePath(deps.Config.BasePath)
	s := &Server{
		deps: deps,
		base: base,
		echo: e,
		// Per-IP login attempts per minute; the global cap is four times that.
		limiter:       auth.NewRateLimiter(deps.Config.LoginRate, deps.Config.LoginRate*4, time.Minute),
		unlockLimiter: auth.NewRateLimiter(deps.Config.LoginRate, deps.Config.LoginRate*4, time.Minute),
		static:        newStaticHandler(deps.Assets, deps.Config.Dev, base),
	}
	e.HTTPErrorHandler = s.handleError
	if base != "/" {
		e.Pre(s.stripBasePath)
	}

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(s.requestLogger())
	e.Use(s.securityHeaders)
	e.Use(middleware.Gzip())

	s.routes()
	return s
}

// Handler exposes the HTTP handler (used by tests).
func (s *Server) Handler() http.Handler {
	return s.echo
}

func (s *Server) routes() {
	e := s.echo

	e.GET("/healthz", s.health)

	api := e.Group("/api")
	api.GET("/config", s.publicConfig)
	// Legacy path of the default dashboard's menu.
	api.GET("/menu", s.publicMenu, s.useDefaultDashboard, s.requireDashboardAccess)

	dash := api.Group("/dash/:slug", noStore, s.resolvePublicDashboard)
	dash.GET("/status", s.dashboardStatus)
	dash.GET("/menu", s.publicMenu, s.requireDashboardAccess)
	dash.POST("/unlock", s.unlockDashboard, middleware.BodyLimit(maxUnlockBody))
	dash.POST("/lock", s.lockDashboard, middleware.BodyLimit(maxUnlockBody))

	// Admin responses depend on the session cookie: never cache them, errors included.
	admin := api.Group("/admin", noStore, middleware.BodyLimit(maxAdminBody))
	admin.GET("/session", s.sessionStatus)
	admin.POST("/login", s.login)

	authed := admin.Group("", s.requireSession, s.requireCSRF)
	authed.POST("/logout", s.logout)
	authed.GET("/dashboards", s.listDashboards)
	authed.POST("/dashboards", s.createDashboard)

	scoped := authed.Group("/dashboards/:dashboardID", s.resolveAdminDashboard)
	scoped.PUT("", s.updateDashboard)
	scoped.DELETE("", s.deleteDashboard)
	scoped.PUT("/password", s.setDashboardPassword)
	scoped.DELETE("/password", s.clearDashboardPassword)
	registerItemRoutes(scoped, s)

	// Legacy item routes act on the default dashboard.
	registerItemRoutes(authed.Group("", s.useDefaultDashboard), s)

	api.RouteNotFound("/*", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	})

	e.GET("/*", s.static.serve)
	e.HEAD("/*", s.static.serve)
}

// registerItemRoutes adds the item management routes to a group whose
// middleware has resolved the dashboard.
func registerItemRoutes(g *echo.Group, s *Server) {
	g.GET("/items", s.adminTree)
	g.POST("/items", s.createItem)
	g.PUT("/items/:itemID", s.updateItem)
	g.DELETE("/items/:itemID", s.deleteItem)
	g.POST("/items/reorder", s.reorderItems)
	g.GET("/export", s.exportItems)
	g.POST("/import", s.importItems)
}

// Run starts the server and blocks until ctx is cancelled, then shuts down
// gracefully, waiting up to shutdownTimeout for in-flight requests.
func (s *Server) Run(ctx context.Context) error {
	cfg := s.deps.Config
	log := s.deps.Logger

	go s.purgeSessions(ctx)

	if !cfg.TLSEnabled() && !cfg.CookieSecure {
		log.Warn("serving without HTTPS: the admin password travels in clear text unless a TLS proxy is used")
	}
	if !s.static.hasIndex() {
		log.Warn("frontend build not found; run `make frontend` before building", "dist", cfg.DistDir)
	}

	sc := echo.StartConfig{
		Address:         cfg.Addr,
		HideBanner:      true,
		GracefulTimeout: shutdownTimeout,
		OnShutdownError: func(err error) {
			log.Warn("graceful shutdown timed out; closing remaining connections", "error", err)
		},
		BeforeServeFunc: func(srv *http.Server) error {
			srv.ReadHeaderTimeout = 10 * time.Second
			srv.IdleTimeout = 120 * time.Second
			return nil
		},
	}

	log.Info("starting server", "addr", cfg.Addr, "tls", cfg.TLSEnabled(), "version", s.deps.Build.Version)

	var err error
	if cfg.TLSEnabled() {
		err = sc.StartTLS(ctx, s.echo, cfg.TLSCert, cfg.TLSKey)
	} else {
		err = sc.Start(ctx, s.echo)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("server stopped")
	return nil
}

func (s *Server) purgeSessions(ctx context.Context) {
	ticker := time.NewTicker(sessionPurgeTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.deps.Auth.PurgeExpired(ctx); err != nil && ctx.Err() == nil {
				s.deps.Logger.Warn("purging expired sessions failed", "error", err)
			}
		}
	}
}

func (s *Server) requestLogger() echo.MiddlewareFunc {
	log := s.deps.Logger
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogURIPath:   true,
		LogMethod:    true,
		LogLatency:   true,
		LogRemoteIP:  true,
		LogRequestID: true,
		HandleError:  true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			level := slog.LevelInfo
			switch {
			case v.Status >= 500:
				level = slog.LevelError
			case v.Status >= 400:
				level = slog.LevelWarn
			}
			log.LogAttrs(
				c.Request().Context(),
				level,
				"request",
				slog.String("method", v.Method),
				slog.String("path", v.URIPath),
				slog.Int("status", v.Status),
				slog.Duration("latency", v.Latency),
				slog.String("ip", v.RemoteIP),
				slog.String("request_id", v.RequestID),
			)
			return nil
		},
	})
}
