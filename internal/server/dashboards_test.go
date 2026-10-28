package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jniltinho/go-dash/internal/menu"
)

// doCookies sends a request with exactly the given cookies.
func (e *testEnv) doCookies(method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *testEnv) createDashboard(slug, title string) menu.Dashboard {
	e.t.Helper()
	rec := e.do(http.MethodPost, "/api/admin/dashboards", `{"slug":"`+slug+`","title":"`+title+`"}`, e.csrfHeaders())
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("create dashboard %s = %d %s", slug, rec.Code, rec.Body)
	}
	var d menu.Dashboard
	_ = json.Unmarshal(rec.Body.Bytes(), &d)
	return d
}

func (e *testEnv) setDashPassword(id, password string) {
	e.t.Helper()
	rec := e.do(http.MethodPut, "/api/admin/dashboards/"+id+"/password", `{"password":"`+password+`"}`, e.csrfHeaders())
	if rec.Code != http.StatusNoContent {
		e.t.Fatalf("set password %s = %d %s", id, rec.Code, rec.Body)
	}
}

// unlock returns the viewer cookie of a successful unlock.
func (e *testEnv) unlock(slug, password string) *http.Cookie {
	e.t.Helper()
	rec := e.doCookies(http.MethodPost, "/api/dash/"+slug+"/unlock", `{"password":"`+password+`"}`)
	if rec.Code != http.StatusNoContent {
		e.t.Fatalf("unlock %s = %d %s", slug, rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if strings.HasPrefix(c.Name, viewerCookiePrefix) {
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
				e.t.Fatalf("weak viewer cookie: %+v", c)
			}
			return c
		}
	}
	e.t.Fatal("unlock did not set a viewer cookie")
	return nil
}

func TestNamedDashboardsAreIsolated(t *testing.T) {
	env := newTestEnv(t, true)
	env.login()
	h := env.csrfHeaders()
	bio := env.createDashboard("acme", "Acme")
	bras := env.createDashboard("globex", "Globex")

	rec := env.do(http.MethodPost, "/api/admin/dashboards/"+itoa(bio.ID)+"/items", `{"title":"A"}`, h)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create A = %d %s", rec.Code, rec.Body)
	}
	var a menu.Item
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	if rec := env.do(http.MethodPost, "/api/admin/dashboards/"+itoa(bras.ID)+"/items", `{"title":"B"}`, h); rec.Code != http.StatusCreated {
		t.Fatalf("create B = %d", rec.Code)
	}

	rec = env.doCookies(http.MethodGet, "/api/dash/acme/menu", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"A"`) || strings.Contains(rec.Body.String(), `"B"`) {
		t.Fatalf("acme menu = %d %s", rec.Code, rec.Body)
	}
	if rec := env.doCookies(http.MethodGet, "/api/menu", ""); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("default menu = %s", rec.Body)
	}

	// Item of another dashboard: 404 and unchanged.
	rec = env.do(http.MethodPut, "/api/admin/dashboards/"+itoa(bras.ID)+"/items/"+itoa(a.ID), `{"title":"hijack"}`, h)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-dashboard update = %d", rec.Code)
	}
	if rec := env.do(http.MethodDelete, "/api/admin/items/"+itoa(a.ID), "", h); rec.Code != http.StatusNotFound {
		t.Fatalf("legacy delete of a named dashboard item = %d", rec.Code)
	}

	// The "default" sentinel and the legacy routes address the default dashboard.
	if rec := env.do(http.MethodPost, "/api/admin/dashboards/default/items", `{"title":"D"}`, h); rec.Code != http.StatusCreated {
		t.Fatalf("create in default = %d", rec.Code)
	}
	if rec := env.do(http.MethodGet, "/api/admin/items", "", nil); !strings.Contains(rec.Body.String(), `"D"`) {
		t.Fatalf("legacy tree = %s", rec.Body)
	}
	if rec := env.do(http.MethodGet, "/api/admin/dashboards/"+itoa(bio.ID)+"/export", "", nil); strings.Contains(rec.Body.String(), `"D"`) {
		t.Fatalf("export leaked items: %s", rec.Body)
	}

	rec = env.doCookies(http.MethodGet, "/api/dash/missing/status", "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "dashboard not found") {
		t.Fatalf("unknown status = %d %s", rec.Code, rec.Body)
	}
	rec = env.doCookies(http.MethodGet, "/api/dash/acme/status", "")
	if strings.TrimSpace(rec.Body.String()) != `{"slug":"acme","is_default":false,"title":"Acme","protected":false,"unlocked":true}` {
		t.Fatalf("status = %s", rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status cache = %q", rec.Header().Get("Cache-Control"))
	}
	for _, p := range []string{"/api/admin/login", "/api/admin/logout"} {
		if rec := env.doCookies(http.MethodPost, p, `{"password":"x"}`); rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s cache = %q", p, rec.Header().Get("Cache-Control"))
		}
	}
	rec = env.doCookies(http.MethodGet, "/api/dash/default/status", "")
	if !strings.Contains(rec.Body.String(), `"title":"Panel"`) {
		t.Fatalf("default title falls back to config: %s", rec.Body)
	}
}

func TestDashboardAdministration(t *testing.T) {
	env := newTestEnv(t, true)
	env.login()
	h := env.csrfHeaders()
	over := env.createDashboard("initech", "Initech")
	id := itoa(over.ID)

	for _, body := range []string{`{"slug":"admin","title":"X"}`, `{"slug":"Over Mind","title":"X"}`, `{"slug":"Initech","title":"X"}`} {
		rec := env.do(http.MethodPost, "/api/admin/dashboards", body, h)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"field":"slug"`) {
			t.Fatalf("%s = %d %s", body, rec.Code, rec.Body)
		}
	}
	if rec := env.do(http.MethodDelete, "/api/admin/dashboards/default", "", h); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete default = %d", rec.Code)
	}
	if rec := env.do(http.MethodPut, "/api/admin/dashboards/abc", `{"title":"x"}`, h); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid dashboard id = %d", rec.Code)
	}

	env.do(http.MethodPost, "/api/admin/dashboards/"+id+"/items", `{"title":"X"}`, h)
	if rec := env.do(http.MethodDelete, "/api/admin/dashboards/"+id, "", h); rec.Code != http.StatusConflict {
		t.Fatalf("delete non-empty = %d %s", rec.Code, rec.Body)
	}

	rec := env.do(http.MethodPut, "/api/admin/dashboards/"+id+"/password", `{"password":"short"}`, h)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"field":"password"`) {
		t.Fatalf("short password = %d %s", rec.Code, rec.Body)
	}
	env.setDashPassword(id, "initech-secret")
	if rec := env.do(http.MethodDelete, "/api/admin/dashboards/"+id+"/password", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("clear without csrf = %d", rec.Code)
	}

	rec = env.do(http.MethodGet, "/api/admin/dashboards", "", nil)
	if !strings.Contains(rec.Body.String(), `"protected":true`) || strings.Contains(rec.Body.String(), "$2") ||
		strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("list exposes secrets or misses protected: %s", rec.Body)
	}

	if rec := env.do(http.MethodPut, "/api/admin/dashboards/"+id, `{"slug":"initech-ops","title":"Initech"}`, h); rec.Code != http.StatusOK {
		t.Fatalf("rename = %d %s", rec.Code, rec.Body)
	}
	if rec := env.doCookies(http.MethodGet, "/api/dash/initech/status", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("old slug = %d", rec.Code)
	}
	if rec := env.do(http.MethodDelete, "/api/admin/dashboards/"+id+"?force=true", "", h); rec.Code != http.StatusNoContent {
		t.Fatalf("delete with force = %d", rec.Code)
	}
	if rec := env.doCookies(http.MethodGet, "/api/dash/initech-ops/status", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted dashboard = %d", rec.Code)
	}
}

func TestProtectedDashboardAccess(t *testing.T) {
	env := newTestEnv(t, true)
	env.login()
	over := env.createDashboard("initech", "Initech")
	bras := env.createDashboard("globex", "Globex")
	env.do(http.MethodPost, "/api/admin/dashboards/"+itoa(over.ID)+"/items", `{"title":"Secret"}`, env.csrfHeaders())
	env.setDashPassword(itoa(over.ID), "initech-secret")
	env.setDashPassword(itoa(bras.ID), "globex-secret")
	admin := env.cookie

	rec := env.doCookies(http.MethodGet, "/api/dash/initech/menu", "")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "dashboard locked") ||
		strings.Contains(rec.Body.String(), "Secret") || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("locked menu = %d %s %v", rec.Code, rec.Body, rec.Header())
	}
	rec = env.doCookies(http.MethodGet, "/api/dash/initech/status", "")
	if !strings.Contains(rec.Body.String(), `"protected":true,"unlocked":false`) {
		t.Fatalf("locked status = %s", rec.Body)
	}

	rec = env.doCookies(http.MethodPost, "/api/dash/initech/unlock", `{"password":"wrong-pass"}`)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "invalid password") || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("wrong unlock = %d %s", rec.Code, rec.Body)
	}

	viewer := env.unlock("initech", "initech-secret")
	rec = env.doCookies(http.MethodGet, "/api/dash/initech/menu", "", viewer)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Secret") {
		t.Fatalf("unlocked menu = %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "private, no-store" || rec.Header().Get("ETag") != "" {
		t.Fatalf("protected menu caching = %v", rec.Header())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/dash/initech/menu", nil)
	req.AddCookie(viewer)
	req.Header.Set("If-None-Match", "*")
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("If-None-Match on protected menu = %d", rr.Code)
	}

	// Scoped: the initech session does not open globex.
	if rec := env.doCookies(http.MethodGet, "/api/dash/globex/menu", "", viewer); rec.Code != http.StatusUnauthorized {
		t.Fatalf("viewer of initech on globex = %d", rec.Code)
	}
	// Tokens swapped between cookie names are rejected.
	asOther := &http.Cookie{Name: viewerCookieName(bras.ID), Value: viewer.Value}
	if rec := env.doCookies(http.MethodGet, "/api/dash/globex/menu", "", asOther); rec.Code != http.StatusUnauthorized {
		t.Fatalf("initech token as globex cookie = %d", rec.Code)
	}
	adminAsViewer := &http.Cookie{Name: viewerCookieName(over.ID), Value: admin.Value}
	if rec := env.doCookies(http.MethodGet, "/api/dash/initech/menu", "", adminAsViewer); rec.Code != http.StatusUnauthorized {
		t.Fatalf("admin token as viewer cookie = %d", rec.Code)
	}
	viewerAsAdmin := &http.Cookie{Name: sessionCookie, Value: viewer.Value}
	rec = env.doCookies(http.MethodGet, "/api/admin/session", "", viewerAsAdmin)
	if !strings.Contains(rec.Body.String(), `"authenticated":false,"csrf_token":null`) {
		t.Fatalf("viewer token as admin session = %s", rec.Body)
	}
	for _, p := range []string{"/api/admin/items", "/api/admin/dashboards", "/api/admin/export"} {
		if rec := env.doCookies(http.MethodGet, p, "", viewer, viewerAsAdmin); rec.Code != http.StatusUnauthorized {
			t.Fatalf("viewer on %s = %d", p, rec.Code)
		}
	}

	// Admin bypass.
	rec = env.doCookies(http.MethodGet, "/api/dash/initech/menu", "", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin bypass = %d", rec.Code)
	}
	if rec := env.doCookies(http.MethodGet, "/api/dash/initech/status", "", admin); !strings.Contains(rec.Body.String(), `"unlocked":true`) {
		t.Fatalf("admin status = %s", rec.Body)
	}

	// Changing the password re-locks the viewer.
	env.setDashPassword(itoa(over.ID), "initech-new")
	if rec := env.doCookies(http.MethodGet, "/api/dash/initech/menu", "", viewer); rec.Code != http.StatusUnauthorized {
		t.Fatalf("viewer after password change = %d", rec.Code)
	}

	// Lock revokes the session.
	viewer = env.unlock("initech", "initech-new")
	rec = env.doCookies(http.MethodPost, "/api/dash/initech/lock", "", viewer)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("lock = %d", rec.Code)
	}
	if rec := env.doCookies(http.MethodGet, "/api/dash/initech/menu", "", viewer); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after lock = %d", rec.Code)
	}

	// Clearing the password makes it public again.
	if rec := env.do(http.MethodDelete, "/api/admin/dashboards/"+itoa(over.ID)+"/password", "", env.csrfHeaders()); rec.Code != http.StatusNoContent {
		t.Fatalf("clear = %d", rec.Code)
	}
	if rec := env.doCookies(http.MethodGet, "/api/dash/initech/menu", ""); rec.Code != http.StatusOK {
		t.Fatalf("public again = %d", rec.Code)
	}
	if rec := env.doCookies(http.MethodPost, "/api/dash/initech/unlock", `{"password":"anything1"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unlock public dashboard = %d", rec.Code)
	}
}

func TestDefaultDashboardBecomesProtected(t *testing.T) {
	env := newTestEnv(t, true)
	env.login()
	rec := env.doCookies(http.MethodGet, "/api/menu", "")
	etag := rec.Header().Get("ETag")
	if rec.Code != http.StatusOK || etag == "" {
		t.Fatalf("public default = %d %v", rec.Code, rec.Header())
	}

	env.setDashPassword("default", "default-secret")
	for _, p := range []string{"/api/menu", "/api/dash/default/menu"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req.Header.Set("If-None-Match", etag)
		rr := httptest.NewRecorder()
		env.handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), "dashboard locked") {
			t.Fatalf("%s after protection = %d %s", p, rr.Code, rr.Body)
		}
	}
	viewer := env.unlock("default", "default-secret")
	if viewer.Name != viewerCookieName(menu.DefaultDashboardID) {
		t.Fatalf("viewer cookie = %q", viewer.Name)
	}
	if rec := env.doCookies(http.MethodGet, "/api/menu", "", viewer); rec.Code != http.StatusOK {
		t.Fatalf("unlocked default = %d", rec.Code)
	}
}

func TestUnlockLimits(t *testing.T) {
	env := newTestEnv(t, true)
	env.login()
	for _, slug := range []string{"initech", "globex"} {
		d := env.createDashboard(slug, slug)
		env.setDashPassword(itoa(d.ID), "some-secret")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/dash/initech/unlock", strings.NewReader(`{"password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("cross-origin unlock = %d", rr.Code)
	}

	big := `{"password":"` + strings.Repeat("a", maxUnlockBody) + `"}`
	if rec := env.doCookies(http.MethodPost, "/api/dash/initech/unlock", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized unlock = %d", rec.Code)
	}

	// The budget is per IP across dashboards (login_rate = 5).
	slugs := []string{"initech", "globex", "default", "initech", "globex"}
	for i, slug := range slugs {
		rec := env.doCookies(http.MethodPost, "/api/dash/"+slug+"/unlock", `{"password":"wrong-pass"}`)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d rate limited too early", i+1)
		}
	}
	if rec := env.doCookies(http.MethodPost, "/api/dash/initech/unlock", `{"password":"some-secret"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt = %d", rec.Code)
	}

	// Admin login keeps its own limiter.
	if rec := env.doCookies(http.MethodPost, "/api/admin/login", `{"password":"`+testPassword+`"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("login after unlock spam = %d", rec.Code)
	}
}

func TestDashboardRouting(t *testing.T) {
	env := newTestEnv(t, false)
	cases := []struct {
		path, location string
		code           int
	}{
		{"/dash", "/", http.StatusFound},
		{"/dash/", "/", http.StatusFound},
		{"/dash/default", "/", http.StatusFound},
		{"/dash/Default/", "/", http.StatusFound},
		{"/dash/Acme/", "/dash/acme", http.StatusMovedPermanently},
		{"/dash/acme/", "/dash/acme", http.StatusMovedPermanently},
		{"/dash/Initech?x=1", "/dash/initech?x=1", http.StatusMovedPermanently},
		{"/dash/acme", "", http.StatusOK},
		{"/dash/missing", "", http.StatusOK},
		{"/dash/acme/extra", "", http.StatusNotFound},
		{"/dash/-bad", "", http.StatusNotFound},
	}
	for _, tc := range cases {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := env.do(method, tc.path, "", nil)
			if rec.Code != tc.code || rec.Header().Get("Location") != tc.location {
				t.Errorf("%s %s = %d %q, want %d %q", method, tc.path, rec.Code, rec.Header().Get("Location"), tc.code, tc.location)
			}
			if tc.code == http.StatusOK && method == http.MethodGet && !strings.Contains(rec.Body.String(), "id=app") {
				t.Errorf("%s did not serve the SPA", tc.path)
			}
		}
	}
}
