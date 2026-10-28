package menu

import (
	"context"
	"errors"
	"testing"
)

func mustDashboard(t *testing.T, s *Service, slug, title string) Dashboard {
	t.Helper()
	d, err := s.CreateDashboard(context.Background(), DashboardInput{Slug: &slug, Title: title})
	if err != nil {
		t.Fatalf("create dashboard %q: %v", slug, err)
	}
	return d
}

func TestDashboardSlugValidation(t *testing.T) {
	s := newTestService(t)
	mustDashboard(t, s, "initech", "Initech")

	cases := []struct {
		name, slug string
	}{
		{"spaces and upper case", "Over Mind"},
		{"leading hyphen", "-x"},
		{"trailing hyphen", "x-"},
		{"too long", "a234567890123456789012345678901234567890b"},
		{"reserved admin", "admin"},
		{"reserved default", "default"},
		{"reserved dash", "dash"},
		{"duplicate ignoring case", "Initech"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CreateDashboard(context.Background(), DashboardInput{Slug: &tc.slug, Title: "X"})
			assertValidation(t, err, "slug")
		})
	}

	_, err := s.CreateDashboard(context.Background(), DashboardInput{Slug: ptr("ok"), Title: " "})
	assertValidation(t, err, "title")

	d := mustDashboard(t, s, "  Acme-Corp1 ", "Acme")
	if d.Slug == nil || *d.Slug != "acme-corp1" {
		t.Fatalf("slug not normalized: %v", d.Slug)
	}
}

func TestDefaultDashboard(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	list, err := s.ListDashboards(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || !list[0].IsDefault || list[0].Slug != nil {
		t.Fatalf("dashboards = %+v", list)
	}

	assertValidation(t, s.DeleteDashboard(ctx, DefaultDashboardID, true), "id")

	_, err = s.UpdateDashboard(ctx, DefaultDashboardID, DashboardInput{Slug: ptr("home"), Title: "Home"})
	assertValidation(t, err, "slug")

	d, err := s.UpdateDashboard(ctx, DefaultDashboardID, DashboardInput{Title: "Principal"})
	if err != nil || d.Title != "Principal" || d.Slug != nil {
		t.Fatalf("rename default: %+v, %v", d, err)
	}

	byDefault, err := s.DashboardBySlug(ctx, "DEFAULT")
	if err != nil || byDefault.ID != DefaultDashboardID {
		t.Fatalf("default sentinel: %+v, %v", byDefault, err)
	}
	if _, err := s.DashboardBySlug(ctx, "missing"); !errors.Is(err, ErrDashboardNotFound) {
		t.Fatalf("unknown slug err = %v", err)
	}
}

func TestDashboardIsolation(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	bio := mustDashboard(t, s, "acme", "Acme")
	over := mustDashboard(t, s, "initech", "Initech")

	a, err := s.Create(ctx, bio.ID, Input{Title: "A"}, ValidateOptions{})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := s.Create(ctx, over.ID, Input{Title: "B"}, ValidateOptions{})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}

	tree, err := s.Tree(ctx, bio.ID, true)
	if err != nil || len(tree) != 1 || tree[0].ID != a.ID {
		t.Fatalf("acme tree = %+v, %v", tree, err)
	}

	_, err = s.Create(ctx, bio.ID, Input{Title: "child", ParentID: &b.ID}, ValidateOptions{})
	assertValidation(t, err, "parent_id")

	err = s.Reorder(ctx, bio.ID, []Move{{ID: a.ID, Position: 1}, {ID: b.ID, Position: 0}})
	assertValidation(t, err, "id")
	if got, _ := s.Get(ctx, bio.ID, a.ID); got.Position != 0 {
		t.Fatal("partial reorder applied")
	}

	if _, err := s.Update(ctx, bio.ID, b.ID, Input{Title: "hijack"}, ValidateOptions{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-dashboard update err = %v", err)
	}
	if err := s.Delete(ctx, bio.ID, b.ID, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-dashboard delete err = %v", err)
	}
	if got, err := s.Get(ctx, over.ID, b.ID); err != nil || got.Title != "B" {
		t.Fatalf("item B changed: %+v, %v", got, err)
	}
}

func TestImportExportScopedByDashboard(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	bio := mustDashboard(t, s, "acme", "Acme")
	bras := mustDashboard(t, s, "globex", "Globex")

	for _, title := range []string{"Default 1", "Default 2", "Default 3"} {
		if _, err := s.Create(ctx, DefaultDashboardID, Input{Title: title, Position: 5}, ValidateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Create(ctx, bio.ID, Input{Title: "Bio old"}, ValidateOptions{}); err != nil {
		t.Fatal(err)
	}

	// Documents exported before multiple dashboards existed still import.
	doc := Document{Format: ExportFormat, Version: 1, Items: []*DocumentItem{
		{Title: "Imported", Items: []*DocumentItem{{Title: "Child"}}},
	}}

	if _, err := s.Import(ctx, bio.ID, doc, ImportReplace, ValidateOptions{}); err != nil {
		t.Fatalf("replace import: %v", err)
	}
	if items, _ := s.List(ctx, DefaultDashboardID); len(items) != 3 {
		t.Fatalf("replace import touched the default dashboard: %d items", len(items))
	}
	tree, _ := s.Tree(ctx, bio.ID, false)
	if len(tree) != 1 || tree[0].Title != "Imported" || len(tree[0].Items) != 1 {
		t.Fatalf("acme after replace = %+v", tree)
	}

	if _, err := s.Import(ctx, bras.ID, doc, ImportAppend, ValidateOptions{}); err != nil {
		t.Fatalf("append import: %v", err)
	}
	roots, _ := s.Tree(ctx, bras.ID, false)
	if len(roots) != 1 || roots[0].Position != 0 {
		t.Fatalf("append positions leaked from other dashboards: %+v", roots)
	}

	exported, err := s.Export(ctx, bio.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if exported.Version != ExportVersion || len(exported.Items) != 1 || exported.Items[0].Title != "Imported" {
		t.Fatalf("export = %+v", exported)
	}
}

func TestDeleteDashboard(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	over := mustDashboard(t, s, "initech", "Initech")
	if _, err := s.Create(ctx, over.ID, Input{Title: "X"}, ValidateOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteDashboard(ctx, over.ID, false); !errors.Is(err, ErrDashboardNotEmpty) {
		t.Fatalf("delete without force err = %v", err)
	}
	if err := s.DeleteDashboard(ctx, over.ID, true); err != nil {
		t.Fatalf("delete with force: %v", err)
	}
	if _, err := s.GetDashboard(ctx, over.ID); !errors.Is(err, ErrDashboardNotFound) {
		t.Fatalf("dashboard still exists: %v", err)
	}
	if items, _ := s.List(ctx, over.ID); len(items) != 0 {
		t.Fatalf("items survived: %d", len(items))
	}
}

func TestRenameSlug(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	over := mustDashboard(t, s, "initech", "Initech")
	mustDashboard(t, s, "taken", "Taken")

	_, err := s.UpdateDashboard(ctx, over.ID, DashboardInput{Slug: ptr("taken"), Title: "Initech"})
	assertValidation(t, err, "slug")

	if _, err := s.UpdateDashboard(ctx, over.ID, DashboardInput{Slug: ptr("initech-ops"), Title: "Initech"}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := s.DashboardBySlug(ctx, "initech"); !errors.Is(err, ErrDashboardNotFound) {
		t.Fatalf("old slug still resolves: %v", err)
	}
	if d, err := s.ResolveDashboard(ctx, "initech-ops"); err != nil || d.ID != over.ID {
		t.Fatalf("new slug: %+v, %v", d, err)
	}
}
