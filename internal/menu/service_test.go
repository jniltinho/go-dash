package menu

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/jniltinho/go-dash/internal/database"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := database.OpenAndMigrate(context.Background(), filepath.Join(t.TempDir(), "menu.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(db)
}

func ptr[T any](v T) *T { return &v }

func mustCreate(t *testing.T, s *Service, in Input) Item {
	t.Helper()
	item, err := s.Create(context.Background(), DefaultDashboardID, in, ValidateOptions{})
	if err != nil {
		t.Fatalf("create %q: %v", in.Title, err)
	}
	return item
}

func assertValidation(t *testing.T, err error, field string) {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected validation error on %q, got %v", field, err)
	}
	if ve.Field != field {
		t.Fatalf("validation field = %q, want %q", ve.Field, field)
	}
}

func TestCreateDefaultsAndNesting(t *testing.T) {
	s := newTestService(t)
	group := mustCreate(t, s, Input{Title: "Monitoring"})
	child := mustCreate(t, s, Input{Title: "Grafana", URL: ptr("https://grafana.example.com"), ParentID: &group.ID})

	if child.Target != TargetIframe {
		t.Fatalf("default target = %q", child.Target)
	}
	if child.Placement != PlacementBoth {
		t.Fatalf("default placement = %q", child.Placement)
	}
	if !child.Enabled {
		t.Fatal("default enabled should be true")
	}
	if child.ParentID == nil || *child.ParentID != group.ID {
		t.Fatal("child not linked to parent")
	}
}

func TestValidation(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	cases := []struct {
		name  string
		in    Input
		opts  ValidateOptions
		field string
	}{
		{"javascript url", Input{Title: "x", URL: ptr("javascript:alert(1)")}, ValidateOptions{}, "url"},
		{"relative url", Input{Title: "x", URL: ptr("/admin")}, ValidateOptions{}, "url"},
		{"empty title", Input{Title: "   "}, ValidateOptions{}, "title"},
		{"bad target", Input{Title: "x", Target: "popup"}, ValidateOptions{}, "target"},
		{"bad placement", Input{Title: "x", Placement: "sidebar"}, ValidateOptions{}, "placement"},
		{"unknown parent", Input{Title: "x", ParentID: ptr(int64(999))}, ValidateOptions{}, "parent_id"},
		{
			"same origin iframe",
			Input{Title: "x", URL: ptr("https://dash.example.com/admin")},
			ValidateOptions{ForbiddenIframeOrigins: []string{"https://dash.example.com"}},
			"url",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Create(ctx, DefaultDashboardID, tc.in, tc.opts)
			assertValidation(t, err, tc.field)
		})
	}

	// Default ports are normalized away.
	_, err := s.Create(ctx, DefaultDashboardID, Input{Title: "x", URL: ptr("https://dash.example.com:443/admin")},
		ValidateOptions{ForbiddenIframeOrigins: []string{"https://dash.example.com"}})
	assertValidation(t, err, "url")

	// Same origin is allowed when opened in a new tab.
	_, err = s.Create(ctx, DefaultDashboardID, Input{Title: "x", URL: ptr("https://dash.example.com/admin"), Target: TargetBlank},
		ValidateOptions{ForbiddenIframeOrigins: []string{"https://dash.example.com"}})
	if err != nil {
		t.Fatalf("blank same-origin should be allowed: %v", err)
	}
}

func TestTreeOrderingAndDisabledBranch(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	group := mustCreate(t, s, Input{Title: "Monitoring", Position: 1})
	mustCreate(t, s, Input{Title: "Grafana", ParentID: &group.ID, URL: ptr("https://g.example.com")})
	mustCreate(t, s, Input{Title: "C", Position: 2})
	mustCreate(t, s, Input{Title: "A", Position: 0})

	tree, err := s.Tree(ctx, DefaultDashboardID, true)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{tree[0].Title, tree[1].Title, tree[2].Title}
	if got[0] != "A" || got[1] != "Monitoring" || got[2] != "C" {
		t.Fatalf("order = %v", got)
	}

	disabled := false
	in := Input{Title: "Monitoring", Position: 1, Enabled: &disabled}
	if _, err := s.Update(ctx, DefaultDashboardID, group.ID, in, ValidateOptions{}); err != nil {
		t.Fatal(err)
	}
	tree, _ = s.Tree(ctx, DefaultDashboardID, true)
	for _, n := range tree {
		if n.Title == "Monitoring" {
			t.Fatal("disabled branch should be hidden")
		}
	}

	full, _ := s.Tree(ctx, DefaultDashboardID, false)
	if len(full) != 3 {
		t.Fatalf("admin tree should include disabled items, got %d roots", len(full))
	}
}

func TestEmptyTreeIsNotNil(t *testing.T) {
	s := newTestService(t)
	tree, err := s.Tree(context.Background(), DefaultDashboardID, true)
	if err != nil || tree == nil || len(tree) != 0 {
		t.Fatalf("tree = %v, err = %v", tree, err)
	}
}

func TestUpdateReplacesOptionalFields(t *testing.T) {
	s := newTestService(t)
	item := mustCreate(t, s, Input{Title: "Wiki", Icon: ptr("mdi mdi-book")})

	updated, err := s.Update(context.Background(), DefaultDashboardID, item.ID, Input{Title: "Wiki", Target: TargetBlank}, ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Icon != nil {
		t.Fatal("omitted icon should become nil")
	}
	if updated.Target != TargetBlank {
		t.Fatalf("target = %q", updated.Target)
	}

	_, err = s.Update(context.Background(), DefaultDashboardID, 999, Input{Title: "x"}, ValidateOptions{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCyclePrevention(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	a := mustCreate(t, s, Input{Title: "A"})
	b := mustCreate(t, s, Input{Title: "B", ParentID: &a.ID})

	_, err := s.Update(ctx, DefaultDashboardID, a.ID, Input{Title: "A", ParentID: &b.ID}, ValidateOptions{})
	assertValidation(t, err, "parent_id")

	_, err = s.Update(ctx, DefaultDashboardID, a.ID, Input{Title: "A", ParentID: &a.ID}, ValidateOptions{})
	assertValidation(t, err, "parent_id")

	c := mustCreate(t, s, Input{Title: "C"})
	err = s.Reorder(ctx, DefaultDashboardID, []Move{
		{ID: c.ID, ParentID: &a.ID, Position: 0},
		{ID: a.ID, ParentID: &c.ID, Position: 0},
	})
	assertValidation(t, err, "parent_id")

	// Nothing from the failed batch was applied.
	got, _ := s.Get(ctx, DefaultDashboardID, c.ID)
	if got.ParentID != nil {
		t.Fatal("failed reorder must not be applied")
	}
}

func TestReorder(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	a := mustCreate(t, s, Input{Title: "A", Position: 0})
	b := mustCreate(t, s, Input{Title: "B", Position: 1})

	err := s.Reorder(ctx, DefaultDashboardID, []Move{{ID: a.ID, Position: 1}, {ID: b.ID, Position: 0}})
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := s.Tree(ctx, DefaultDashboardID, false)
	if tree[0].Title != "B" {
		t.Fatalf("first = %q", tree[0].Title)
	}
}

func TestDeleteCascadeAndForce(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	group := mustCreate(t, s, Input{Title: "G"})
	mustCreate(t, s, Input{Title: "C1", ParentID: &group.ID})
	mustCreate(t, s, Input{Title: "C2", ParentID: &group.ID})

	assertValidation(t, s.Delete(ctx, DefaultDashboardID, group.ID, false), "id")

	if err := s.Delete(ctx, DefaultDashboardID, group.ID, true); err != nil {
		t.Fatal(err)
	}
	items, _ := s.List(ctx, DefaultDashboardID)
	if len(items) != 0 {
		t.Fatalf("expected cascade delete, %d items remain", len(items))
	}
	if err := s.Delete(ctx, DefaultDashboardID, group.ID, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	src := newTestService(t)
	ctx := context.Background()
	g := mustCreate(t, src, Input{Title: "Infra", Icon: ptr("mdi mdi-server"), Description: ptr("infra tools")})
	b := mustCreate(t, src, Input{Title: "Bancos", ParentID: &g.ID, Position: 3})
	off := false
	mustCreate(t, src, Input{
		Title:     "pgAdmin",
		ParentID:  &b.ID,
		URL:       ptr("https://pg.example.com"),
		Target:    TargetBlank,
		Placement: PlacementCard,
		Enabled:   &off,
	})
	mustCreate(t, src, Input{Title: "Menu only", Placement: PlacementMenu})

	doc, err := src.Export(ctx, DefaultDashboardID)
	if err != nil {
		t.Fatal(err)
	}

	dst := newTestService(t)
	n, err := dst.Import(ctx, DefaultDashboardID, doc, ImportReplace, ValidateOptions{})
	if err != nil || n != 4 {
		t.Fatalf("import n=%d err=%v", n, err)
	}

	again, _ := dst.Export(ctx, DefaultDashboardID)
	if !equalDocItems(doc.Items, again.Items) {
		t.Fatal("round trip changed the tree")
	}
}

func equalDocItems(a, b []*DocumentItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		same := x.Title == y.Title && eqPtr(x.URL, y.URL) && x.Target == y.Target && x.Placement == y.Placement &&
			eqPtr(x.Icon, y.Icon) &&
			eqPtr(x.Description, y.Description) && eqPtr(x.Enabled, y.Enabled) && x.Position == y.Position
		if !same || !equalDocItems(x.Items, y.Items) {
			return false
		}
	}
	return true
}

func eqPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func TestImportAppendAndRollback(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	for i := range 3 {
		mustCreate(t, s, Input{Title: "Existing", Position: i})
	}

	doc := Document{Format: ExportFormat, Version: 1, Items: []*DocumentItem{{Title: "N1"}, {Title: "N2", Position: 1}}}
	if _, err := s.Import(ctx, DefaultDashboardID, doc, ImportAppend, ValidateOptions{}); err != nil {
		t.Fatal(err)
	}
	tree, _ := s.Tree(ctx, DefaultDashboardID, false)
	if len(tree) != 5 || tree[3].Title != "N1" || tree[4].Title != "N2" {
		t.Fatalf("append result wrong: %d roots", len(tree))
	}

	neg := Document{Format: ExportFormat, Version: 1, Items: []*DocumentItem{{Title: "Neg", Position: -10}}}
	if _, err := s.Import(ctx, DefaultDashboardID, neg, ImportAppend, ValidateOptions{}); err != nil {
		t.Fatal(err)
	}
	tree, _ = s.Tree(ctx, DefaultDashboardID, false)
	if tree[len(tree)-1].Title != "Neg" {
		t.Fatalf("appended item with negative position must go last, got %q", tree[len(tree)-1].Title)
	}

	bad := Document{Format: ExportFormat, Version: 1, Items: []*DocumentItem{{Title: "ok"}, {Title: "bad", URL: ptr("ftp://x")}}}
	_, err := s.Import(ctx, DefaultDashboardID, bad, ImportReplace, ValidateOptions{})
	assertValidation(t, err, "url")
	items, _ := s.List(ctx, DefaultDashboardID)
	if len(items) != 6 {
		t.Fatalf("failed replace must keep data, got %d items", len(items))
	}

	_, err = s.Import(ctx, DefaultDashboardID, Document{Format: ExportFormat, Version: 2}, ImportAppend, ValidateOptions{})
	assertValidation(t, err, "version")
}

func TestNormalizeOrigin(t *testing.T) {
	cases := []struct{ scheme, host, want string }{
		{"https", "Dash.Example.com:443", "https://dash.example.com"},
		{"https", "dash.example.com:0443", "https://dash.example.com"},
		{"http", "dash.example.com:080", "http://dash.example.com"},
		{"http", "dash.example.com:", "http://dash.example.com"},
		{"http", "dash.example.com:08080", "http://dash.example.com:8080"},
		{"HTTPS", "[::1]:443", "https://[::1]"},
		{"http", "localhost:8080", "http://localhost:8080"},
	}
	for _, tc := range cases {
		if got := NormalizeOrigin(tc.scheme, tc.host); got != tc.want {
			t.Errorf("NormalizeOrigin(%q, %q) = %q, want %q", tc.scheme, tc.host, got, tc.want)
		}
	}
}

func TestPlacementDefaultsForExistingRows(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	// Rows written before migration 0002 (or by raw SQL) get "both".
	if _, err := s.db.ExecContext(ctx, "INSERT INTO menu_items (title) VALUES ('Legacy')"); err != nil {
		t.Fatal(err)
	}
	items, _ := s.List(ctx, DefaultDashboardID)
	if items[0].Placement != PlacementBoth {
		t.Fatalf("placement = %q", items[0].Placement)
	}
}

func TestImportWithoutPlacementDefaultsToBoth(t *testing.T) {
	s := newTestService(t)
	doc := Document{Format: ExportFormat, Version: 1, Items: []*DocumentItem{{Title: "Old export"}}}
	if _, err := s.Import(context.Background(), DefaultDashboardID, doc, ImportAppend, ValidateOptions{}); err != nil {
		t.Fatal(err)
	}
	items, _ := s.List(context.Background(), DefaultDashboardID)
	if items[0].Placement != PlacementBoth {
		t.Fatalf("placement = %q", items[0].Placement)
	}
}
