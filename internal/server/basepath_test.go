package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jniltinho/go-dash/internal/config"
)

func newBaseEnv(t *testing.T, base string) *testEnv {
	t.Helper()
	return newTestEnvWith(t, true, func(c *config.Config) { c.BasePath = base })
}

func TestBasePathBothProxyStyles(t *testing.T) {
	env := newBaseEnv(t, "/panel/")

	// The same requests work with the prefix kept or stripped by the proxy.
	for _, prefix := range []string{"/panel", ""} {
		for _, p := range []string{"/", "/admin", "/dash/acme", "/api/menu", "/api/dash/default/status", "/healthz"} {
			rec := env.do(http.MethodGet, prefix+p, "", nil)
			if rec.Code != http.StatusOK {
				t.Errorf("GET %s%s = %d %s", prefix, p, rec.Code, rec.Body)
			}
		}
	}

	rec := env.do(http.MethodGet, "/panel?x=1", "", nil)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/panel/?x=1" {
		t.Fatalf("missing slash = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestBasePathBoundariesAndEncoding(t *testing.T) {
	env := newBaseEnv(t, "/panel/")
	cases := []struct {
		path string
		code int
	}{
		{"/panel-x/api/menu", http.StatusNotFound}, // sibling prefix is not stripped
		{"/panelapi/menu", http.StatusNotFound},
		{"/pai%6Eel/api/menu", http.StatusNotFound}, // prefix encoded differently is not ours
		{"/panel/api/dash/a%2Fb/status", http.StatusNotFound},
		{"/panel/panel/api/menu", http.StatusNotFound}, // stripped once only
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		env.handler.ServeHTTP(rec, req)
		if rec.Code != tc.code {
			t.Errorf("GET %s = %d, want %d", tc.path, rec.Code, tc.code)
		}
	}

	nested := newBaseEnv(t, "/ops/dash/")
	if rec := nested.do(http.MethodGet, "/ops/dash/dash/acme", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("nested base = %d", rec.Code)
	}
	if rec := nested.do(http.MethodGet, "/ops/dash", "", nil); rec.Header().Get("Location") != "/ops/dash/" {
		t.Fatalf("nested missing slash = %q", rec.Header().Get("Location"))
	}
}

func TestBasePathEmittedURLs(t *testing.T) {
	env := newBaseEnv(t, "/panel/")

	rec := env.do(http.MethodGet, "/panel/dash/Acme/", "", nil)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/panel/dash/acme" {
		t.Fatalf("canonical redirect = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, p := range []string{"/panel/dash/default", "/panel/dash", "/dash/default"} {
		if rec := env.do(http.MethodGet, p, "", nil); rec.Header().Get("Location") != "/panel/" {
			t.Fatalf("%s redirect = %q", p, rec.Header().Get("Location"))
		}
	}

	rec = env.do(http.MethodGet, "/panel/admin", "", nil)
	if !strings.Contains(rec.Body.String(), `<head><base href="/panel/" />`) &&
		!strings.Contains(rec.Body.String(), `<base href="/panel/" />`) {
		t.Fatalf("index without base: %s", rec.Body)
	}
	etag := rec.Header().Get("ETag")
	req := httptest.NewRequest(http.MethodGet, "/panel/", nil)
	req.Header.Set("If-None-Match", etag)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotModified {
		t.Fatalf("conditional index = %d", rr.Code)
	}
	if root := newTestEnv(t, false).do(http.MethodGet, "/", "", nil); root.Header().Get("ETag") == etag {
		t.Fatal("index ETag must depend on the injected base")
	}
	// Changing the base path changes the HTML: no Last-Modified revalidation.
	if rec.Header().Get("Last-Modified") != "" {
		t.Fatalf("index must not send Last-Modified: %q", rec.Header().Get("Last-Modified"))
	}
	req = httptest.NewRequest(http.MethodGet, "/panel/", nil)
	req.Header.Set("If-Modified-Since", "Mon, 01 Jan 2200 00:00:00 GMT")
	rr = httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("If-Modified-Since on index = %d", rr.Code)
	}

	env.login()
	if env.cookie.Path != "/panel/" {
		t.Fatalf("admin cookie path = %q", env.cookie.Path)
	}
	env.setDashPassword("default", "default-secret")
	if viewer := env.unlock("default", "default-secret"); viewer.Path != "/panel/" {
		t.Fatalf("viewer cookie path = %q", viewer.Path)
	}
}

func TestRootBasePathUnchanged(t *testing.T) {
	env := newTestEnv(t, true)
	rec := env.do(http.MethodGet, "/dash/default", "", nil)
	if rec.Header().Get("Location") != "/" {
		t.Fatalf("root redirect = %q", rec.Header().Get("Location"))
	}
	if !strings.Contains(env.do(http.MethodGet, "/", "", nil).Body.String(), `<base href="/" />`) {
		t.Fatal("index must carry <base href=\"/\"> for relative assets")
	}
	env.login()
	if env.cookie.Path != "/" {
		t.Fatalf("cookie path = %q", env.cookie.Path)
	}
}

func TestCheckBasePath(t *testing.T) {
	assets := fstest.MapFS{"index.html": {}, "robots.txt": {}}
	if err := CheckBasePath(assets, "/robots.txt/"); err == nil {
		t.Fatal("a base path named like a build file must be rejected")
	}
	if err := CheckBasePath(assets, "/panel/"); err != nil {
		t.Fatal(err)
	}
	if err := CheckBasePath(assets, "/"); err != nil {
		t.Fatal(err)
	}
}
