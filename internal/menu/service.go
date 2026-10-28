package menu

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxTitleLength = 120

// Service holds the menu business rules on top of the database.
type Service struct {
	db *sql.DB
}

// NewService creates a menu service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ValidateOptions carries request-dependent validation context.
type ValidateOptions struct {
	// ForbiddenIframeOrigins lists origins (scheme://host[:port]) of the
	// dashboard itself; iframe items must not point to them.
	ForbiddenIframeOrigins []string
}

// List returns all items of a dashboard ordered by position.
func (s *Service) List(ctx context.Context, dashboardID int64) ([]Item, error) {
	return listItems(ctx, s.db, dashboardID)
}

// Get returns one item of a dashboard.
func (s *Service) Get(ctx context.Context, dashboardID, id int64) (Item, error) {
	return getItem(ctx, s.db, dashboardID, id)
}

// Tree returns the item hierarchy. When onlyEnabled is true, disabled items
// and their whole branch are omitted.
func (s *Service) Tree(ctx context.Context, dashboardID int64, onlyEnabled bool) ([]*Node, error) {
	items, err := listItems(ctx, s.db, dashboardID)
	if err != nil {
		return nil, err
	}
	return BuildTree(items, onlyEnabled), nil
}

// Create validates and persists a new item in a dashboard.
func (s *Service) Create(ctx context.Context, dashboardID int64, in Input, opts ValidateOptions) (Item, error) {
	in, err := s.normalize(ctx, s.db, dashboardID, in, opts)
	if err != nil {
		return Item{}, err
	}

	id, err := insertItem(ctx, s.db, dashboardID, in)
	if err != nil {
		return Item{}, err
	}
	return getItem(ctx, s.db, dashboardID, id)
}

// Update fully replaces an item.
func (s *Service) Update(ctx context.Context, dashboardID, id int64, in Input, opts ValidateOptions) (Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := getItem(ctx, tx, dashboardID, id); err != nil {
		return Item{}, err
	}

	in, err = s.normalize(ctx, tx, dashboardID, in, opts)
	if err != nil {
		return Item{}, err
	}

	if in.ParentID != nil {
		items, err := listItems(ctx, tx, dashboardID)
		if err != nil {
			return Item{}, err
		}
		moves := []Move{{ID: id, ParentID: in.ParentID, Position: in.Position}}
		if err := checkNoCycles(items, moves); err != nil {
			return Item{}, err
		}
	}

	if err := updateItem(ctx, tx, dashboardID, id, in); err != nil {
		return Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return Item{}, fmt.Errorf("committing update: %w", err)
	}
	return getItem(ctx, s.db, dashboardID, id)
}

// Delete removes an item and, by cascade, all its descendants. When force is
// false and the item has children, it fails without deleting anything.
func (s *Service) Delete(ctx context.Context, dashboardID, id int64, force bool) error {
	if !force {
		n, err := countChildren(ctx, s.db, dashboardID, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return invalid("id", "item has %d children; use force to delete them too", n)
		}
	}
	return deleteItem(ctx, s.db, dashboardID, id)
}

// Reorder applies a batch of moves atomically after validating the final
// tree. Items and parents must belong to dashboardID.
func (s *Service) Reorder(ctx context.Context, dashboardID int64, moves []Move) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	items, err := listItems(ctx, tx, dashboardID)
	if err != nil {
		return err
	}
	if err := checkNoCycles(items, moves); err != nil {
		return err
	}

	for _, m := range moves {
		if err := moveItem(ctx, tx, dashboardID, m); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing reorder: %w", err)
	}
	return nil
}

// normalize trims and validates input, applying defaults.
func (s *Service) normalize(ctx context.Context, q querier, dashboardID int64, in Input, opts ValidateOptions) (Input, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return in, invalid("title", "title is required")
	}
	if utf8.RuneCountInString(in.Title) > maxTitleLength {
		return in, invalid("title", "title must have at most %d characters", maxTitleLength)
	}

	if in.Target == "" {
		in.Target = TargetIframe
	}
	if !in.Target.Valid() {
		return in, invalid("target", "target must be %q or %q", TargetIframe, TargetBlank)
	}

	if in.Placement == "" {
		in.Placement = PlacementBoth
	}
	if !in.Placement.Valid() {
		return in, invalid("placement", "placement must be %q, %q or %q", PlacementMenu, PlacementCard, PlacementBoth)
	}

	in.URL = trimOptional(in.URL)
	in.Icon = trimOptional(in.Icon)
	in.Description = trimOptional(in.Description)

	if in.URL != nil {
		origin, err := validateURL(*in.URL)
		if err != nil {
			return in, err
		}
		if in.Target == TargetIframe && containsFold(opts.ForbiddenIframeOrigins, origin) {
			return in, invalid("url", "the dashboard's own origin cannot be opened in an iframe; use target %q", TargetBlank)
		}
	}

	if in.Enabled == nil {
		enabled := true
		in.Enabled = &enabled
	}

	if in.ParentID != nil {
		// The parent must belong to the same dashboard.
		if _, err := getItem(ctx, q, dashboardID, *in.ParentID); err != nil {
			if err == ErrNotFound {
				return in, invalid("parent_id", "parent %d does not exist", *in.ParentID)
			}
			return in, err
		}
	}

	return in, nil
}

// validateURL checks that raw is an absolute http(s) URL and returns its origin.
func validateURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", invalid("url", "invalid URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", invalid("url", "URL must use http or https")
	}
	if u.Hostname() == "" {
		return "", invalid("url", "URL must be absolute")
	}
	return NormalizeOrigin(scheme, u.Host), nil
}

// NormalizeOrigin returns scheme://host[:port] in lower case, the way
// browsers serialize origins: the port is parsed numerically ("080" -> 80)
// and dropped when empty or equal to the scheme's default.
func NormalizeOrigin(scheme, host string) string {
	scheme = strings.ToLower(scheme)
	host = strings.ToLower(host)

	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		// No port; a trailing ":" (empty port) is also dropped.
		return scheme + "://" + strings.TrimSuffix(host, ":")
	}
	if strings.Contains(hostname, ":") {
		hostname = "[" + hostname + "]"
	}
	if port == "" {
		return scheme + "://" + hostname
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return scheme + "://" + host
	}
	if (scheme == "http" && n == 80) || (scheme == "https" && n == 443) {
		return scheme + "://" + hostname
	}
	return scheme + "://" + hostname + ":" + strconv.Itoa(n)
}

// Origin returns the normalized origin of a URL, or "" if it is invalid.
func Origin(raw string) string {
	origin, err := validateURL(raw)
	if err != nil {
		return ""
	}
	return origin
}

func trimOptional(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
