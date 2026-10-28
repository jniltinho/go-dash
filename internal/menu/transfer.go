package menu

import (
	"context"
	"fmt"
	"sort"
)

const (
	// ExportFormat identifies the import/export document type.
	ExportFormat = "dashboard-menu"
	// ExportVersion is the current import/export document version.
	ExportVersion = 1
)

// ImportMode selects how an import interacts with existing items.
type ImportMode string

const (
	// ImportAppend adds imported root items after the existing ones.
	ImportAppend ImportMode = "append"
	// ImportReplace deletes every existing item of the dashboard before importing.
	ImportReplace ImportMode = "replace"
)

// Document is the versioned import/export payload. It carries no IDs; the
// hierarchy is expressed by nesting.
type Document struct {
	Format  string          `json:"format"`
	Version int             `json:"version"`
	Items   []*DocumentItem `json:"items"`
}

// DocumentItem is one exported item with its children.
type DocumentItem struct {
	Title       string          `json:"title"`
	URL         *string         `json:"url"`
	Target      Target          `json:"target"`
	Placement   Placement       `json:"placement,omitempty"`
	Icon        *string         `json:"icon"`
	Description *string         `json:"description"`
	Enabled     *bool           `json:"enabled"`
	Position    int             `json:"position"`
	Items       []*DocumentItem `json:"items"`
}

// Export returns the full tree of a dashboard, including disabled items.
// The document has no dashboard field, so it imports into any dashboard.
func (s *Service) Export(ctx context.Context, dashboardID int64) (Document, error) {
	tree, err := s.Tree(ctx, dashboardID, false)
	if err != nil {
		return Document{}, err
	}
	return Document{Format: ExportFormat, Version: ExportVersion, Items: toDocumentItems(tree)}, nil
}

func toDocumentItems(nodes []*Node) []*DocumentItem {
	out := make([]*DocumentItem, 0, len(nodes))
	for _, n := range nodes {
		enabled := n.Enabled
		out = append(out, &DocumentItem{
			Title:       n.Title,
			URL:         n.URL,
			Target:      n.Target,
			Placement:   n.Placement,
			Icon:        n.Icon,
			Description: n.Description,
			Enabled:     &enabled,
			Position:    n.Position,
			Items:       toDocumentItems(n.Items),
		})
	}
	return out
}

// Import validates the whole document and writes it into dashboardID in a
// single transaction. It returns the number of imported items.
func (s *Service) Import(ctx context.Context, dashboardID int64, doc Document, mode ImportMode, opts ValidateOptions) (int, error) {
	if doc.Format != ExportFormat {
		return 0, invalid("format", "unsupported format %q", doc.Format)
	}
	if doc.Version != ExportVersion {
		return 0, invalid("version", "unsupported version %d", doc.Version)
	}
	if mode == "" {
		mode = ImportAppend
	}
	if mode != ImportAppend && mode != ImportReplace {
		return 0, invalid("mode", "mode must be %q or %q", ImportAppend, ImportReplace)
	}

	// Validate everything before touching the database.
	if err := s.validateDocument(ctx, dashboardID, doc.Items, opts); err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("starting import: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	offset := 0
	if mode == ImportReplace {
		if _, err := tx.ExecContext(ctx, "DELETE FROM menu_items WHERE dashboard_id = ?", dashboardID); err != nil {
			return 0, fmt.Errorf("clearing menu items: %w", err)
		}
	} else {
		maxPos, err := maxRootPosition(ctx, tx, dashboardID)
		if err != nil {
			return 0, err
		}
		offset = maxPos + 1
	}

	count := 0
	var insert func(items []*DocumentItem, parentID *int64, posOffset int) error
	insert = func(items []*DocumentItem, parentID *int64, posOffset int) error {
		for _, di := range items {
			in := documentInput(di, parentID)
			in.Position += posOffset
			// Parents are inserted in this same transaction, so normalize must read through tx.
			in, err := s.normalize(ctx, tx, dashboardID, in, opts)
			if err != nil {
				return err
			}
			id, err := insertItem(ctx, tx, dashboardID, in)
			if err != nil {
				return err
			}
			count++
			if err := insert(di.Items, &id, 0); err != nil {
				return err
			}
		}
		return nil
	}

	roots := doc.Items
	if mode == ImportAppend {
		// Renumber roots after the existing ones, keeping their relative order,
		// so imported items never land before existing ones.
		roots = make([]*DocumentItem, len(doc.Items))
		copy(roots, doc.Items)
		sort.SliceStable(roots, func(i, j int) bool { return roots[i].Position < roots[j].Position })
		for i, root := range roots {
			clone := *root
			clone.Position = i
			roots[i] = &clone
		}
	}
	if err := insert(roots, nil, offset); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing import: %w", err)
	}
	return count, nil
}

func (s *Service) validateDocument(ctx context.Context, dashboardID int64, items []*DocumentItem, opts ValidateOptions) error {
	for _, di := range items {
		if di == nil {
			return invalid("items", "null item")
		}
		// Parent existence is checked during insertion; validate fields only.
		if _, err := s.normalize(ctx, s.db, dashboardID, documentInput(di, nil), opts); err != nil {
			return err
		}
		if err := s.validateDocument(ctx, dashboardID, di.Items, opts); err != nil {
			return err
		}
	}
	return nil
}

func documentInput(di *DocumentItem, parentID *int64) Input {
	return Input{
		ParentID:    parentID,
		Title:       di.Title,
		URL:         di.URL,
		Target:      di.Target,
		Placement:   di.Placement,
		Icon:        di.Icon,
		Description: di.Description,
		Position:    di.Position,
		Enabled:     di.Enabled,
	}
}
