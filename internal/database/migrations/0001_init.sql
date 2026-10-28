CREATE TABLE menu_items (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id   INTEGER NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    title       TEXT    NOT NULL,
    url         TEXT    NULL,
    target      TEXT    NOT NULL DEFAULT 'iframe' CHECK (target IN ('iframe', 'blank')),
    icon        TEXT    NULL,
    description TEXT    NULL,
    position    INTEGER NOT NULL DEFAULT 0,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_menu_items_parent ON menu_items(parent_id, position);

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE sessions (
    id_hash      TEXT PRIMARY KEY,
    csrf_token   TEXT NOT NULL,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL
);

CREATE INDEX idx_sessions_expires ON sessions(expires_at);
