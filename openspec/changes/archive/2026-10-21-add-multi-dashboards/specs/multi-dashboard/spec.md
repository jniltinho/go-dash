## ADDED Requirements

### Requirement: Default dashboard
The system SHALL always have exactly one default dashboard. It SHALL have no
slug, SHALL be served at `/`, SHALL NOT be deletable and its slug SHALL NOT be
settable. Its title SHALL default to the configured `title`.

#### Scenario: Fresh install
- **WHEN** the server starts on an empty database
- **THEN** the default dashboard exists and `GET /` shows its menu

#### Scenario: Delete default dashboard
- **WHEN** an admin tries to delete the default dashboard (API or CLI)
- **THEN** the request is rejected with HTTP 400 `{"error","field":"id"}` (CLI: non-zero exit) and nothing changes

### Requirement: Named dashboards
The system SHALL support any number of named dashboards, each with a unique
slug and a title, served at `/dash/<slug>`. A slug SHALL match
`^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$` (1–40 characters, lower-case
letters, digits and inner hyphens) and SHALL NOT be one of the reserved names
`admin`, `api`, `assets`, `dash`, `default`, `healthz`, `login`. Slugs are
compared after lower-casing input; uniqueness is case-insensitive.

#### Scenario: Create named dashboard
- **WHEN** an admin creates a dashboard with slug `acme` and title `Acme`
- **THEN** it is stored and `/dash/acme` shows its (empty) menu

#### Scenario: Reject invalid slug
- **WHEN** an admin creates a dashboard with slug `Over Mind`, `-x`, or `admin`
- **THEN** the request is rejected with HTTP 400 and `field` = `slug`

#### Scenario: Reject duplicate slug
- **WHEN** a dashboard `initech` exists and another one is created with slug `Initech`
- **THEN** the request is rejected with HTTP 400 and `field` = `slug`

#### Scenario: Unknown slug
- **WHEN** a browser opens `/dash/does-not-exist`
- **THEN** the SPA is served and shows "Dashboard not found" with a link to Home, and `GET /api/dash/does-not-exist/status` returns 404

### Requirement: Per-dashboard menus
Every menu item SHALL belong to exactly one dashboard. Parent and child items
SHALL belong to the same dashboard; reorder moves and imports SHALL NOT move
items across dashboards. Public and admin trees SHALL only contain the items
of the requested dashboard.

#### Scenario: Isolated trees
- **WHEN** `acme` has item A and `globex` has item B
- **THEN** `GET /api/dash/acme/menu` returns A only and `GET /api/dash/globex/menu` returns B only

#### Scenario: Cross-dashboard parent
- **WHEN** an admin creates an item in `acme` whose `parent_id` belongs to `initech`
- **THEN** the request is rejected with HTTP 400 and `field` = `parent_id`

#### Scenario: Cross-dashboard reorder
- **WHEN** a reorder request for `acme` contains an item id of another dashboard
- **THEN** the whole reorder is rejected and no item moves

#### Scenario: Item of another dashboard
- **WHEN** an admin updates or deletes item 7 via `/api/admin/dashboards/{acme id}/items/7` and item 7 belongs to `initech`
- **THEN** the request returns 404 and item 7 is unchanged

#### Scenario: Replace import is scoped
- **WHEN** an admin imports a file in replace mode into `acme`
- **THEN** only `acme` items are replaced and every other dashboard keeps all its items

#### Scenario: Append positions per dashboard
- **WHEN** an admin appends items to `globex`
- **THEN** new root positions follow the last root position of `globex`, not of other dashboards

#### Scenario: Export is scoped and compatible
- **WHEN** an admin exports `initech`
- **THEN** the document contains only `initech` items, keeps `version: 1`, and can be imported into any dashboard; files exported before this change still import

#### Scenario: Seed checks the default dashboard
- **WHEN** named dashboards have items but the default dashboard is empty on a first start with a seed file
- **THEN** the seed is imported into the default dashboard only

### Requirement: Dashboard deletion
Deleting a named dashboard SHALL delete its items and its viewer sessions in
one transaction. When the dashboard still has items the request SHALL require
explicit confirmation (`force=true` in the API, `--force` in the CLI).

#### Scenario: Delete non-empty dashboard without force
- **WHEN** an admin deletes `initech`, which has items, without force
- **THEN** the request is rejected with HTTP 409 and nothing is deleted

#### Scenario: Delete with force
- **WHEN** an admin deletes `initech` with force
- **THEN** the dashboard, its items and its viewer sessions are removed and `/dash/initech` shows "Dashboard not found"

### Requirement: Dashboard routing
The server SHALL serve the SPA for `/` and for `/dash/<slug>` where `<slug>`
is a single segment with valid slug syntax. `/dash`, `/dash/` and
`/dash/default` SHALL redirect to `/`; a trailing slash or upper-case letters
SHALL redirect (301) to the canonical lower-case path without trailing slash;
an invalid slug or extra path segments SHALL return 404 without the SPA. The
default dashboard menu SHALL be available at `GET /api/menu` (unchanged path)
and at `GET /api/dash/default/menu`; named dashboards at
`GET /api/dash/:slug/menu`. `default` SHALL be accepted only as an API/CLI
sentinel, never as a stored slug. Menus of public dashboards SHALL keep the
existing ETag/304 behaviour (protected menus: see `dashboard-access`).

#### Scenario: Canonical URLs
- **WHEN** a browser requests `/dash/Acme/`
- **THEN** it is redirected to `/dash/acme`

#### Scenario: Default sentinel in the browser
- **WHEN** a browser requests `/dash/default`
- **THEN** it is redirected to `/`

#### Scenario: Extra segments
- **WHEN** a browser requests `/dash/acme/extra`
- **THEN** the server returns 404 and does not render the default dashboard

#### Scenario: Direct named dashboard URL
- **WHEN** a browser loads `/dash/globex` directly
- **THEN** the server returns `index.html` and the SPA renders the `globex` dashboard

#### Scenario: Legacy default menu endpoint
- **WHEN** a client calls `GET /api/menu`
- **THEN** it receives the default dashboard's enabled tree, as before this change

### Requirement: Dashboard UI context
The dashboard view SHALL display the current dashboard's title in the toolbar
and document title. The Home tab and button SHALL return to the current
dashboard's home. Per-viewer `localStorage` conveniences tied to a menu (home
card order) SHALL be namespaced per dashboard; theme preferences SHALL stay
global. The Admin button SHALL keep opening `/admin`.

#### Scenario: Card order per dashboard
- **WHEN** a viewer reorders cards on `/dash/acme`
- **THEN** the card order of `/` and of `/dash/initech` is unchanged

### Requirement: Dashboard administration
The Admin area SHALL list all dashboards (default first, then by title), allow
creating, renaming (title and slug of named dashboards) and deleting them, and
SHALL scope item management, reorder and import/export to a selected
dashboard. The admin API SHALL expose, behind session and CSRF checks:
`GET, POST /api/admin/dashboards`; `PUT, DELETE /api/admin/dashboards/:dashboardID`;
`GET, POST /api/admin/dashboards/:dashboardID/items`;
`PUT, DELETE /api/admin/dashboards/:dashboardID/items/:itemID`;
`POST /api/admin/dashboards/:dashboardID/items/reorder`;
`GET /api/admin/dashboards/:dashboardID/export`;
`POST /api/admin/dashboards/:dashboardID/import`. `:dashboardID` SHALL
accept a numeric id or `default`. The legacy routes `GET, POST /api/admin/items`,
`PUT, DELETE /api/admin/items/:id`, `POST /api/admin/items/reorder`,
`GET /api/admin/export` and `POST /api/admin/import` SHALL keep working on
the default dashboard.

#### Scenario: Manage items of a named dashboard
- **WHEN** an admin selects `initech` in the Admin area and creates a link
- **THEN** the link appears on `/dash/initech` and not on `/`

#### Scenario: Rename slug
- **WHEN** an admin changes the slug of `initech` to `initech-ops`
- **THEN** `/dash/initech-ops` shows the dashboard and `/dash/initech` shows "Dashboard not found"

### Requirement: Dashboard CLI
The CLI SHALL provide `dashboard dash list|add|update|remove|set-password|clear-password`.
`menu list|add|update|remove|export|import` SHALL accept `--dash SLUG`,
defaulting to the default dashboard. The initial menu seed SHALL be imported
into the default dashboard.

#### Scenario: Add dashboard from CLI
- **WHEN** an operator runs `dashboard dash add acme --title "Acme"`
- **THEN** the dashboard is created and listed by `dashboard dash list`

#### Scenario: Import into a named dashboard
- **WHEN** an operator runs `dashboard menu import menu.json --dash globex`
- **THEN** the items are added to `globex` only

### Requirement: Migration of existing data
Upgrading an existing database SHALL create the default dashboard and assign
every existing menu item to it, preserving ids, hierarchy, order and all
fields. Existing admin sessions SHALL remain valid. Item ids SHALL never be
reused, even ids deleted before the upgrade. Foreign keys (parent cascade,
dashboard cascade) and CHECK constraints SHALL work after the upgrade, and a
failed upgrade SHALL leave the database unchanged, unmarked and retryable,
with foreign key enforcement still on for every connection.

#### Scenario: Upgrade
- **WHEN** a database created before this change is opened by the new binary
- **THEN** `/` shows exactly the same menu as before and the admin stays logged in

#### Scenario: Ids are not reused
- **WHEN** the highest item id was deleted before the upgrade and a new item is created afterwards
- **THEN** the new id is greater than the deleted one

#### Scenario: Constraints after upgrade
- **WHEN** a parent item is deleted after the upgrade, or an item is inserted with an unknown `parent_id`
- **THEN** children are deleted in cascade, and the insert is rejected

#### Scenario: Failed upgrade
- **WHEN** migration 0003 fails midway
- **THEN** the schema and data stay as in 0002, the migration is not marked as applied, the next start retries it, and `PRAGMA foreign_keys` is 1 on all pooled connections
