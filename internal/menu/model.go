// Package menu implements the hierarchical menu items domain: validation,
// persistence, tree building, reordering and import/export.
package menu

import (
	"errors"
	"fmt"
)

// Target defines how a menu item URL is opened by the UI.
type Target string

const (
	// TargetIframe opens the URL in a tab of the central Tabbar.
	TargetIframe Target = "iframe"
	// TargetBlank opens the URL in a new browser tab.
	TargetBlank Target = "blank"
)

// Valid reports whether t is a known target.
func (t Target) Valid() bool {
	return t == TargetIframe || t == TargetBlank
}

// Placement defines where an item is shown in the dashboard.
type Placement string

const (
	// PlacementMenu shows the item only in the header toolbar menu.
	PlacementMenu Placement = "menu"
	// PlacementCard shows the item only as a card (or card link) on the home tab.
	PlacementCard Placement = "card"
	// PlacementBoth shows the item in the menu and on the home tab.
	PlacementBoth Placement = "both"
)

// Valid reports whether p is a known placement.
func (p Placement) Valid() bool {
	return p == PlacementMenu || p == PlacementCard || p == PlacementBoth
}

// Item is a persisted menu item.
type Item struct {
	ID          int64     `json:"id"`
	ParentID    *int64    `json:"parent_id"`
	Title       string    `json:"title"`
	URL         *string   `json:"url"`
	Target      Target    `json:"target"`
	Placement   Placement `json:"placement"`
	Icon        *string   `json:"icon"`
	Description *string   `json:"description"`
	Position    int       `json:"position"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// Input holds the writable fields of an item. PUT requests replace all of
// them: omitted optional fields become nil or their default.
type Input struct {
	ParentID    *int64    `json:"parent_id"`
	Title       string    `json:"title"`
	URL         *string   `json:"url"`
	Target      Target    `json:"target"`
	Placement   Placement `json:"placement"`
	Icon        *string   `json:"icon"`
	Description *string   `json:"description"`
	Position    int       `json:"position"`
	Enabled     *bool     `json:"enabled"`
}

// Node is an item with its children, used for tree responses.
type Node struct {
	Item
	Items []*Node `json:"items"`
}

// Move changes the parent and position of one item inside a reorder batch.
type Move struct {
	ID       int64  `json:"id"`
	ParentID *int64 `json:"parent_id"`
	Position int    `json:"position"`
}

// ErrNotFound is returned when an item does not exist.
var ErrNotFound = errors.New("menu item not found")

// ValidationError describes an invalid field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}
