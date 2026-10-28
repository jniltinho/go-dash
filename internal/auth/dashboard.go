package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	minDashboardPassword = 8
	// maxDashboardPassword is bcrypt's input limit in bytes.
	maxDashboardPassword = 72
)

var (
	// ErrNotProtected is returned when unlocking a dashboard without a password.
	ErrNotProtected = errors.New("dashboard is not protected")
	// ErrUnknownDashboard is returned for a dashboard id that does not exist.
	ErrUnknownDashboard = errors.New("dashboard not found")
)

// PasswordError describes a password that violates the length rules.
type PasswordError struct {
	Message string
}

func (e *PasswordError) Error() string { return e.Message }

// ValidateDashboardPassword checks the 8 to 72 byte rule shared by the API and the CLI.
func ValidateDashboardPassword(password string) error {
	if len(password) < minDashboardPassword || len(password) > maxDashboardPassword {
		return &PasswordError{Message: fmt.Sprintf("password must have %d to %d bytes", minDashboardPassword, maxDashboardPassword)}
	}
	return nil
}

// Unlock verifies a dashboard password and creates a viewer session for that
// dashboard only. The session is inserted only if the password did not
// change since it was verified, so a concurrent password change can never be
// outlived by a session obtained with the old password.
func (m *Manager) Unlock(ctx context.Context, dashboardID int64, password string) (string, Session, error) {
	var (
		hash    sql.NullString
		version int64
	)
	err := m.db.QueryRowContext(ctx,
		"SELECT password_hash, password_version FROM dashboards WHERE id = ?", dashboardID,
	).Scan(&hash, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return "", Session{}, ErrUnknownDashboard
	}
	if err != nil {
		return "", Session{}, fmt.Errorf("reading dashboard password: %w", err)
	}
	if !hash.Valid {
		return "", Session{}, ErrNotProtected
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash.String), []byte(password)); err != nil {
		return "", Session{}, ErrInvalidPassword
	}

	if m.beforeUnlockInsert != nil {
		m.beforeUnlockInsert()
	}

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
	res, err := m.db.ExecContext(ctx,
		`INSERT INTO sessions (id_hash, csrf_token, created_at, last_seen_at, expires_at, kind, dashboard_id)
		 SELECT ?, ?, ?, ?, ?, 'viewer', id FROM dashboards
		 WHERE id = ? AND password_version = ? AND password_hash IS NOT NULL`,
		hashID(id), csrf, now.Unix(), now.Unix(), expires.Unix(), dashboardID, version,
	)
	if err != nil {
		return "", Session{}, fmt.Errorf("creating viewer session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return "", Session{}, fmt.Errorf("reading affected rows: %w", err)
	}
	if n == 0 {
		return "", Session{}, ErrInvalidPassword
	}
	return id, Session{CSRFToken: csrf, ExpiresAt: expires}, nil
}

// SetDashboardPassword stores a new bcrypt hash for a dashboard and revokes
// its viewer sessions.
func (m *Manager) SetDashboardPassword(ctx context.Context, dashboardID int64, password string) error {
	if err := ValidateDashboardPassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}
	return m.replaceDashboardPassword(ctx, dashboardID, string(hash))
}

// ClearDashboardPassword makes a dashboard public and revokes its viewer sessions.
func (m *Manager) ClearDashboardPassword(ctx context.Context, dashboardID int64) error {
	return m.replaceDashboardPassword(ctx, dashboardID, nil)
}

func (m *Manager) replaceDashboardPassword(ctx context.Context, dashboardID int64, hash any) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`UPDATE dashboards SET password_hash = ?, password_version = password_version + 1,
		     updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = ?`,
		hash, dashboardID,
	)
	if err != nil {
		return fmt.Errorf("storing dashboard password: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("reading affected rows: %w", err)
	}
	if n == 0 {
		return ErrUnknownDashboard
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE kind = 'viewer' AND dashboard_id = ?", dashboardID); err != nil {
		return fmt.Errorf("revoking viewer sessions: %w", err)
	}
	return tx.Commit()
}
