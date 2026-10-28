package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
)

func openTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenAndMigrate(context.Background(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func TestMigrateIsIdempotent(t *testing.T) {
	db, _ := openTestDB(t)

	applied, err := Migrate(context.Background(), db)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("expected no migrations on second run, got %v", applied)
	}
}

func TestPragmasOnEveryConnection(t *testing.T) {
	db, _ := openTestDB(t)
	db.SetMaxOpenConns(4)

	ctx := context.Background()
	conns := make([]*sql.Conn, 0, 4)
	for range 4 {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("conn: %v", err)
		}
		conns = append(conns, conn)
	}

	var wg sync.WaitGroup
	for _, conn := range conns {
		wg.Go(func() {
			var fk int
			if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
				t.Errorf("pragma: %v", err)
				return
			}
			if fk != 1 {
				t.Errorf("foreign_keys = %d, want 1", fk)
			}
		})
	}
	wg.Wait()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

func TestBackupContainsWALData(t *testing.T) {
	db, dir := openTestDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, "INSERT INTO menu_items (title) VALUES ('Grafana')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	out := filepath.Join(filepath.Dir(dir), "backup.db")
	if err := Backup(ctx, db, out); err != nil {
		t.Fatalf("backup: %v", err)
	}

	restored, err := Open(ctx, out)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer restored.Close()

	var check string
	if err := restored.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&check); err != nil || check != "ok" {
		t.Fatalf("integrity_check = %q, err = %v", check, err)
	}

	var title string
	if err := restored.QueryRowContext(ctx, "SELECT title FROM menu_items").Scan(&title); err != nil {
		t.Fatalf("select: %v", err)
	}
	if title != "Grafana" {
		t.Fatalf("title = %q", title)
	}
}

// foreignKeysOn asserts that a fresh pooled query sees foreign keys enabled.
func foreignKeysOn(t *testing.T, db *sql.DB) {
	t.Helper()
	var fk int
	if err := db.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d after migration, want 1", fk)
	}
}

func migrationApplied(t *testing.T, db *sql.DB, version string) bool {
	t.Helper()
	done, err := isApplied(context.Background(), db, version)
	if err != nil {
		t.Fatalf("isApplied: %v", err)
	}
	return done
}

func TestNoForeignKeysMigration(t *testing.T) {
	const base = "CREATE TABLE parent (id INTEGER PRIMARY KEY);\n" +
		"CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id));"

	tests := []struct {
		name    string
		body    string
		cancel  bool
		wantErr bool
	}{
		{name: "success", body: noForeignKeysDirective + "\nINSERT INTO parent (id) VALUES (1);\nINSERT INTO child (parent_id) VALUES (1);"},
		{name: "sql error", body: noForeignKeysDirective + "\nINSERT INTO parent (id) VALUES (2);\nNOT SQL;", wantErr: true},
		{name: "foreign key violation", body: noForeignKeysDirective + "\nINSERT INTO child (parent_id) VALUES (99);", wantErr: true},
		{name: "cancelled context", body: noForeignKeysDirective + "\nINSERT INTO parent (id) VALUES (3);", cancel: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Open(context.Background(), filepath.Join(t.TempDir(), "fk.db"))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })
			// A single pooled connection proves the migration connection is
			// handed back with foreign keys restored (or discarded).
			db.SetMaxOpenConns(1)

			fsys := fstest.MapFS{
				"0001_base.sql": {Data: []byte(base)},
				"0002_fk.sql":   {Data: []byte(tt.body)},
			}
			if _, err := migrate(context.Background(), db, fstest.MapFS{"0001_base.sql": fsys["0001_base.sql"]}); err != nil {
				t.Fatalf("base migration: %v", err)
			}

			ctx := context.Background()
			if tt.cancel {
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cctx
			}
			_, err = migrate(ctx, db, fsys)
			if (err != nil) != tt.wantErr {
				t.Fatalf("migrate error = %v, wantErr %v", err, tt.wantErr)
			}
			foreignKeysOn(t, db)
			if got := migrationApplied(t, db, "0002_fk"); got == tt.wantErr {
				t.Fatalf("0002 applied = %v, want %v", got, !tt.wantErr)
			}
			if !tt.wantErr {
				return
			}

			var n int
			if err := db.QueryRow("SELECT COUNT(*) FROM parent").Scan(&n); err != nil || n != 0 {
				t.Fatalf("failed migration left %d rows (err %v)", n, err)
			}
			// A fixed migration is retried on the next run.
			fsys["0002_fk.sql"] = &fstest.MapFile{Data: []byte(noForeignKeysDirective + "\nINSERT INTO parent (id) VALUES (5);")}
			if _, err := migrate(context.Background(), db, fsys); err != nil {
				t.Fatalf("retry: %v", err)
			}
			if !migrationApplied(t, db, "0002_fk") {
				t.Fatal("retry did not record the migration")
			}
			foreignKeysOn(t, db)
		})
	}
}

// openAt0002 creates a database migrated only up to 0002, as shipped before
// multiple dashboards existed.
func openAt0002(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	old := fstest.MapFS{}
	for _, name := range []string{"0001_init.sql", "0002_placement.sql"} {
		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		old[name] = &fstest.MapFile{Data: data}
	}
	if _, err := migrate(context.Background(), db, old); err != nil {
		t.Fatalf("migrate to 0002: %v", err)
	}
	return db
}

func TestUpgradeToDashboards(t *testing.T) {
	db := openAt0002(t)
	ctx := context.Background()

	stmts := []string{
		"INSERT INTO menu_items (id, title, position, placement) VALUES (1, 'Ops', 0, 'menu')",
		"INSERT INTO menu_items (id, parent_id, title, url, target, position, enabled) VALUES (2, 1, 'Grafana', 'https://grafana.example', 'blank', 3, 0)",
		"INSERT INTO menu_items (id, title, position) VALUES (9, 'Gone', 1)",
		"DELETE FROM menu_items WHERE id = 9",
		"INSERT INTO sessions (id_hash, csrf_token, created_at, last_seen_at, expires_at) VALUES ('h', 'c', 1, 1, 9999999999)",
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	applied, err := Migrate(ctx, db)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if len(applied) != 1 || applied[0] != "0003_dashboards" {
		t.Fatalf("applied = %v", applied)
	}
	foreignKeysOn(t, db)

	var (
		dash, parent, position   int
		title, target, placement string
		enabled                  bool
	)
	err = db.QueryRowContext(ctx,
		"SELECT dashboard_id, parent_id, title, target, placement, position, enabled FROM menu_items WHERE id = 2",
	).Scan(&dash, &parent, &title, &target, &placement, &position, &enabled)
	if err != nil {
		t.Fatalf("select item: %v", err)
	}
	if dash != 1 || parent != 1 || title != "Grafana" || target != "blank" || placement != "both" || position != 3 || enabled {
		t.Fatalf("item 2 changed: dash=%d parent=%d title=%q target=%q placement=%q position=%d enabled=%v",
			dash, parent, title, target, placement, position, enabled)
	}

	var kind string
	if err := db.QueryRowContext(ctx, "SELECT kind FROM sessions WHERE id_hash = 'h'").Scan(&kind); err != nil || kind != "admin" {
		t.Fatalf("session kind = %q, err = %v", kind, err)
	}

	// Ids deleted before the upgrade are not reused.
	res, err := db.ExecContext(ctx, "INSERT INTO menu_items (title) VALUES ('New')")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id, _ := res.LastInsertId(); id <= 9 {
		t.Fatalf("new id = %d, want > 9", id)
	}

	// Constraints still work.
	if _, err := db.ExecContext(ctx, "INSERT INTO menu_items (parent_id, title) VALUES (404, 'Orphan')"); err == nil {
		t.Fatal("unknown parent accepted")
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO menu_items (title, placement) VALUES ('Bad', 'x')"); err == nil {
		t.Fatal("invalid placement accepted")
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO menu_items (title, target) VALUES ('Bad', 'x')"); err == nil {
		t.Fatal("invalid target accepted")
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM menu_items WHERE id = 1"); err != nil {
		t.Fatalf("delete parent: %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM menu_items WHERE id = 2").Scan(&n); err != nil || n != 0 {
		t.Fatalf("child survived parent delete: n=%d err=%v", n, err)
	}

	// Dashboard deletion cascades to items and sessions.
	stmts = []string{
		"INSERT INTO dashboards (id, slug, title) VALUES (2, 'ops', 'Ops')",
		"INSERT INTO menu_items (dashboard_id, title) VALUES (2, 'Ops link')",
		"INSERT INTO sessions (id_hash, csrf_token, created_at, last_seen_at, expires_at, kind, dashboard_id) VALUES ('v', 'c', 1, 1, 9999999999, 'viewer', 2)",
		"DELETE FROM dashboards WHERE id = 2",
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.QueryRowContext(ctx,
		"SELECT (SELECT COUNT(*) FROM menu_items WHERE dashboard_id = 2) + (SELECT COUNT(*) FROM sessions WHERE dashboard_id = 2)",
	).Scan(&n); err != nil || n != 0 {
		t.Fatalf("dashboard delete left %d rows (err %v)", n, err)
	}

	assertForeignKeys(t, db)
}

// assertForeignKeys checks the rebuilt menu_items references.
func assertForeignKeys(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.Query("SELECT \"table\", \"from\", on_delete FROM pragma_foreign_key_list('menu_items') ORDER BY \"from\"")
	if err != nil {
		t.Fatalf("foreign_key_list: %v", err)
	}
	defer rows.Close()
	got := []string{}
	for rows.Next() {
		var table, from, onDelete string
		if err := rows.Scan(&table, &from, &onDelete); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, from+"->"+table+":"+onDelete)
	}
	want := []string{"dashboard_id->dashboards:CASCADE", "parent_id->menu_items:CASCADE"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("foreign keys = %v, want %v", got, want)
	}
}

func TestFreshInstallHasDefaultDashboard(t *testing.T) {
	db, _ := openTestDB(t)
	var slug sql.NullString
	var isDefault bool
	if err := db.QueryRow("SELECT slug, is_default FROM dashboards WHERE id = 1").Scan(&slug, &isDefault); err != nil {
		t.Fatalf("default dashboard: %v", err)
	}
	if slug.Valid || !isDefault {
		t.Fatalf("default dashboard slug=%v is_default=%v", slug, isDefault)
	}
	assertForeignKeys(t, db)
	foreignKeysOn(t, db)
}
