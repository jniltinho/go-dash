-- migrate:no-foreign-keys
-- Several dashboards per instance: the default one (no slug, served at "/")
-- and named ones served at "/dash/<slug>", each with an optional password.
CREATE TABLE dashboards (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    slug             TEXT    NULL UNIQUE,
    title            TEXT    NOT NULL DEFAULT '',
    password_hash    TEXT    NULL,
    -- Bumped on every password change; unlock only stores a session when unchanged.
    password_version INTEGER NOT NULL DEFAULT 0,
    is_default       INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    created_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK ((is_default = 1) = (slug IS NULL))
);

CREATE UNIQUE INDEX idx_dashboards_default ON dashboards(is_default) WHERE is_default = 1;

-- An empty title on the default dashboard means "use the configured title".
INSERT INTO dashboards (id, slug, title, is_default) VALUES (1, NULL, '', 1);

-- Rebuild menu_items to add dashboard_id with a foreign key (SQLite cannot
-- add a REFERENCES column with a non-NULL default). The self-reference names
-- the final table, so it resolves after the rename.
CREATE TABLE menu_items_new (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    dashboard_id INTEGER NOT NULL DEFAULT 1 REFERENCES dashboards(id) ON DELETE CASCADE,
    parent_id    INTEGER NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    title        TEXT    NOT NULL,
    url          TEXT    NULL,
    target       TEXT    NOT NULL DEFAULT 'iframe' CHECK (target IN ('iframe', 'blank')),
    icon         TEXT    NULL,
    description  TEXT    NULL,
    position     INTEGER NOT NULL DEFAULT 0,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    placement    TEXT    NOT NULL DEFAULT 'both' CHECK (placement IN ('menu', 'card', 'both'))
);

INSERT INTO menu_items_new (id, dashboard_id, parent_id, title, url, target, icon, description,
                            position, enabled, created_at, updated_at, placement)
SELECT id, 1, parent_id, title, url, target, icon, description,
       position, enabled, created_at, updated_at, placement
FROM menu_items;

-- Keep the AUTOINCREMENT high-water mark, so ids deleted before the upgrade
-- are never reused.
DELETE FROM sqlite_sequence WHERE name = 'menu_items_new';
INSERT INTO sqlite_sequence (name, seq)
SELECT 'menu_items_new', MAX(seq)
FROM (
    SELECT seq FROM sqlite_sequence WHERE name = 'menu_items'
    UNION ALL
    SELECT MAX(id) FROM menu_items_new
)
HAVING MAX(seq) IS NOT NULL;

DROP TABLE menu_items;
ALTER TABLE menu_items_new RENAME TO menu_items;

CREATE INDEX idx_menu_items_parent ON menu_items(parent_id, position);
CREATE INDEX idx_menu_items_dashboard ON menu_items(dashboard_id, parent_id, position);

-- Sessions: existing rows are admin sessions; viewer sessions unlock one dashboard.
ALTER TABLE sessions ADD COLUMN kind TEXT NOT NULL DEFAULT 'admin' CHECK (kind IN ('admin', 'viewer'));
ALTER TABLE sessions ADD COLUMN dashboard_id INTEGER NULL REFERENCES dashboards(id) ON DELETE CASCADE;

CREATE INDEX idx_sessions_dashboard ON sessions(kind, dashboard_id);
