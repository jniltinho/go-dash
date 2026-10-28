package cmd

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jniltinho/go-dash/internal/database"
	"github.com/jniltinho/go-dash/internal/menu"
)

// run executes a fresh command tree (cobra keeps flag state between runs).
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd(embed.FS{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestVersion(t *testing.T) {
	out, err := run(t, "version")
	if err != nil || !strings.Contains(out, "go-dash dev") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestUnknownCommand(t *testing.T) {
	if _, err := run(t, "foo"); err == nil {
		t.Fatal("unknown command must fail")
	}
}

func TestMenuLifecycle(t *testing.T) {
	db := filepath.Join(t.TempDir(), "cli.db")

	out, err := run(t, "--db", db, "menu", "add", "--title", "Monitoring")
	if err != nil {
		t.Fatal(err)
	}
	parent := strings.TrimSpace(out)

	if _, err := run(t, "--db", db, "menu", "add", "--title", "Grafana", "--url", "https://g.example.com", "--parent", parent); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "--db", db, "menu", "add", "--title", "x", "--url", "ftp://x"); err == nil {
		t.Fatal("invalid URL must fail")
	}

	if _, err := run(t, "--db", db, "menu", "add", "--title", "x", "--placement", "sidebar"); err == nil {
		t.Fatal("invalid placement must fail")
	}
	if _, err := run(t, "--db", db, "menu", "update", "2", "--placement", "card"); err != nil {
		t.Fatal(err)
	}

	out, err = run(t, "--db", db, "menu", "list", "--tree")
	if err != nil || !strings.Contains(out, "  ["+"2] Grafana") || !strings.Contains(out, "placement=card") {
		t.Fatalf("list out=%q err=%v", out, err)
	}

	if _, err := run(t, "--db", db, "menu", "remove", parent); err == nil {
		t.Fatal("remove with children must require --force")
	}
	if _, err := run(t, "--db", db, "menu", "update", "2", "--target", "blank"); err != nil {
		t.Fatal(err)
	}

	export := filepath.Join(t.TempDir(), "menu.json")
	if _, err := run(t, "--db", db, "menu", "export", "-o", export); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other.db")
	out, err = run(t, "--db", other, "menu", "import", export)
	if err != nil || !strings.Contains(out, "imported 2 items") {
		t.Fatalf("import out=%q err=%v", out, err)
	}
}

func TestSeedOnlyWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	doc := `{"format":"dashboard-menu","version":1,"items":[{"title":"Seeded"}]}`
	if err := os.WriteFile(seed, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := database.OpenAndMigrate(t.Context(), filepath.Join(dir, "seed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := menu.NewService(db)

	n, err := seedMenu(t.Context(), svc, seed, menu.ValidateOptions{})
	if err != nil || n != 1 {
		t.Fatalf("first seed n=%d err=%v", n, err)
	}
	n, err = seedMenu(t.Context(), svc, seed, menu.ValidateOptions{})
	if err != nil || n != 0 {
		t.Fatalf("second seed must be a no-op: n=%d err=%v", n, err)
	}
}

func TestDashLifecycle(t *testing.T) {
	db := filepath.Join(t.TempDir(), "dash.db")

	out, err := run(t, "--db", db, "dash", "add", "acme", "--title", "Acme")
	if err != nil || !strings.Contains(out, "/dash/acme") {
		t.Fatalf("add out=%q err=%v", out, err)
	}
	if _, err := run(t, "--db", db, "dash", "add", "admin", "--title", "X"); err == nil {
		t.Fatal("reserved slug must fail")
	}

	if _, err := run(t, "--db", db, "menu", "add", "--dash", "acme", "--title", "Bio link"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "--db", db, "menu", "add", "--title", "Default link"); err != nil {
		t.Fatal(err)
	}
	out, err = run(t, "--db", db, "menu", "list", "--dash", "acme")
	if err != nil || !strings.Contains(out, "Bio link") || strings.Contains(out, "Default link") {
		t.Fatalf("scoped list out=%q err=%v", out, err)
	}
	if _, err := run(t, "--db", db, "menu", "list", "--dash", "missing"); err == nil {
		t.Fatal("unknown dashboard must fail")
	}

	export := filepath.Join(t.TempDir(), "bio.json")
	if _, err := run(t, "--db", db, "menu", "export", "--dash", "acme", "-o", export); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "--db", db, "dash", "add", "globex", "--title", "Globex"); err != nil {
		t.Fatal(err)
	}
	out, err = run(t, "--db", db, "menu", "import", export, "--dash", "globex")
	if err != nil || !strings.Contains(out, "imported 1 items") {
		t.Fatalf("import out=%q err=%v", out, err)
	}

	cmd := newRootCmd(embed.FS{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(strings.NewReader("short\n"))
	cmd.SetArgs([]string{"--db", db, "dash", "set-password", "default", "--stdin"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("short password must fail")
	}
	cmd = newRootCmd(embed.FS{})
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(strings.NewReader("long-enough-secret\n"))
	cmd.SetArgs([]string{"--db", db, "dash", "set-password", "default", "--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-password: %v (%s)", err, buf.String())
	}

	out, err = run(t, "--db", db, "dash", "list")
	if err != nil || !strings.Contains(out, "default  path=/") || !strings.Contains(out, "protected") {
		t.Fatalf("list out=%q err=%v", out, err)
	}
	if _, err := run(t, "--db", db, "dash", "clear-password", "default"); err != nil {
		t.Fatal(err)
	}

	if _, err := run(t, "--db", db, "dash", "remove", "default"); err == nil {
		t.Fatal("removing the default dashboard must fail")
	}
	if _, err := run(t, "--db", db, "dash", "remove", "acme"); err == nil {
		t.Fatal("removing a non-empty dashboard must require --force")
	}
	if _, err := run(t, "--db", db, "dash", "update", "acme", "--slug", "bio"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "--db", db, "dash", "remove", "bio", "--force"); err != nil {
		t.Fatal(err)
	}
}

func TestSeedChecksOnlyDefaultDashboard(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	if err := os.WriteFile(seed, []byte(`{"format":"dashboard-menu","version":1,"items":[{"title":"Seeded"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := database.OpenAndMigrate(t.Context(), filepath.Join(dir, "seed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := menu.NewService(db)

	slug := "ops"
	d, err := svc.CreateDashboard(t.Context(), menu.DashboardInput{Slug: &slug, Title: "Ops"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(t.Context(), d.ID, menu.Input{Title: "Ops link"}, menu.ValidateOptions{}); err != nil {
		t.Fatal(err)
	}
	n, err := seedMenu(t.Context(), svc, seed, menu.ValidateOptions{})
	if err != nil || n != 1 {
		t.Fatalf("seed n=%d err=%v", n, err)
	}
	if items, _ := svc.List(t.Context(), d.ID); len(items) != 1 {
		t.Fatalf("seed touched another dashboard: %d items", len(items))
	}
}
