package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func addDashboard(t *testing.T, db *sql.DB, id int64, slug string) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO dashboards (id, slug, title) VALUES (?, ?, ?)", id, slug, slug); err != nil {
		t.Fatalf("insert dashboard: %v", err)
	}
}

func TestDashboardPasswordLength(t *testing.T) {
	m, _ := NewManager(openDB(t), Options{})
	ctx := context.Background()
	for _, pw := range []string{"short", strings.Repeat("x", 73)} {
		var pe *PasswordError
		if err := m.SetDashboardPassword(ctx, 1, pw); !errors.As(err, &pe) {
			t.Fatalf("password of %d bytes: err = %v", len(pw), err)
		}
	}
	if err := m.SetDashboardPassword(ctx, 1, strings.Repeat("x", 72)); err != nil {
		t.Fatalf("72 bytes: %v", err)
	}
	if err := m.SetDashboardPassword(ctx, 404, "long-enough"); !errors.Is(err, ErrUnknownDashboard) {
		t.Fatalf("unknown dashboard: %v", err)
	}
}

func TestViewerSessionsAreScoped(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	m, _ := NewManager(db, Options{})
	addDashboard(t, db, 2, "initech")
	addDashboard(t, db, 3, "globex")

	if _, _, err := m.Unlock(ctx, 2, "whatever"); !errors.Is(err, ErrNotProtected) {
		t.Fatalf("unlock public dashboard: %v", err)
	}
	for _, id := range []int64{2, 3} {
		if err := m.SetDashboardPassword(ctx, id, "secret-pass"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := m.Unlock(ctx, 2, "wrong-pass"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("wrong password: %v", err)
	}

	over, _, err := m.Unlock(ctx, 2, "secret-pass")
	if err != nil {
		t.Fatalf("unlock: %v", err)
	}
	bras, _, err := m.Unlock(ctx, 3, "secret-pass")
	if err != nil {
		t.Fatalf("unlock: %v", err)
	}
	admin, _, _ := m.CreateSession(ctx)

	if _, err := m.LookupViewer(ctx, over, 2); err != nil {
		t.Fatalf("viewer lookup: %v", err)
	}
	if _, err := m.LookupViewer(ctx, over, 3); !errors.Is(err, ErrNoSession) {
		t.Fatal("viewer session accepted for another dashboard")
	}
	if _, err := m.LookupAdmin(ctx, over); !errors.Is(err, ErrNoSession) {
		t.Fatal("viewer session accepted as admin")
	}
	if _, err := m.LookupViewer(ctx, admin, 2); !errors.Is(err, ErrNoSession) {
		t.Fatal("admin session accepted as viewer")
	}

	// Changing one dashboard password revokes only its viewers.
	if err := m.SetDashboardPassword(ctx, 2, "new-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LookupViewer(ctx, over, 2); !errors.Is(err, ErrNoSession) {
		t.Fatal("password change must revoke the dashboard's viewers")
	}
	if _, err := m.LookupViewer(ctx, bras, 3); err != nil {
		t.Fatalf("other dashboard viewer revoked: %v", err)
	}

	// The admin password does not touch viewers.
	if err := m.SetPassword(ctx, "admin-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LookupViewer(ctx, bras, 3); err != nil {
		t.Fatalf("admin password change revoked a viewer: %v", err)
	}
	if _, err := m.LookupAdmin(ctx, admin); !errors.Is(err, ErrNoSession) {
		t.Fatal("admin password change must revoke admin sessions")
	}

	if err := m.ClearDashboardPassword(ctx, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LookupViewer(ctx, bras, 3); !errors.Is(err, ErrNoSession) {
		t.Fatal("clearing the password must revoke viewers")
	}

	// RevokeViewer only removes a viewer session of that dashboard.
	again, _, _ := m.Unlock(ctx, 2, "new-secret")
	if err := m.RevokeViewer(ctx, again, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LookupViewer(ctx, again, 2); err != nil {
		t.Fatal("revoking with the wrong dashboard removed the session")
	}
	if err := m.RevokeViewer(ctx, again, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LookupViewer(ctx, again, 2); !errors.Is(err, ErrNoSession) {
		t.Fatal("lock did not revoke the session")
	}
}

func TestDashboardHashIsNeverExposedInSessions(t *testing.T) {
	db := openDB(t)
	m, _ := NewManager(db, Options{})
	if err := m.SetDashboardPassword(context.Background(), 1, "secret-pass"); err != nil {
		t.Fatal(err)
	}
	var hash string
	if err := db.QueryRow("SELECT password_hash FROM dashboards WHERE id = 1").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$2") || strings.Contains(hash, "secret-pass") {
		t.Fatalf("stored value is not a bcrypt hash: %q", hash)
	}
}

func TestUnlockRacingPasswordChange(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	admin, _ := NewManager(db, Options{})
	if err := admin.SetDashboardPassword(ctx, 1, "old-password"); err != nil {
		t.Fatal(err)
	}

	// The password changes after the old one was verified but before the
	// session is stored.
	viewer, _ := NewManager(db, Options{BeforeUnlockInsert: func() {
		if err := admin.SetDashboardPassword(ctx, 1, "new-password"); err != nil {
			t.Errorf("change password: %v", err)
		}
	}})
	if _, _, err := viewer.Unlock(ctx, 1, "old-password"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("unlock with a stale password: err = %v", err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions WHERE kind = 'viewer'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("viewer sessions = %d, err = %v", n, err)
	}
}
