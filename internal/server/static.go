package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/jniltinho/go-dash/internal/menu"
)

const missingBuildPage = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Go Dash</title></head>
<body style="background:#000;color:#e5e7eb;font-family:sans-serif;padding:2rem">
<h1>Frontend not built</h1><p>Run <code>make frontend</code> and build the binary again.</p></body></html>`

type cachedFile struct {
	content []byte
	etag    string
	modTime time.Time
}

// staticHandler serves the SPA build with ETags, long-lived caching for
// hashed assets and an index.html fallback for client-side routes.
type staticHandler struct {
	fsys fs.FS
	// cache is disabled in dev mode so rebuilt files are picked up.
	cache bool
	mu    sync.RWMutex
	files map[string]*cachedFile
	// base is the base path: prefixed onto redirects and injected into
	// index.html as <base href>, so the relative asset URLs of the build
	// resolve under it from any route.
	base string
}

func newStaticHandler(fsys fs.FS, dev bool, base string) *staticHandler {
	return &staticHandler{fsys: fsys, cache: !dev, files: map[string]*cachedFile{}, base: base}
}

func (h *staticHandler) hasIndex() bool {
	_, err := fs.Stat(h.fsys, "index.html")
	return err == nil
}

func (h *staticHandler) serve(c *echo.Context) error {
	if p := c.Request().URL.Path; p == "/dash" || strings.HasPrefix(p, "/dash/") {
		return h.serveDashboard(c, p)
	}

	reqPath := strings.TrimPrefix(path.Clean("/"+c.Request().URL.Path), "/")
	if reqPath == "" {
		reqPath = "index.html"
	}

	file, err := h.load(reqPath)
	switch {
	case err == nil:
		return h.write(c, reqPath, file)
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}

	if !isSPARoute(reqPath) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	return h.serveIndex(c)
}

// serveDashboard canonicalizes /dash/<slug> URLs: /dash, /dash/ and the
// "default" sentinel go to "/", a trailing slash or upper case is redirected
// to the lower-case form, and only one segment with valid slug syntax gets
// the SPA. Whether the slug exists is decided by the SPA through the API.
func (h *staticHandler) serveDashboard(c *echo.Context, p string) error {
	rest := strings.TrimPrefix(strings.TrimPrefix(p, "/dash"), "/")
	segment := strings.TrimSuffix(rest, "/")
	if segment == "" {
		return c.Redirect(http.StatusFound, h.base)
	}
	if strings.Contains(segment, "/") {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	slug := strings.ToLower(segment)
	if slug == menu.DefaultSlug {
		return c.Redirect(http.StatusFound, h.base)
	}
	if !menu.ValidSlugSyntax(slug) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if slug != rest {
		target := joinBase(h.base, "/dash/"+slug)
		if q := c.Request().URL.RawQuery; q != "" {
			target += "?" + q
		}
		return c.Redirect(http.StatusMovedPermanently, target)
	}
	return h.serveIndex(c)
}

func (h *staticHandler) serveIndex(c *echo.Context) error {
	index, err := h.load("index.html")
	if errors.Is(err, fs.ErrNotExist) {
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.HTML(http.StatusOK, missingBuildPage)
	}
	if err != nil {
		return err
	}
	return h.write(c, "index.html", index)
}

// isSPARoute reports whether a missing path belongs to a client-side route.
func isSPARoute(p string) bool {
	return p == "index.html" || p == "admin" || strings.HasPrefix(p, "admin/")
}

func (h *staticHandler) load(name string) (*cachedFile, error) {
	if h.cache {
		h.mu.RLock()
		f, ok := h.files[name]
		h.mu.RUnlock()
		if ok {
			return f, nil
		}
	}

	info, err := fs.Stat(h.fsys, name)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fs.ErrNotExist
	}

	content, err := fs.ReadFile(h.fsys, name)
	if err != nil {
		return nil, err
	}
	modTime := info.ModTime()
	if name == "index.html" {
		content = injectBase(content, h.base)
		// The served HTML depends on the base path, not only on the file:
		// revalidate by ETag only (no Last-Modified).
		modTime = time.Time{}
	}
	sum := sha256.Sum256(content)
	f := &cachedFile{
		content: content,
		etag:    `"` + hex.EncodeToString(sum[:16]) + `"`,
		modTime: modTime,
	}

	if h.cache {
		h.mu.Lock()
		h.files[name] = f
		h.mu.Unlock()
	}
	return f, nil
}

func (h *staticHandler) write(c *echo.Context, name string, f *cachedFile) error {
	header := c.Response().Header()
	header.Set("ETag", f.etag)
	switch {
	case name == "index.html":
		header.Set("Cache-Control", "no-cache")
	case strings.HasPrefix(name, "assets/"):
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		header.Set("Cache-Control", "public, max-age=3600")
	}

	var reader io.ReadSeeker = bytes.NewReader(f.content)
	http.ServeContent(c.Response(), c.Request(), name, f.modTime, reader)
	return nil
}

// injectBase adds <base href> right after <head>. It is always added, also
// for "/": the build uses relative asset URLs, which would otherwise resolve
// against deep routes such as /dash/<slug>.
func injectBase(content []byte, base string) []byte {
	tag := []byte(`<base href="` + html.EscapeString(base) + `" />`)
	i := bytes.Index(bytes.ToLower(content), []byte("<head>"))
	if i < 0 {
		return append(tag, content...)
	}
	i += len("<head>")
	out := make([]byte, 0, len(content)+len(tag))
	out = append(out, content[:i]...)
	out = append(out, tag...)
	return append(out, content[i:]...)
}
