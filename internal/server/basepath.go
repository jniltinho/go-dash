package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jniltinho/go-dash/internal/config"
)

// CheckBasePath rejects a base path whose first segment is a file in the
// root of the frontend build: a stripped request for that file could not be
// told apart from a request under the prefix.
func CheckBasePath(assets fs.FS, basePath string) error {
	first, _, _ := strings.Cut(strings.Trim(config.NormalizeBasePath(basePath), "/"), "/")
	if first == "" {
		return nil
	}
	if _, err := fs.Stat(assets, first); err == nil {
		return fmt.Errorf("base_path %q must not start with %q: the frontend build has a file with that name", basePath, first)
	}
	return nil
}

// stripBasePath lets the application work behind a proxy that keeps the
// prefix (/panel/api/menu) as well as one that strips it (/api/menu): the
// prefix is removed on a segment boundary only, and only when the escaped
// form of the path carries it too. "/panel" is redirected to "/panel/".
func (s *Server) stripBasePath(next echo.HandlerFunc) echo.HandlerFunc {
	trimmed := strings.TrimSuffix(s.base, "/")
	return func(c *echo.Context) error {
		u := c.Request().URL
		if u.Path == trimmed {
			target := s.base
			if u.RawQuery != "" {
				target += "?" + u.RawQuery
			}
			return c.Redirect(http.StatusMovedPermanently, target)
		}
		if !strings.HasPrefix(u.Path, s.base) {
			return next(c)
		}
		if u.RawPath != "" {
			if !strings.HasPrefix(u.RawPath, s.base) {
				// The prefix is encoded differently: not ours, leave it (404).
				return next(c)
			}
			u.RawPath = u.RawPath[len(trimmed):]
		}
		u.Path = u.Path[len(trimmed):]
		return next(c)
	}
}

// joinBase prefixes a root-relative path ("/dash/x") with the base path.
func joinBase(base, p string) string {
	return base + strings.TrimPrefix(p, "/")
}
