package menu

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// querier is satisfied by *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

const selectColumns = `id, parent_id, title, url, target, placement, icon, description, position, enabled, created_at, updated_at`

func scanItem(scan func(dest ...any) error) (Item, error) {
	var (
		item      Item
		parentID  sql.NullInt64
		url       sql.NullString
		icon      sql.NullString
		desc      sql.NullString
		target    string
		placement string
	)
	err := scan(
		&item.ID,
		&parentID,
		&item.Title,
		&url,
		&target,
		&placement,
		&icon,
		&desc,
		&item.Position,
		&item.Enabled,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Item{}, err
	}

	item.Target = Target(target)
	item.Placement = Placement(placement)
	if parentID.Valid {
		item.ParentID = &parentID.Int64
	}
	item.URL = nullableString(url)
	item.Icon = nullableString(icon)
	item.Description = nullableString(desc)
	return item, nil
}

func nullableString(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func listItems(ctx context.Context, q querier, dashboardID int64) ([]Item, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT "+selectColumns+" FROM menu_items WHERE dashboard_id = ? ORDER BY position, id", dashboardID)
	if err != nil {
		return nil, fmt.Errorf("listing menu items: %w", err)
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		item, err := scanItem(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scanning menu item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating menu items: %w", err)
	}
	return items, nil
}

// getItem returns an item of dashboardID; items of other dashboards are ErrNotFound.
func getItem(ctx context.Context, q querier, dashboardID, id int64) (Item, error) {
	row := q.QueryRowContext(ctx,
		"SELECT "+selectColumns+" FROM menu_items WHERE id = ? AND dashboard_id = ?", id, dashboardID)
	item, err := scanItem(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("getting menu item %d: %w", id, err)
	}
	return item, nil
}

func insertItem(ctx context.Context, q querier, dashboardID int64, in Input) (int64, error) {
	res, err := q.ExecContext(
		ctx,
		`INSERT INTO menu_items (dashboard_id, parent_id, title, url, target, placement, icon, description, position, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dashboardID,
		in.ParentID,
		in.Title,
		in.URL,
		string(in.Target),
		string(in.Placement),
		in.Icon,
		in.Description,
		in.Position,
		*in.Enabled,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting menu item: %w", err)
	}
	return res.LastInsertId()
}

func updateItem(ctx context.Context, q querier, dashboardID, id int64, in Input) error {
	res, err := q.ExecContext(
		ctx,
		`UPDATE menu_items
		 SET parent_id = ?, title = ?, url = ?, target = ?, placement = ?, icon = ?, description = ?,
		     position = ?, enabled = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = ? AND dashboard_id = ?`,
		in.ParentID,
		in.Title,
		in.URL,
		string(in.Target),
		string(in.Placement),
		in.Icon,
		in.Description,
		in.Position,
		*in.Enabled,
		id,
		dashboardID,
	)
	if err != nil {
		return fmt.Errorf("updating menu item %d: %w", id, err)
	}
	return requireAffected(res)
}

func deleteItem(ctx context.Context, q querier, dashboardID, id int64) error {
	res, err := q.ExecContext(ctx, "DELETE FROM menu_items WHERE id = ? AND dashboard_id = ?", id, dashboardID)
	if err != nil {
		return fmt.Errorf("deleting menu item %d: %w", id, err)
	}
	return requireAffected(res)
}

func moveItem(ctx context.Context, q querier, dashboardID int64, m Move) error {
	res, err := q.ExecContext(
		ctx,
		`UPDATE menu_items SET parent_id = ?, position = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = ? AND dashboard_id = ?`,
		m.ParentID,
		m.Position,
		m.ID,
		dashboardID,
	)
	if err != nil {
		return fmt.Errorf("moving menu item %d: %w", m.ID, err)
	}
	return requireAffected(res)
}

func countChildren(ctx context.Context, q querier, dashboardID, id int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM menu_items WHERE parent_id = ? AND dashboard_id = ?", id, dashboardID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting children of %d: %w", id, err)
	}
	return n, nil
}

func maxRootPosition(ctx context.Context, q querier, dashboardID int64) (int, error) {
	var pos sql.NullInt64
	err := q.QueryRowContext(ctx,
		"SELECT MAX(position) FROM menu_items WHERE parent_id IS NULL AND dashboard_id = ?", dashboardID).Scan(&pos)
	if err != nil {
		return 0, fmt.Errorf("reading max root position: %w", err)
	}
	if !pos.Valid {
		return -1, nil
	}
	return int(pos.Int64), nil
}

func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("reading affected rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
