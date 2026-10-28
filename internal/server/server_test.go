package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jniltinho/go-dash/internal/auth"
	"github.com/jniltinho/go-dash/internal/config"
	"github.com/jniltinho/go-dash/internal/database"
	"github.com/jniltinho/go-dash/internal/menu"
)

const testPassword = "correct-horse"

type testEnv struct {
	t       *testing.T
	handler http.Handler
	cookie  *http.Cookie
	csrf    string
}

func newTestEnv(t *testing.T, withPassword bool) *testEnv {
	t.Helper()
	return newTestEnvWith(t, withPassword, func(*config.Config) {})
}

// newTestEnvWith lets a test adjust the configuration before the server is built.
func newTestEnvWith(t *testing.T, withPassword bool, adjust func(*config.Config)) *testEnv {
	t.Helper()
	db, err := database.OpenAndMigrate(context.Background(), filepath.Join(t.TempDir(), "srv.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	opts := auth.Options{}
	if withPassword {
		opts.EnvPassword = testPassword
	}
	mgr, err := auth.NewManager(db, opts)
	if err != nil {
		t.Fatal(err)
	}

	assets := fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html><div id=app></div>")},
		"assets/app-1.js": {Data: []byte("console.log(1)")},
		"favicon.svg":     {Data: []byte("<svg/>")},
	}
	cfg := config.Config{
		Addr:              ":0",
		DB:                "x",
		Title:             "Panel",
		FrameSrc:          "http: https:",
		IframeSandbox:     config.DefaultSandbox,
		MaxTabs:           10,
		LoginRate:         5,
		DefaultTheme:      "ocean-light",
		DefaultThemeStyle: "flat",
	}
	adjust(&cfg)
	srv := New(Deps{
		Config: cfg,
		DB:     db,
		Menu:   menu.NewService(db),
		Auth:   mgr,
		Assets: assets,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return &testEnv{t: t, handler: srv.Handler()}
}

func (e *testEnv) do(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if e.cookie != nil {
		req.AddCookie(e.cookie)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *testEnv) login() {
	e.t.Helper()
	rec := e.do(http.MethodPost, "/api/admin/login", `{"password":"`+testPassword+`"}`, nil)
	if rec.Code != http.StatusNoContent {
		e.t.Fatalf("login status = %d body=%s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			e.cookie = c
		}
	}
	if e.cookie == nil || !e.cookie.HttpOnly || e.cookie.SameSite != http.SameSiteStrictMode {
		e.t.Fatalf("bad session cookie: %+v", e.cookie)
	}

	var sess sessionResponse
	rec = e.do(http.MethodGet, "/api/admin/session", "", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)
	if !sess.Authenticated || sess.CSRFToken == nil {
		e.t.Fatalf("session not authenticated: %s", rec.Body)
	}
	e.csrf = *sess.CSRFToken
}

func (e *testEnv) csrfHeaders() map[string]string {
	return map[string]string{csrfHeader: e.csrf}
}

func TestHealthAndHeaders(t *testing.T) {
	env := newTestEnv(t, false)
	rec := env.do(http.MethodGet, "/healthz", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("healthz = %d %s", rec.Code, rec.Body)
	}

	rec = env.do(http.MethodGet, "/", "", nil)
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"script-src 'self'", "object-src 'none'", "frame-src http: https:", "frame-ancestors 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q: %s", want, csp)
		}
	}
	if rec.Header().Get("X-Frame-Options") != "SAMEORIGIN" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing hardening headers")
	}
}

func TestPublicConfigDefaultTheme(t *testing.T) {
	env := newTestEnv(t, false)
	rec := env.do(http.MethodGet, "/api/config", "", nil)
	body := rec.Body.String()
	if !strings.Contains(body, `"default_theme":"ocean-light"`) || !strings.Contains(body, `"default_theme_style":"flat"`) {
		t.Fatalf("config = %s", body)
	}
}

func TestSessionStatus(t *testing.T) {
	disabled := newTestEnv(t, false)
	rec := disabled.do(http.MethodGet, "/api/admin/session", "", nil)
	if strings.TrimSpace(rec.Body.String()) != `{"admin_enabled":false,"authenticated":false,"csrf_token":null}` {
		t.Fatalf("disabled session = %s", rec.Body)
	}

	enabled := newTestEnv(t, true)
	rec = enabled.do(http.MethodGet, "/api/admin/session", "", nil)
	if strings.TrimSpace(rec.Body.String()) != `{"admin_enabled":true,"authenticated":false,"csrf_token":null}` {
		t.Fatalf("anonymous session = %s", rec.Body)
	}
}

func TestLoginFailuresAndRateLimit(t *testing.T) {
	env := newTestEnv(t, true)

	rec := env.do(http.MethodPost, "/api/admin/login", `{"password":"x"}`, map[string]string{"Origin": "https://evil.example"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin login = %d", rec.Code)
	}

	for i := range 5 {
		rec = env.do(http.MethodPost, "/api/admin/login", `{"password":"wrong"}`, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d", i+1, rec.Code)
		}
	}
	rec = env.do(http.MethodPost, "/api/admin/login", `{"password":"`+testPassword+`"}`, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt = %d", rec.Code)
	}

	disabled := newTestEnv(t, false)
	rec = disabled.do(http.MethodPost, "/api/admin/login", `{"password":"x"}`, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("login with admin disabled = %d", rec.Code)
	}
}

func TestAdminAuthAndCSRF(t *testing.T) {
	env := newTestEnv(t, true)

	rec := env.do(http.MethodPost, "/api/admin/items", `{"title":"x"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create = %d", rec.Code)
	}

	env.login()
	rec = env.do(http.MethodPost, "/api/admin/items", `{"title":"x"}`, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create without csrf = %d", rec.Code)
	}

	other := newTestEnv(t, true)
	other.handler = env.handler
	other.login()
	rec = env.do(http.MethodPost, "/api/admin/items", `{"title":"x"}`, map[string]string{csrfHeader: other.csrf})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create with another session's csrf = %d", rec.Code)
	}

	rec = env.do(http.MethodPost, "/api/admin/logout", "", env.csrfHeaders())
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d", rec.Code)
	}
	rec = env.do(http.MethodGet, "/api/admin/items", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout = %d", rec.Code)
	}
}

func TestItemsCRUD(t *testing.T) {
	env := newTestEnv(t, true)
	env.login()
	h := env.csrfHeaders()

	rec := env.do(http.MethodPost, "/api/admin/items", `{"title":"Monitoring"}`, h)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	var group menu.Item
	_ = json.Unmarshal(rec.Body.Bytes(), &group)

	body := `{"title":"Grafana","url":"https://grafana.example.com","parent_id":` + itoa(group.ID) + `}`
	rec = env.do(http.MethodPost, "/api/admin/items", body, h)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create child = %d %s", rec.Code, rec.Body)
	}

	rec = env.do(http.MethodPost, "/api/admin/items", `{"title":"x","url":"javascript:alert(1)"}`, h)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"field":"url"`) {
		t.Fatalf("invalid url = %d %s", rec.Code, rec.Body)
	}

	rec = env.do(http.MethodPost, "/api/admin/items", `{"title":"self","url":"http://example.com/admin"}`, h)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("same-origin iframe (Host example.com) = %d %s", rec.Code, rec.Body)
	}

	rec = env.do(http.MethodPut, "/api/admin/items/999", `{"title":"x"}`, h)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update missing = %d", rec.Code)
	}

	big := `{"title":"` + strings.Repeat("a", maxAdminBody) + `"}`
	rec = env.do(http.MethodPost, "/api/admin/items", big, h)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body = %d", rec.Code)
	}

	rec = env.do(http.MethodGet, "/api/menu", "", nil)
	var tree []menu.Node
	_ = json.Unmarshal(rec.Body.Bytes(), &tree)
	if len(tree) != 1 || len(tree[0].Items) != 1 || tree[0].Items[0].Title != "Grafana" {
		t.Fatalf("public tree = %s", rec.Body)
	}

	// Polling an unchanged menu costs a 304; any change gives a new ETag.
	etag := rec.Header().Get("ETag")
	if etag == "" || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("menu caching headers = %v", rec.Header())
	}
	rec = env.do(http.MethodGet, "/api/menu", "", map[string]string{"If-None-Match": "W/" + etag})
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Fatalf("unchanged menu = %d %q", rec.Code, rec.Body)
	}
	rec = env.do(http.MethodPut, "/api/admin/items/"+itoa(group.ID),
		`{"title":"Monitoria","placement":"both","target":"iframe"}`, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename = %d %s", rec.Code, rec.Body)
	}
	rec = env.do(http.MethodGet, "/api/menu", "", map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusOK || rec.Header().Get("ETag") == etag {
		t.Fatalf("changed menu = %d etag %q", rec.Code, rec.Header().Get("ETag"))
	}

	rec = env.do(http.MethodDelete, "/api/admin/items/"+itoa(group.ID), "", h)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rec.Code)
	}
	rec = env.do(http.MethodGet, "/api/menu", "", nil)
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty menu = %s", rec.Body)
	}
}

func TestImportExport(t *testing.T) {
	env := newTestEnv(t, true)
	env.login()
	h := env.csrfHeaders()

	doc := `{"format":"dashboard-menu","version":1,"items":[{"title":"Infra","items":[{"title":"Wiki","url":"https://wiki.example.com","target":"blank"}]}]}`
	rec := env.do(http.MethodPost, "/api/admin/import?mode=replace", doc, h)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"imported":2`) {
		t.Fatalf("import = %d %s", rec.Code, rec.Body)
	}

	rec = env.do(http.MethodGet, "/api/admin/export", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"format":"dashboard-menu"`) {
		t.Fatalf("export = %d %s", rec.Code, rec.Body)
	}
}

func TestSPAAndStatic(t *testing.T) {
	env := newTestEnv(t, false)

	for _, p := range []string{"/", "/admin", "/admin/login"} {
		rec := env.do(http.MethodGet, p, "", nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "id=app") {
			t.Fatalf("%s = %d", p, rec.Code)
		}
		if rec.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s cache = %q", p, rec.Header().Get("Cache-Control"))
		}
	}

	rec := env.do(http.MethodGet, "/assets/app-1.js", "", nil)
	etag := rec.Header().Get("ETag")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") || etag == "" {
		t.Fatalf("asset = %d %v", rec.Code, rec.Header())
	}
	rec = env.do(http.MethodGet, "/assets/app-1.js", "", map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusNotModified {
		t.Fatalf("conditional = %d", rec.Code)
	}

	rec = env.do(http.MethodGet, "/api/missing", "", nil)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("unknown api = %d %s", rec.Code, rec.Body)
	}

	rec = env.do(http.MethodGet, "/nope.txt", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown file = %d", rec.Code)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
