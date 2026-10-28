package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/jniltinho/go-dash/internal/auth"
	"github.com/jniltinho/go-dash/internal/menu"
)

const (
	dashboardContextKey = "dashboard"
	// viewerCookiePrefix + dashboard id names the viewer session cookie. The
	// id (not the slug) keeps the cookie valid across slug renames.
	viewerCookiePrefix = "dashboard_view_"
	maxUnlockBody      = 4 << 10 // 4 KiB
)

func viewerCookieName(dashboardID int64) string {
	return viewerCookiePrefix + strconv.FormatInt(dashboardID, 10)
}

func currentDashboard(c *echo.Context) menu.Dashboard {
	d, _ := c.Get(dashboardContextKey).(menu.Dashboard)
	return d
}

// noStore marks responses that depend on cookies, including their errors.
func noStore(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		return next(c)
	}
}

// resolvePublicDashboard loads the dashboard named by :slug ("default" for
// the default one). The record is read on every request, so password changes
// made by the CLI apply to a running server.
func (s *Server) resolvePublicDashboard(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		d, err := s.deps.Menu.DashboardBySlug(c.Request().Context(), c.Param("slug"))
		if err != nil {
			return err
		}
		c.Set(dashboardContextKey, d)
		return next(c)
	}
}

// useDefaultDashboard backs the legacy routes (/api/menu, /api/admin/items…).
func (s *Server) useDefaultDashboard(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		d, err := s.deps.Menu.GetDashboard(c.Request().Context(), menu.DefaultDashboardID)
		if err != nil {
			return err
		}
		c.Set(dashboardContextKey, d)
		return next(c)
	}
}

// resolveAdminDashboard loads :dashboardID, a numeric id or "default".
func (s *Server) resolveAdminDashboard(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ref := c.Param("dashboardID")
		if _, err := strconv.ParseInt(ref, 10, 64); err != nil && ref != menu.DefaultSlug {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid dashboard id")
		}
		d, err := s.deps.Menu.ResolveDashboard(c.Request().Context(), ref)
		if err != nil {
			return err
		}
		c.Set(dashboardContextKey, d)
		return next(c)
	}
}

// hasAdminSession reports whether the request carries a valid admin session.
func (s *Server) hasAdminSession(c *echo.Context) (bool, error) {
	cookie, err := c.Cookie(sessionCookie)
	if err != nil {
		return false, nil
	}
	_, err = s.deps.Auth.LookupAdmin(c.Request().Context(), cookie.Value)
	if errors.Is(err, auth.ErrNoSession) {
		return false, nil
	}
	return err == nil, err
}

// canRead reports whether the request may read the current dashboard: it is
// public, the client is an admin, or it holds a viewer session of exactly
// this dashboard.
func (s *Server) canRead(c *echo.Context, d menu.Dashboard) (bool, error) {
	if !d.Protected {
		return true, nil
	}
	admin, err := s.hasAdminSession(c)
	if err != nil || admin {
		return admin, err
	}
	cookie, err := c.Cookie(viewerCookieName(d.ID))
	if err != nil {
		return false, nil
	}
	_, err = s.deps.Auth.LookupViewer(c.Request().Context(), cookie.Value, d.ID)
	if errors.Is(err, auth.ErrNoSession) {
		return false, nil
	}
	return err == nil, err
}

func (s *Server) requireDashboardAccess(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ok, err := s.canRead(c, currentDashboard(c))
		if err != nil {
			return err
		}
		if !ok {
			c.Response().Header().Set("Cache-Control", "no-store")
			return echo.NewHTTPError(http.StatusUnauthorized, "dashboard locked")
		}
		return next(c)
	}
}

// displayTitle falls back to the configured title on the default dashboard.
func (s *Server) displayTitle(d menu.Dashboard) string {
	if d.Title == "" {
		return s.deps.Config.Title
	}
	return d.Title
}

type dashboardStatusResponse struct {
	Slug      *string `json:"slug"`
	IsDefault bool    `json:"is_default"`
	Title     string  `json:"title"`
	Protected bool    `json:"protected"`
	Unlocked  bool    `json:"unlocked"`
}

func (s *Server) dashboardStatus(c *echo.Context) error {
	d := currentDashboard(c)
	unlocked, err := s.canRead(c, d)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, dashboardStatusResponse{
		Slug:      d.Slug,
		IsDefault: d.IsDefault,
		Title:     s.displayTitle(d),
		Protected: d.Protected,
		Unlocked:  unlocked,
	})
}

func (s *Server) unlockDashboard(c *echo.Context) error {
	if !s.sameOriginRequest(c) {
		return echo.NewHTTPError(http.StatusForbidden, "cross-origin request rejected")
	}
	// Keyed by IP only, so cycling dashboards does not multiply attempts.
	if !s.unlockLimiter.Allow(c.RealIP()) {
		return echo.NewHTTPError(http.StatusTooManyRequests, "too many attempts; try again in a minute")
	}

	var req loginRequest
	if err := bindJSONBody(c, &req); err != nil {
		return err
	}

	d := currentDashboard(c)
	ctx := c.Request().Context()
	id, _, err := s.deps.Auth.Unlock(ctx, d.ID, req.Password)
	switch {
	case errors.Is(err, auth.ErrNotProtected):
		return echo.NewHTTPError(http.StatusBadRequest, "dashboard is not protected")
	case errors.Is(err, auth.ErrInvalidPassword):
		s.deps.Logger.Warn("failed dashboard unlock", "ip", c.RealIP(), "dashboard", d.ID)
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid password")
	case err != nil:
		return err
	}

	name := viewerCookieName(d.ID)
	if old, err := c.Cookie(name); err == nil {
		_ = s.deps.Auth.RevokeViewer(ctx, old.Value, d.ID)
	}
	s.setCookie(c, name, id, int(auth.DefaultAbsoluteTimeout.Seconds()))
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) lockDashboard(c *echo.Context) error {
	if !s.sameOriginRequest(c) {
		return echo.NewHTTPError(http.StatusForbidden, "cross-origin request rejected")
	}
	d := currentDashboard(c)
	name := viewerCookieName(d.ID)
	if cookie, err := c.Cookie(name); err == nil {
		if err := s.deps.Auth.RevokeViewer(c.Request().Context(), cookie.Value, d.ID); err != nil {
			return err
		}
	}
	s.setCookie(c, name, "", -1)
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) listDashboards(c *echo.Context) error {
	list, err := s.deps.Menu.ListDashboards(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, list)
}

func (s *Server) createDashboard(c *echo.Context) error {
	var in menu.DashboardInput
	if err := bindJSONBody(c, &in); err != nil {
		return err
	}
	d, err := s.deps.Menu.CreateDashboard(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, d)
}

func (s *Server) updateDashboard(c *echo.Context) error {
	var in menu.DashboardInput
	if err := bindJSONBody(c, &in); err != nil {
		return err
	}
	d, err := s.deps.Menu.UpdateDashboard(c.Request().Context(), currentDashboard(c).ID, in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, d)
}

func (s *Server) deleteDashboard(c *echo.Context) error {
	force := c.QueryParam("force") == "true"
	if err := s.deps.Menu.DeleteDashboard(c.Request().Context(), currentDashboard(c).ID, force); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) setDashboardPassword(c *echo.Context) error {
	var req loginRequest
	if err := bindJSONBody(c, &req); err != nil {
		return err
	}
	if err := s.deps.Auth.SetDashboardPassword(c.Request().Context(), currentDashboard(c).ID, req.Password); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Server) clearDashboardPassword(c *echo.Context) error {
	if err := s.deps.Auth.ClearDashboardPassword(c.Request().Context(), currentDashboard(c).ID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
