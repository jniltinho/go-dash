## Context

The application serves one menu tree (`menu_items`) at `/`, edited in `/admin`
behind a single admin password (`settings.admin_password_hash`, or env
override) with server-side sessions (`sessions` table, cookie
`dashboard_session`, CSRF token per session). The SPA has routes `/`,
`/admin`, `/admin/login`, `/admin/disabled`; the Go static handler falls back
to `index.html` only for `/` and `/admin*`.

We want several dashboards in one binary (`/` plus `/dash/<slug>`), each with
an optional access password, without regressing the security baseline in
`AGENTS.md` (CSP, CSRF, bcrypt, rate limit, sandboxed iframes).

## Goals / Non-Goals

**Goals:**
- Independent menus per dashboard; existing data becomes the default dashboard.
- Optional per-dashboard password, including for the default dashboard.
- One admin (existing password) manages every dashboard.
- Zero-config upgrade: an existing install behaves exactly as before.

**Non-Goals:**
- Per-dashboard admins / roles or user accounts.
- Sharing one item between dashboards (export/import copies instead).
- Protecting the *targets* of links: the password hides the list of links;
  the linked sites keep their own authentication.
- Per-dashboard theme, `max_tabs` or iframe sandbox settings (global config
  stays global; can be a follow-up).
- Environment-variable passwords for dashboards (DB only for now).

## Decisions

### 1. Data model (`0003_dashboards.sql`)

```sql
CREATE TABLE dashboards (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    slug          TEXT    NULL UNIQUE,          -- NULL only for the default
    title         TEXT    NOT NULL,
    password_hash TEXT    NULL,
    -- bumped on every set/clear; unlock only creates a session if unchanged
    password_version INTEGER NOT NULL DEFAULT 0,
    is_default    INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    created_at / updated_at  (same defaults as menu_items)
);
CREATE UNIQUE INDEX idx_dashboards_default ON dashboards(is_default) WHERE is_default = 1;
INSERT INTO dashboards (id, slug, title, is_default) VALUES (1, NULL, '', 1);
```

- An empty `title` on the default dashboard means "use config `title`", so the
  existing `title` setting keeps working.
- Slugs are stored lower-case; `UNIQUE` on a lower-cased column gives
  case-insensitive uniqueness. Validation lives in `internal/menu`
  (`ValidationError{Field:"slug"}`), the DB constraint is the backstop.
- **`menu_items.dashboard_id`**: SQLite forbids `ALTER TABLE ADD COLUMN ...
  REFERENCES` with a non-NULL default while foreign keys are on, so the
  migration **rebuilds `menu_items`** (SQLite's documented 12-step
  procedure). Final definition:

  ```sql
  CREATE TABLE menu_items_new (
      id           INTEGER PRIMARY KEY AUTOINCREMENT,
      dashboard_id INTEGER NOT NULL DEFAULT 1 REFERENCES dashboards(id) ON DELETE CASCADE,
      parent_id    INTEGER NULL REFERENCES menu_items(id) ON DELETE CASCADE,
      -- title, url, target CHECK, icon, description, position, enabled,
      -- placement CHECK, created_at, updated_at: copied verbatim from 0001/0002
  );
  ```

  The self-reference is written against the *final* name `menu_items`, so it
  resolves after the rename. Steps: copy all rows keeping ids → remember
  `sqlite_sequence.seq` of `menu_items` → drop old → rename new → restore
  `seq` to `max(old seq, max(id))` so ids deleted before the upgrade are never
  reused → recreate `idx_menu_items_parent` and add
  `idx_menu_items_dashboard (dashboard_id, parent_id, position)`.
- **Runner opt-in for `foreign_keys=OFF`.** `PRAGMA foreign_keys` cannot
  change inside a transaction and is connection-local (the DSN sets it only
  when a pooled connection is opened, `internal/database/database.go`). A
  migration file starting with `-- migrate:no-foreign-keys` is therefore run
  with this exact lifecycle on a dedicated `*sql.Conn`:
  1. `PRAGMA foreign_keys=OFF` (outside any transaction), verify it reads 0;
  2. `BEGIN`; execute the file; run `PRAGMA foreign_key_check` and fail if it
     returns any row; insert the `schema_migrations` marker — all in the same
     transaction, as the runner does today;
  3. `COMMIT` (or `ROLLBACK` on any error, leaving no marker so the next start
     retries);
  4. always (deferred, also on error/cancel) `PRAGMA foreign_keys=ON` and
     verify it reads 1; if restoring fails, the connection is discarded
     (`conn.Raw` returning `driver.ErrBadConn`) instead of being returned to
     the pool.
  *Alternative considered:* column without FK and app-enforced cascade —
  rejected because it silently allows orphans.
- Same-dashboard parent rule: validated in `menu.Service.normalize` (parent
  must exist **and** share `dashboard_id`), and in `Reorder`/`Import` inside
  the transaction. A trigger is not added (keep logic in Go, tested).
- **`sessions`**: add `kind TEXT NOT NULL DEFAULT 'admin' CHECK (kind IN
  ('admin','viewer'))` and `dashboard_id INTEGER NULL REFERENCES dashboards(id)
  ON DELETE CASCADE`. Plain `ADD COLUMN` works here because the FK column
  defaults to NULL. Existing rows become admin sessions → admins stay logged in.

### 2. Domain packages

- `internal/menu` gains `Dashboard` model and `DashboardService`
  (List/Get/BySlug/Create/Update/Delete) and every item method takes a
  `dashboardID int64`. Keeping dashboards in `menu` avoids a new package and
  an import cycle with validation/import.
- `internal/auth`:
  - `Lookup` is split into `LookupAdmin(ctx, id)` and `LookupViewer(ctx, id,
    dashboardID)`; each filters in SQL on `kind` (and `dashboard_id` for
    viewers) and returns `ErrNoSession` otherwise. **Every** admin check uses
    `LookupAdmin`: `requireSession` *and* `sessionStatus`
    (`GET /api/admin/session`, which today calls `Lookup` directly and hands
    out the CSRF token). A viewer token copied into `dashboard_session`, or an
    admin/other-dashboard token copied into `dashboard_view_<id>`, is
    therefore rejected. `logout` and `lock` revoke only a session of the
    matching kind/dashboard.
  - `Unlock(ctx, dashboardID, pw) (rawID, Session, error)` is one atomic
    operation: read `(password_hash, password_version)`, run bcrypt outside
    any transaction, then `INSERT INTO sessions (...) SELECT ... FROM
    dashboards WHERE id = ? AND password_version = ? AND password_hash IS NOT
    NULL`; zero rows inserted → `ErrInvalidPassword`. A concurrent password
    change (which bumps the version and deletes viewer sessions in one
    transaction) can never be outlived by a session verified with the old
    password.
  - `SetDashboardPassword` / `ClearDashboardPassword` (transaction: update
    hash, `password_version = password_version + 1`, `DELETE FROM sessions
    WHERE kind='viewer' AND dashboard_id=?`).
  - Dashboard password length: 8–72 **bytes** (bcrypt's input limit),
    validated identically in API and CLI (`ValidationError{Field:"password"}`).
  - `SetPassword` (admin) changes to `DELETE FROM sessions WHERE kind='admin'`
    so viewers are unaffected.
  - Hashes are read from the DB on every check, so a CLI change is seen by a
    running server (same as the admin password today).

### 3. HTTP API

| Method & path | Auth | Notes |
| --- | --- | --- |
| `GET /api/menu` | dashboard access | alias of `/api/dash/default/menu` |
| `GET /api/dash/:slug/status` | none | `{slug,title,protected,unlocked}`; 404 unknown |
| `GET /api/dash/:slug/menu` | dashboard access | 401 `dashboard locked` |
| `POST /api/dash/:slug/unlock` | same-origin + rate limit | sets viewer cookie |
| `POST /api/dash/:slug/lock` | same-origin | revokes viewer session |
| `GET, POST /api/admin/dashboards` | admin + CSRF | |
| `PUT, DELETE /api/admin/dashboards/:dashboardID` | admin + CSRF | `?force=true` on DELETE |
| `PUT, DELETE /api/admin/dashboards/:dashboardID/password` | admin + CSRF | |
| `GET, POST /api/admin/dashboards/:dashboardID/items` | admin + CSRF | |
| `PUT, DELETE /api/admin/dashboards/:dashboardID/items/:itemID` | admin + CSRF | item must belong to the dashboard, else 404 |
| `POST /api/admin/dashboards/:dashboardID/items/reorder` | admin + CSRF | |
| `GET /api/admin/dashboards/:dashboardID/export` | admin + CSRF | |
| `POST /api/admin/dashboards/:dashboardID/import` | admin + CSRF | |
| legacy `GET, POST /api/admin/items`, `PUT, DELETE /api/admin/items/:id`, `POST /api/admin/items/reorder`, `GET /api/admin/export`, `POST /api/admin/import` | admin + CSRF | default dashboard |

- `:dashboardID` is a numeric id or the sentinel `default`; `:itemID` is
  numeric. `pathID` is replaced by `pathInt64(c, name)` and a
  `dashboardParam(c)` resolver, so a dashboard id is never read as an item id.
  Legacy routes inject the default dashboard and call the same handlers.
- `default` is an **API/CLI sentinel** for the default dashboard (reserved, so
  no named dashboard can take it). The frontend uses it for `/`. In the
  browser, `/dash/default` redirects to `/`.
- The unlock and lock routes live outside `/api/admin`, so they get their own
  `middleware.BodyLimit(4 KiB)`; the handler order is same-origin check →
  rate limit → bind `{"password": string}` → `Unlock`. Oversized bodies get
  413 and no session.
- `requireDashboardAccess` middleware resolves the slug, stores the dashboard
  in the context and allows the request when the dashboard is public, when a
  valid admin session is present (`LookupAdmin`), or when cookie
  `dashboard_view_<id>` maps to a session with `kind='viewer'` **and** that
  `dashboard_id` (`LookupViewer`). Otherwise 401 `dashboard locked`.
- Cookie per dashboard (`dashboard_view_<id>`, `Path=/`, `HttpOnly`,
  `SameSite=Strict`, `Secure` like the admin cookie) instead of one cookie
  with many grants: simpler revocation and no shared mutable state.
  Using the numeric id (not the slug) keeps the cookie valid across slug
  renames.
- Unlock uses a **second `RateLimiter`** instance (same `login_rate` per IP,
  `4×` global), keyed by **client IP only** (via the configured
  `IPExtractor`, so `trust_proxy` is honoured). The budget is therefore
  aggregate across dashboards: cycling slugs does not multiply attempts. Being
  a separate instance, unlock spam cannot lock admins out of login and vice
  versa.
- Unlock/lock are mutating but carry no CSRF token (no session yet), so they
  reuse `sameOriginRequest` exactly like `/api/admin/login`. Lock is harmless
  to forge but is still same-origin checked.
- Cache policy:
  - menu of a **public** dashboard: unchanged (`no-cache` + ETag/304);
  - menu of a **protected** dashboard: always a full `200` with
    `Cache-Control: private, no-store`, no `ETag`, `If-None-Match` ignored
    (no 304 ever), and the access check runs first;
  - `status`, `unlock`, `lock`, admin session/login/logout and every 401/404
    from these routes: `Cache-Control: no-store` (they depend on cookies).
- Error mapping: new messages `dashboard locked`, `dashboard not found`
  translated in `frontend/src/lib/messages.ts` ("Dashboard protected by a
  password.", "Dashboard not found.").
- Canonical browser routing (server side, before the SPA fallback):

  | Request | Response |
  | --- | --- |
  | `/dash`, `/dash/`, `/dash/default`, `/dash/default/` | 302 → `/` |
  | `/dash/<seg>/` (trailing slash) | 301 → `/dash/<seg>` |
  | `/dash/<seg>` with upper-case letters | 301 → lower-cased path |
  | `/dash/<seg>` (one segment, slug syntax valid) | SPA `index.html` (GET/HEAD); unknown slug is resolved client side |
  | `/dash/<seg>` with invalid slug syntax, or `/dash/<a>/<b>…` | 404 (no SPA) |
  | `/api/...` unknown | JSON 404, unchanged |

  `isSPARoute` accepts exactly `dash/<valid-slug>`; the Vue router declares
  `/dash/:slug([a-z0-9-]+)`, so its catch-all redirect to `/` can no longer
  mask a bad dashboard URL.

- Item scoping hot spots in `internal/menu` that must take `dashboardID`:
  replace-import's `DELETE FROM menu_items` (must become `WHERE
  dashboard_id = ?`), `maxRootPosition`, `getItem` (ownership: an item of
  another dashboard is `ErrNotFound`), `countChildren`, export, and the seed
  check (seed only when the *default* dashboard is empty). Export documents
  keep `version: 1` and contain no dashboard field, so old files import into
  any dashboard.

### 4. Frontend

- Routes: `/` → `DashboardView` with `slug = 'default'`; `/dash/:slug` →
  same component with the route param. Changing slug re-mounts the view
  (`:key="slug"`) so DHTMLX widgets and iframe panes are rebuilt cleanly.
- `useDashboardStore`: `status` from `/api/dash/:slug/status`; renders
  `UnlockView` (semantic classes `card`, `field`, `btn`) when
  `protected && !unlocked`, "Dashboard not found" on 404, else the
  normal dashboard.
- `useMenuStore` is a Pinia singleton and its background refresh currently
  keeps the old tree on any error, so re-mounting alone does not isolate
  dashboards. The store becomes slug-aware with an explicit
  `reset()` that clears tree, index and ETag, and bumps a **generation
  counter**; every request captures `(slug, generation)` and drops its result
  if either changed (or uses an `AbortController`). `reset()` runs on
  dashboard change, lock, any 401 `dashboard locked`, and 404 `dashboard not
  found`; then the view destroys all tabs, iframe panes and DHTMLX widgets
  before showing the unlock / not-found screen. Only network errors and 5xx
  keep the last tree (current behaviour); 401/404 never do.
- Menu polling and the existing ETag logic take the slug.
- `cardOrder` storage key becomes `dashboard.cardOrder.<slug>`; the existing
  key is migrated once to `default` (read old → write new → remove old, all in
  `try/catch`). Theme key unchanged.
- Admin view: a dashboard selector (DHTMLX Combo/Select, text-only labels)
  at the top; a "Dashboards" panel for create/rename/delete and
  "Set password"/"Remove password". Titles always rendered as text.
- Optional "Log out" (lock) action in the toolbar of a protected dashboard,
  toolbar id `action-lock`.

### 5. CLI

- `dashboard dash list` (`--json`), `add SLUG --title`, `update SLUG
  [--slug NEW] [--title]`, `remove SLUG [--force]`, `set-password SLUG
  [--stdin]`, `clear-password SLUG`. `default` addresses the default dashboard
  where allowed.
- `menu ...` subcommands get a persistent `--dash` flag (default `default`).
  It is a command flag, not a config key (not bound to viper).

## Risks / Trade-offs

- [Table rebuild migration on large DBs / partial failure] → runs in one
  transaction with `foreign_key_check`; `dashboard backup` documented as a
  pre-upgrade step; covered by a migration test that upgrades a 0002 DB
  fixture and compares rows.
- [Viewer session accepted as admin] → `LookupAdmin` filters `kind='admin'`
  in SQL and is used by `requireSession` and `sessionStatus`; handler tests
  swap tokens between cookie names.
- [Unlock racing a password change] → `password_version` + conditional
  insert; deterministic test that changes the password between bcrypt and
  insert (hook via an overridable function in tests).
- [Pooled connection left with `foreign_keys=OFF`] → deferred restore +
  verify, discard connection on failure; runner tests for success, SQL
  error, FK-check error and context cancellation, each asserting
  `PRAGMA foreign_keys` = 1 on a fresh pooled query afterwards.
- [AUTOINCREMENT reset by the rebuild] → `sqlite_sequence` restored; fixture
  with a deleted highest id.
- [Many cookies for a viewer with many dashboards] → one small cookie per
  unlocked dashboard; acceptable for the expected handful of dashboards.
- [Dashboard password is weaker than real auth: shared secret] → documented
  as access gating for link lists, not user authentication; bcrypt + rate
  limit + HTTPS recommendation still apply.
- [Status endpoint reveals titles of protected dashboards] → accepted, needed
  for the unlock screen; slugs are already in the URL.
- [Legacy admin endpoints] → kept as aliases for one release, marked
  deprecated in docs; frontend switches to the new paths immediately.
- [Iframe pane state when switching dashboards] → view is keyed by slug and
  fully re-mounted; tabs are not shared across dashboards.

## Migration Plan

1. Ship migration `0003_dashboards.sql` (+ runner opt-in for
   `foreign_keys=OFF`). On first start it creates the default dashboard and
   moves every item to it.
2. Existing behaviour is unchanged until an admin creates a dashboard or sets
   a password.
3. Rollback: restore the pre-upgrade backup (`dashboard backup`) with the
   previous binary; the new schema is not readable by old binaries.

## Open Questions

- Should the default dashboard optionally redirect to a named dashboard
  (e.g. `/` → `/dash/acme`)? Not in scope; can be a follow-up setting.
- Should the Home page of the default dashboard list the other public
  dashboards? Proposed as a follow-up.
