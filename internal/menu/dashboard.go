package menu

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// DefaultDashboardID is the id of the default dashboard, created by migration 0003.
const DefaultDashboardID int64 = 1

// DefaultSlug is the API/CLI sentinel for the default dashboard. It is
// reserved, so no named dashboard can use it.
const DefaultSlug = "default"

var (
	// ErrDashboardNotFound is returned when a dashboard does not exist.
	ErrDashboardNotFound = errors.New("dashboard not found")
	// ErrDashboardNotEmpty is returned when deleting a dashboard with items without force.
	ErrDashboardNotEmpty = errors.New("dashboard has items; use force to delete them too")
)

var (
	slugPattern   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$`)
	reservedSlugs = map[string]bool{
		"admin": true, "api": true, "assets": true, "dash": true,
		DefaultSlug: true, "healthz": true, "login": true,
	}
)

// Dashboard is one set of menu items served at "/" (default) or "/dash/<slug>".
type Dashboard struct {
	ID int64 `json:"id"`
	// Slug is nil for the default dashboard.
	Slug *string `json:"slug"`
	// Title is empty on the default dashboard when the configured title applies.
	Title     string `json:"title"`
	IsDefault bool   `json:"is_default"`
	Protected bool   `json:"protected"`
	ItemCount int    `json:"item_count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// DashboardInput holds the writable fields of a dashboard.
type DashboardInput struct {
	Slug  *string `json:"slug"`
	Title string  `json:"title"`
}

// ValidSlugSyntax reports whether s is a well-formed lower-case slug
// (reserved names included).
func ValidSlugSyntax(s string) bool {
	return slugPattern.MatchString(s)
}

const selectDashboardColumns = `d.id, d.slug, d.title, d.is_default, d.password_hash IS NOT NULL,
	(SELECT COUNT(*) FROM menu_items m WHERE m.dashboard_id = d.id), d.created_at, d.updated_at`

func scanDashboard(scan func(dest ...any) error) (Dashboard, error) {
	var (
		d    Dashboard
		slug sql.NullString
	)
	if err := scan(&d.ID, &slug, &d.Title, &d.IsDefault, &d.Protected, &d.ItemCount, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return Dashboard{}, err
	}
	d.Slug = nullableString(slug)
	return d, nil
}

// ListDashboards returns the default dashboard first, then the others by title.
func (s *Service) ListDashboards(ctx context.Context) ([]Dashboard, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+selectDashboardColumns+" FROM dashboards d ORDER BY d.is_default DESC, d.title COLLATE NOCASE, d.id")
	if err != nil {
		return nil, fmt.Errorf("listing dashboards: %w", err)
	}
	defer rows.Close()

	list := []Dashboard{}
	for rows.Next() {
		d, err := scanDashboard(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scanning dashboard: %w", err)
		}
		list = append(list, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating dashboards: %w", err)
	}
	return list, nil
}

// GetDashboard returns one dashboard by id.
func (s *Service) GetDashboard(ctx context.Context, id int64) (Dashboard, error) {
	return s.queryDashboard(ctx, "d.id = ?", id)
}

// DashboardBySlug returns a dashboard by slug; DefaultSlug selects the
// default dashboard. The slug is matched case-insensitively.
func (s *Service) DashboardBySlug(ctx context.Context, slug string) (Dashboard, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == DefaultSlug {
		return s.GetDashboard(ctx, DefaultDashboardID)
	}
	if !ValidSlugSyntax(slug) {
		return Dashboard{}, ErrDashboardNotFound
	}
	return s.queryDashboard(ctx, "d.slug = ?", slug)
}

// ResolveDashboard accepts a numeric id, DefaultSlug or a slug.
func (s *Service) ResolveDashboard(ctx context.Context, ref string) (Dashboard, error) {
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		return s.GetDashboard(ctx, id)
	}
	return s.DashboardBySlug(ctx, ref)
}

func (s *Service) queryDashboard(ctx context.Context, where string, arg any) (Dashboard, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+selectDashboardColumns+" FROM dashboards d WHERE "+where, arg)
	d, err := scanDashboard(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Dashboard{}, ErrDashboardNotFound
	}
	if err != nil {
		return Dashboard{}, fmt.Errorf("getting dashboard: %w", err)
	}
	return d, nil
}

// CreateDashboard validates and stores a named dashboard.
func (s *Service) CreateDashboard(ctx context.Context, in DashboardInput) (Dashboard, error) {
	slug, title, err := normalizeDashboard(in, false)
	if err != nil {
		return Dashboard{}, err
	}
	if err := s.checkSlugFree(ctx, slug, 0); err != nil {
		return Dashboard{}, err
	}
	res, err := s.db.ExecContext(ctx, "INSERT INTO dashboards (slug, title) VALUES (?, ?)", slug, title)
	if err != nil {
		return Dashboard{}, fmt.Errorf("inserting dashboard: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Dashboard{}, fmt.Errorf("reading dashboard id: %w", err)
	}
	return s.GetDashboard(ctx, id)
}

// UpdateDashboard replaces title and slug. The default dashboard has no slug
// and may keep an empty title (the configured title applies).
func (s *Service) UpdateDashboard(ctx context.Context, id int64, in DashboardInput) (Dashboard, error) {
	current, err := s.GetDashboard(ctx, id)
	if err != nil {
		return Dashboard{}, err
	}
	slug, title, err := normalizeDashboard(in, current.IsDefault)
	if err != nil {
		return Dashboard{}, err
	}
	if !current.IsDefault {
		if err := s.checkSlugFree(ctx, slug, id); err != nil {
			return Dashboard{}, err
		}
	}

	var slugArg any
	if !current.IsDefault {
		slugArg = slug
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE dashboards SET slug = ?, title = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?`,
		slugArg, title, id)
	if err != nil {
		return Dashboard{}, fmt.Errorf("updating dashboard %d: %w", id, err)
	}
	return s.GetDashboard(ctx, id)
}

// DeleteDashboard removes a named dashboard; its items and viewer sessions
// go with it (foreign key cascade, one statement). Without force a dashboard
// that still has items is kept.
func (s *Service) DeleteDashboard(ctx context.Context, id int64, force bool) error {
	current, err := s.GetDashboard(ctx, id)
	if err != nil {
		return err
	}
	if current.IsDefault {
		return invalid("id", "the default dashboard cannot be deleted")
	}

	query := "DELETE FROM dashboards WHERE id = ?"
	if !force {
		query += " AND NOT EXISTS (SELECT 1 FROM menu_items WHERE dashboard_id = ?)"
	}
	args := []any{id}
	if !force {
		args = append(args, id)
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting dashboard %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("reading affected rows: %w", err)
	}
	if n == 0 {
		return ErrDashboardNotEmpty
	}
	return nil
}

func (s *Service) checkSlugFree(ctx context.Context, slug string, exceptID int64) error {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dashboards WHERE slug = ? AND id <> ?", slug, exceptID).Scan(&n)
	if err != nil {
		return fmt.Errorf("checking slug: %w", err)
	}
	if n > 0 {
		return invalid("slug", "slug %q is already in use", slug)
	}
	return nil
}

func normalizeDashboard(in DashboardInput, isDefault bool) (string, string, error) {
	title := strings.TrimSpace(in.Title)
	if utf8.RuneCountInString(title) > maxTitleLength {
		return "", "", invalid("title", "title must have at most %d characters", maxTitleLength)
	}

	if isDefault {
		if in.Slug != nil && strings.TrimSpace(*in.Slug) != "" {
			return "", "", invalid("slug", "the default dashboard has no slug")
		}
		return "", title, nil
	}

	if title == "" {
		return "", "", invalid("title", "title is required")
	}
	if in.Slug == nil {
		return "", "", invalid("slug", "slug is required")
	}
	slug := strings.ToLower(strings.TrimSpace(*in.Slug))
	if !ValidSlugSyntax(slug) {
		return "", "", invalid("slug", "slug must have 1 to 40 lower-case letters, digits or inner hyphens")
	}
	if reservedSlugs[slug] {
		return "", "", invalid("slug", "slug %q is reserved", slug)
	}
	return slug, title, nil
}
