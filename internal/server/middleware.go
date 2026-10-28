package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jniltinho/go-dash/internal/auth"
	"github.com/jniltinho/go-dash/internal/menu"
)

const sessionContextKey = "session"

// securityHeaders sets CSP and hardening headers on every response.
func (s *Server) securityHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data:",
		"font-src 'self' data:",
		"connect-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'self'",
		"frame-src " + s.deps.Config.FrameSrc,
	}, "; ")

	return func(c *echo.Context) error {
		h := c.Response().Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if s.isHTTPS(c) {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		return next(c)
	}
}

// isHTTPS reports whether the client connection is HTTPS, trusting
// X-Forwarded-Proto only when trust_proxy is enabled.
func (s *Server) isHTTPS(c *echo.Context) bool {
	if c.Request().TLS != nil {
		return true
	}
	return s.deps.Config.TrustProxy && strings.EqualFold(c.Request().Header.Get("X-Forwarded-Proto"), "https")
}

// requestOrigin returns scheme://host as seen by the client.
func (s *Server) requestOrigin(c *echo.Context) string {
	scheme := "http"
	if s.isHTTPS(c) {
		scheme = "https"
	}
	return menu.NormalizeOrigin(scheme, c.Request().Host)
}

// dashboardOrigins lists every origin that identifies the dashboard itself.
func (s *Server) dashboardOrigins(c *echo.Context) []string {
	origins := []string{s.requestOrigin(c)}
	if o := menu.Origin(s.deps.Config.PublicURL); o != "" {
		origins = append(origins, o)
	}
	return origins
}

// sameOriginRequest rejects cross-site browser requests. Non-browser clients
// that send neither Origin nor Sec-Fetch-Site are allowed.
func (s *Server) sameOriginRequest(c *echo.Context) bool {
	origin := c.Request().Header.Get("Origin")
	if origin != "" {
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		normalized := menu.NormalizeOrigin(u.Scheme, u.Host)
		for _, o := range s.dashboardOrigins(c) {
			if normalized == o {
				return true
			}
		}
		return false
	}
	site := c.Request().Header.Get("Sec-Fetch-Site")
	return site == "" || site == "same-origin" || site == "none"
}

func (s *Server) requireSession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		cookie, err := c.Cookie(sessionCookie)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
		}
		sess, err := s.deps.Auth.LookupAdmin(c.Request().Context(), cookie.Value)
		if errors.Is(err, auth.ErrNoSession) {
			s.clearSessionCookie(c)
			return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
		}
		if err != nil {
			return err
		}
		c.Set(sessionContextKey, sess)
		return next(c)
	}
}

func (s *Server) requireCSRF(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		switch c.Request().Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return next(c)
		}
		sess, _ := c.Get(sessionContextKey).(auth.Session)
		if !auth.ValidCSRF(sess.CSRFToken, c.Request().Header.Get(csrfHeader)) {
			return echo.NewHTTPError(http.StatusForbidden, "invalid CSRF token")
		}
		return next(c)
	}
}

func (s *Server) setSessionCookie(c *echo.Context, id string) {
	s.setCookie(c, sessionCookie, id, int(auth.DefaultAbsoluteTimeout.Seconds()))
}

func (s *Server) clearSessionCookie(c *echo.Context) {
	s.setCookie(c, sessionCookie, "", -1)
}

// setCookie writes an HttpOnly, SameSite=Strict cookie scoped to the base
// path; maxAge < 0 deletes it.
func (s *Server) setCookie(c *echo.Context, name, value string, maxAge int) {
	c.SetCookie(&http.Cookie{
		Name:     name,
		Value:    value,
		Path:     s.base,
		HttpOnly: true,
		Secure:   s.deps.Config.CookieSecure || s.isHTTPS(c),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	})
}
