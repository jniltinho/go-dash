// Package auth implements the single administrator password, the optional
// per-dashboard passwords, server-side sessions (admin sessions with CSRF
// tokens and viewer sessions scoped to one dashboard), and login rate limiting.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	passwordSettingKey = "admin_password_hash"
	bcryptCost         = 12
	tokenBytes         = 32

	// DefaultIdleTimeout ends a session after this long without use.
	DefaultIdleTimeout = 2 * time.Hour
	// DefaultAbsoluteTimeout ends a session this long after creation.
	DefaultAbsoluteTimeout = 12 * time.Hour
)

var (
	// ErrInvalidPassword is returned for a wrong password.
	ErrInvalidPassword = errors.New("invalid password")
	// ErrAdminDisabled is returned when no password is configured.
	ErrAdminDisabled = errors.New("admin interface is disabled")
	// ErrNoSession is returned for a missing, expired or revoked session.
	ErrNoSession = errors.New("no valid session")
)

// PasswordSource identifies where the effective password hash comes from.
type PasswordSource string

const (
	SourceNone     PasswordSource = "none"
	SourceEnvHash  PasswordSource = "env-hash"
	SourceEnvPlain PasswordSource = "env-plain"
	SourceDatabase PasswordSource = "database"
)

// Options configures a Manager.
type Options struct {
	// EnvPasswordHash is DASHBOARD_ADMIN_PASSWORD_HASH (bcrypt).
	EnvPasswordHash string
	// EnvPassword is DASHBOARD_ADMIN_PASSWORD (plain text).
	EnvPassword     string
	IdleTimeout     time.Duration
	AbsoluteTimeout time.Duration
	// Now is overridable for tests.
	Now func() time.Time
	// BeforeUnlockInsert, when set, runs between password verification and
	// the session insert of Unlock (tests use it to inject a race).
	BeforeUnlockInsert func()
}

// Session is an authenticated admin session.
type Session struct {
	CSRFToken string
	ExpiresAt time.Time
}

// Manager handles passwords and sessions.
type Manager struct {
	db       *sql.DB
	envHash  []byte
	envPlain bool
	idle     time.Duration
	absolute time.Duration
	now      func() time.Time
	// beforeUnlockInsert is a test hook; see Options.BeforeUnlockInsert.
	beforeUnlockInsert func()
}

// NewManager creates a Manager. A plain-text env password is hashed once in memory.
func NewManager(db *sql.DB, opts Options) (*Manager, error) {
	m := &Manager{
		db:       db,
		idle:     opts.IdleTimeout,
		absolute: opts.AbsoluteTimeout,
		now:      opts.Now,

		beforeUnlockInsert: opts.BeforeUnlockInsert,
	}
	if m.idle <= 0 {
		m.idle = DefaultIdleTimeout
	}
	if m.absolute <= 0 {
		m.absolute = DefaultAbsoluteTimeout
	}
	if m.now == nil {
		m.now = time.Now
	}

	switch {
	case opts.EnvPasswordHash != "":
		if _, err := bcrypt.Cost([]byte(opts.EnvPasswordHash)); err != nil {
			return nil, fmt.Errorf("DASHBOARD_ADMIN_PASSWORD_HASH is not a valid bcrypt hash: %w", err)
		}
		m.envHash = []byte(opts.EnvPasswordHash)
	case opts.EnvPassword != "":
		hash, err := bcrypt.GenerateFromPassword([]byte(opts.EnvPassword), bcryptCost)
		if err != nil {
			return nil, fmt.Errorf("hashing DASHBOARD_ADMIN_PASSWORD: %w", err)
		}
		m.envHash = hash
		m.envPlain = true
	}
	return m, nil
}

// Source reports the effective password source. The database hash is read on
// every call so a CLI password change is seen by a running server.
func (m *Manager) Source(ctx context.Context) (PasswordSource, error) {
	if m.envHash != nil {
		if m.envPlain {
			return SourceEnvPlain, nil
		}
		return SourceEnvHash, nil
	}
	hash, err := m.storedHash(ctx)
	if err != nil {
		return SourceNone, err
	}
	if hash == nil {
		return SourceNone, nil
	}
	return SourceDatabase, nil
}

// Enabled reports whether an admin password is configured.
func (m *Manager) Enabled(ctx context.Context) (bool, error) {
	src, err := m.Source(ctx)
	return src != SourceNone, err
}

// EnvOverrides reports whether an environment password takes precedence over the database.
func (m *Manager) EnvOverrides() bool {
	return m.envHash != nil
}

func (m *Manager) storedHash(ctx context.Context) ([]byte, error) {
	var value string
	err := m.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", passwordSettingKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading admin password: %w", err)
	}
	return []byte(value), nil
}

// CheckPassword verifies password against the effective hash.
func (m *Manager) CheckPassword(ctx context.Context, password string) error {
	hash := m.envHash
	if hash == nil {
		stored, err := m.storedHash(ctx)
		if err != nil {
			return err
		}
		hash = stored
	}
	if hash == nil {
		return ErrAdminDisabled
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		return ErrInvalidPassword
	}
	return nil
}

// SetPassword stores a new bcrypt hash in the database and revokes all admin sessions.
func (m *Manager) SetPassword(ctx context.Context, password string) error {
	if len(password) < 8 {
		return errors.New("password must have at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		passwordSettingKey,
		string(hash),
	)
	if err != nil {
		return fmt.Errorf("storing password: %w", err)
	}
	// Viewer sessions depend on dashboard passwords, not on this one.
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE kind = 'admin'"); err != nil {
		return fmt.Errorf("revoking sessions: %w", err)
	}
	return tx.Commit()
}

// CreateSession creates a new admin session and returns the raw session ID
// (for the cookie) and the session data. Only the ID hash is stored.
func (m *Manager) CreateSession(ctx context.Context) (string, Session, error) {
	id, err := randomToken()
	if err != nil {
		return "", Session{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return "", Session{}, err
	}

	now := m.now()
	expires := now.Add(m.idle)
	_, err = m.db.ExecContext(
		ctx,
		"INSERT INTO sessions (id_hash, csrf_token, created_at, last_seen_at, expires_at, kind) VALUES (?, ?, ?, ?, ?, 'admin')",
		hashID(id),
		csrf,
		now.Unix(),
		now.Unix(),
		expires.Unix(),
	)
	if err != nil {
		return "", Session{}, fmt.Errorf("creating session: %w", err)
	}
	return id, Session{CSRFToken: csrf, ExpiresAt: expires}, nil
}

// LookupAdmin validates an admin session ID, enforcing idle and absolute
// expiration, and slides the idle window. Viewer sessions are rejected.
func (m *Manager) LookupAdmin(ctx context.Context, id string) (Session, error) {
	return m.lookup(ctx, id, "kind = 'admin'")
}

// LookupViewer validates a viewer session of dashboardID. Admin sessions and
// viewer sessions of other dashboards are rejected.
func (m *Manager) LookupViewer(ctx context.Context, id string, dashboardID int64) (Session, error) {
	return m.lookup(ctx, id, "kind = 'viewer' AND dashboard_id = ?", dashboardID)
}

// lookup validates a session matching the scope condition. Expired sessions
// are deleted.
func (m *Manager) lookup(ctx context.Context, id, scope string, scopeArgs ...any) (Session, error) {
	if id == "" {
		return Session{}, ErrNoSession
	}
	idHash := hashID(id)

	var (
		csrf      string
		createdAt int64
		lastSeen  int64
	)
	args := append([]any{idHash}, scopeArgs...)
	err := m.db.QueryRowContext(
		ctx,
		"SELECT csrf_token, created_at, last_seen_at FROM sessions WHERE id_hash = ? AND "+scope,
		args...,
	).Scan(&csrf, &createdAt, &lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("reading session: %w", err)
	}

	now := m.now()
	idleExpired := now.Sub(time.Unix(lastSeen, 0)) >= m.idle
	absoluteExpired := now.Sub(time.Unix(createdAt, 0)) >= m.absolute
	if idleExpired || absoluteExpired {
		_, _ = m.db.ExecContext(ctx, "DELETE FROM sessions WHERE id_hash = ?", idHash)
		return Session{}, ErrNoSession
	}

	expires := now.Add(m.idle)
	if absEnd := time.Unix(createdAt, 0).Add(m.absolute); absEnd.Before(expires) {
		expires = absEnd
	}
	_, err = m.db.ExecContext(
		ctx,
		"UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id_hash = ?",
		now.Unix(),
		expires.Unix(),
		idHash,
	)
	if err != nil {
		return Session{}, fmt.Errorf("touching session: %w", err)
	}
	return Session{CSRFToken: csrf, ExpiresAt: expires}, nil
}

// RevokeAdmin deletes one admin session.
func (m *Manager) RevokeAdmin(ctx context.Context, id string) error {
	if _, err := m.db.ExecContext(ctx, "DELETE FROM sessions WHERE id_hash = ? AND kind = 'admin'", hashID(id)); err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	return nil
}

// RevokeViewer deletes one viewer session of dashboardID.
func (m *Manager) RevokeViewer(ctx context.Context, id string, dashboardID int64) error {
	_, err := m.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE id_hash = ? AND kind = 'viewer' AND dashboard_id = ?", hashID(id), dashboardID)
	if err != nil {
		return fmt.Errorf("revoking viewer session: %w", err)
	}
	return nil
}

// PurgeExpired deletes expired sessions.
func (m *Manager) PurgeExpired(ctx context.Context) error {
	if _, err := m.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", m.now().Unix()); err != nil {
		return fmt.Errorf("purging sessions: %w", err)
	}
	return nil
}

// ValidCSRF compares tokens in constant time.
func ValidCSRF(expected, got string) bool {
	if expected == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(got)) == 1
}

func randomToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashID(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}
