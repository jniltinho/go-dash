package auth

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jniltinho/go-dash/internal/database"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.OpenAndMigrate(context.Background(), filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestDisabledWithoutPassword(t *testing.T) {
	m, _ := NewManager(openDB(t), Options{})
	enabled, err := m.Enabled(context.Background())
	if err != nil || enabled {
		t.Fatalf("enabled = %v, err = %v", enabled, err)
	}
	if err := m.CheckPassword(context.Background(), "x"); !errors.Is(err, ErrAdminDisabled) {
		t.Fatalf("got %v", err)
	}
}

func TestDatabasePasswordIsReadOnEveryCheck(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	server, _ := NewManager(db, Options{})
	cli, _ := NewManager(db, Options{})

	if err := cli.SetPassword(ctx, "first-password"); err != nil {
		t.Fatal(err)
	}
	if err := server.CheckPassword(ctx, "first-password"); err != nil {
		t.Fatal(err)
	}

	id, _, _ := server.CreateSession(ctx)
	if err := cli.SetPassword(ctx, "second-password"); err != nil {
		t.Fatal(err)
	}
	if err := server.CheckPassword(ctx, "first-password"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("old password must be rejected, got %v", err)
	}
	if err := server.CheckPassword(ctx, "second-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := server.LookupAdmin(ctx, id); !errors.Is(err, ErrNoSession) {
		t.Fatal("password change must revoke sessions")
	}
}

func TestEnvHashOverridesDatabase(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	cli, _ := NewManager(db, Options{})
	_ = cli.SetPassword(ctx, "database-password")

	hash, _ := bcrypt.GenerateFromPassword([]byte("env-password"), bcrypt.MinCost)
	m, err := NewManager(db, Options{EnvPasswordHash: string(hash), EnvPassword: "ignored-plain"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.CheckPassword(ctx, "database-password"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatal("database password must not be accepted when env hash is set")
	}
	if err := m.CheckPassword(ctx, "env-password"); err != nil {
		t.Fatal(err)
	}
	if src, _ := m.Source(ctx); src != SourceEnvHash {
		t.Fatalf("source = %q", src)
	}
}

func TestSessionExpiration(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0)
	m, _ := NewManager(db, Options{EnvPassword: "secret-pass", Now: func() time.Time { return now }})

	id, sess, err := m.CreateSession(ctx)
	if err != nil || sess.CSRFToken == "" {
		t.Fatal(err)
	}

	now = now.Add(time.Hour)
	if _, err := m.LookupAdmin(ctx, id); err != nil {
		t.Fatalf("session should be valid: %v", err)
	}

	now = now.Add(2 * time.Hour)
	if _, err := m.LookupAdmin(ctx, id); !errors.Is(err, ErrNoSession) {
		t.Fatal("idle session should expire")
	}

	// Absolute timeout even with continuous use.
	id, _, _ = m.CreateSession(ctx)
	for hour := range 11 {
		now = now.Add(time.Hour)
		if _, err := m.LookupAdmin(ctx, id); err != nil {
			t.Fatalf("session should be valid at hour %d: %v", hour+1, err)
		}
	}
	now = now.Add(time.Hour)
	if _, err := m.LookupAdmin(ctx, id); !errors.Is(err, ErrNoSession) {
		t.Fatal("session should hit absolute timeout")
	}
}

func TestRevokeAndCSRF(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	m, _ := NewManager(db, Options{EnvPassword: "secret-pass"})

	id1, s1, _ := m.CreateSession(ctx)
	_, s2, _ := m.CreateSession(ctx)
	if ValidCSRF(s1.CSRFToken, s2.CSRFToken) {
		t.Fatal("token from another session must not validate")
	}
	if !ValidCSRF(s1.CSRFToken, s1.CSRFToken) || ValidCSRF(s1.CSRFToken, "") {
		t.Fatal("csrf comparison wrong")
	}

	if err := m.RevokeAdmin(ctx, id1); err != nil {
		t.Fatal(err)
	}
	if _, err := m.LookupAdmin(ctx, id1); !errors.Is(err, ErrNoSession) {
		t.Fatal("revoked session must be invalid")
	}
}

func TestRateLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	rl := NewRateLimiter(5, 20, time.Minute)
	rl.now = func() time.Time { return now }

	for i := range 5 {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("6th attempt must be blocked")
	}
	if !rl.Allow("5.6.7.8") {
		t.Fatal("other IP should be allowed")
	}
	now = now.Add(time.Minute)
	if !rl.Allow("1.2.3.4") {
		t.Fatal("new window should allow")
	}
}
