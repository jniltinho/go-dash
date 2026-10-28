package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jniltinho/go-dash/internal/auth"
	"github.com/jniltinho/go-dash/internal/menu"
)

func (s *Server) health(c *echo.Context) error {
	if err := s.deps.DB.PingContext(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

type publicConfigResponse struct {
	Title         string    `json:"title"`
	MaxTabs       int       `json:"max_tabs"`
	IframeSandbox string    `json:"iframe_sandbox"`
	PublicURL     string    `json:"public_url"`
	DefaultTheme  string    `json:"default_theme"`
	DefaultStyle  string    `json:"default_theme_style"`
	Build         BuildInfo `json:"build"`
}

func (s *Server) publicConfig(c *echo.Context) error {
	cfg := s.deps.Config
	return c.JSON(http.StatusOK, publicConfigResponse{
		Title:         cfg.Title,
		MaxTabs:       cfg.MaxTabs,
		IframeSandbox: cfg.IframeSandbox,
		PublicURL:     cfg.PublicURL,
		DefaultTheme:  cfg.DefaultTheme,
		DefaultStyle:  cfg.DefaultThemeStyle,
		Build:         s.deps.Build,
	})
}

// publicMenu serves the enabled tree of the dashboard resolved by the route
// middleware; requireDashboardAccess has already run.
func (s *Server) publicMenu(c *echo.Context) error {
	dash := currentDashboard(c)
	tree, err := s.deps.Menu.Tree(c.Request().Context(), dash.ID, true)
	if err != nil {
		return err
	}
	body, err := json.Marshal(tree)
	if err != nil {
		return fmt.Errorf("encoding menu: %w", err)
	}
	header := c.Response().Header()
	if dash.Protected {
		// Never cached or revalidated: a 304 must not outlive a revoked session.
		header.Set("Cache-Control", "private, no-store")
		return c.Blob(http.StatusOK, echo.MIMEApplicationJSON, body)
	}

	// Open dashboards poll this endpoint to pick up admin changes (items, card
	// order); the ETag turns unchanged polls into an empty 304.
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	header.Set("Cache-Control", "no-cache")
	header.Set("ETag", etag)
	if etagMatches(c.Request().Header.Get("If-None-Match"), etag) {
		return c.NoContent(http.StatusNotModified)
	}
	return c.Blob(http.StatusOK, echo.MIMEApplicationJSON, body)
}

// etagMatches reports whether an If-None-Match header lists etag (weak or strong).
func etagMatches(ifNoneMatch, etag string) bool {
	for candidate := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == etag || candidate == "*" {
			return true
		}
	}
	return false
}

type sessionResponse struct {
	AdminEnabled  bool    `json:"admin_enabled"`
	Authenticated bool    `json:"authenticated"`
	CSRFToken     *string `json:"csrf_token"`
}

func (s *Server) sessionStatus(c *echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	ctx := c.Request().Context()
	enabled, err := s.deps.Auth.Enabled(ctx)
	if err != nil {
		return err
	}
	resp := sessionResponse{AdminEnabled: enabled}
	if !enabled {
		return c.JSON(http.StatusOK, resp)
	}

	cookie, err := c.Cookie(sessionCookie)
	if err != nil {
		return c.JSON(http.StatusOK, resp)
	}
	// Only admin sessions: a viewer token in this cookie must not get a CSRF token.
	sess, err := s.deps.Auth.LookupAdmin(ctx, cookie.Value)
	if errors.Is(err, auth.ErrNoSession) {
		return c.JSON(http.StatusOK, resp)
	}
	if err != nil {
		return err
	}
	resp.Authenticated = true
	resp.CSRFToken = &sess.CSRFToken
	return c.JSON(http.StatusOK, resp)
}

type loginRequest struct {
	Password string `json:"password"`
}

func (s *Server) login(c *echo.Context) error {
	if !s.sameOriginRequest(c) {
		return echo.NewHTTPError(http.StatusForbidden, "cross-origin request rejected")
	}
	if !s.limiter.Allow(c.RealIP()) {
		return echo.NewHTTPError(http.StatusTooManyRequests, "too many login attempts; try again in a minute")
	}

	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx := c.Request().Context()
	err := s.deps.Auth.CheckPassword(ctx, req.Password)
	switch {
	case errors.Is(err, auth.ErrAdminDisabled):
		return echo.NewHTTPError(http.StatusForbidden, "admin interface is disabled")
	case errors.Is(err, auth.ErrInvalidPassword):
		s.deps.Logger.Warn("failed admin login", "ip", c.RealIP())
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	case err != nil:
		return err
	}

	// Rotate: drop any previous session presented by this client.
	if old, err := c.Cookie(sessionCookie); err == nil {
		_ = s.deps.Auth.RevokeAdmin(ctx, old.Value)
	}

	id, _, err := s.deps.Auth.CreateSession(ctx)
	if err != nil {
		return err
	}
	s.setSessionCookie(c, id)
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) logout(c *echo.Context) error {
	if cookie, err := c.Cookie(sessionCookie); err == nil {
		if err := s.deps.Auth.RevokeAdmin(c.Request().Context(), cookie.Value); err != nil {
			return err
		}
	}
	s.clearSessionCookie(c)
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) adminTree(c *echo.Context) error {
	tree, err := s.deps.Menu.Tree(c.Request().Context(), currentDashboard(c).ID, false)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tree)
}

func (s *Server) validateOptions(c *echo.Context) menu.ValidateOptions {
	return menu.ValidateOptions{ForbiddenIframeOrigins: s.dashboardOrigins(c)}
}

func (s *Server) createItem(c *echo.Context) error {
	var in menu.Input
	if err := bindJSONBody(c, &in); err != nil {
		return err
	}
	item, err := s.deps.Menu.Create(c.Request().Context(), currentDashboard(c).ID, in, s.validateOptions(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, item)
}

func (s *Server) updateItem(c *echo.Context) error {
	id, err := pathInt64(c, "itemID")
	if err != nil {
		return err
	}
	var in menu.Input
	if err := bindJSONBody(c, &in); err != nil {
		return err
	}
	item, err := s.deps.Menu.Update(c.Request().Context(), currentDashboard(c).ID, id, in, s.validateOptions(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

func (s *Server) deleteItem(c *echo.Context) error {
	id, err := pathInt64(c, "itemID")
	if err != nil {
		return err
	}
	if err := s.deps.Menu.Delete(c.Request().Context(), currentDashboard(c).ID, id, true); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type reorderRequest struct {
	Moves []menu.Move `json:"moves"`
}

func (s *Server) reorderItems(c *echo.Context) error {
	var req reorderRequest
	if err := bindJSONBody(c, &req); err != nil {
		return err
	}
	if err := s.deps.Menu.Reorder(c.Request().Context(), currentDashboard(c).ID, req.Moves); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) exportItems(c *echo.Context) error {
	doc, err := s.deps.Menu.Export(c.Request().Context(), currentDashboard(c).ID)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename="dashboard-menu.json"`)
	return c.JSON(http.StatusOK, doc)
}

func (s *Server) importItems(c *echo.Context) error {
	var doc menu.Document
	if err := bindJSONBody(c, &doc); err != nil {
		return err
	}
	mode := menu.ImportMode(c.QueryParam("mode"))
	n, err := s.deps.Menu.Import(c.Request().Context(), currentDashboard(c).ID, doc, mode, s.validateOptions(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]int{"imported": n})
}

// bindJSONBody binds only the JSON body (no path/query params), so fields
// like "id" in a path cannot leak into the payload.
func bindJSONBody(c *echo.Context, dst any) error {
	if !strings.HasPrefix(c.Request().Header.Get(echo.HeaderContentType), echo.MIMEApplicationJSON) {
		return echo.NewHTTPError(http.StatusUnsupportedMediaType, "expected application/json")
	}
	if err := echo.BindBody(c, dst); err != nil {
		if code := echo.StatusCode(err); code == http.StatusRequestEntityTooLarge {
			return err
		}
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	return nil
}

// pathInt64 parses a positive numeric path parameter.
func pathInt64(c *echo.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	return id, nil
}
