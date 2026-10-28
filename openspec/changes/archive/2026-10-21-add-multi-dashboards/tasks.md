## 1. Database

- [x] 1.1 Add per-file `-- migrate:no-foreign-keys` opt-in to the migration runner: dedicated `sql.Conn`, `foreign_keys=OFF` verified before `BEGIN`, SQL + `foreign_key_check` + `schema_migrations` marker in one transaction, deferred restore of `foreign_keys=ON` (verified; connection discarded on failure)
- [x] 1.2 Runner tests: success, SQL error, FK-check error and context cancellation each leave no marker, allow a retry, and `PRAGMA foreign_keys` = 1 on subsequent pooled queries
- [x] 1.3 Write `internal/database/migrations/0003_dashboards.sql`: `dashboards` table (with `password_version`) + default row, rebuild `menu_items` per design (self-FK on final name, `dashboard_id` FK, CHECKs copied, `sqlite_sequence` high-water mark preserved, indexes), `sessions.kind` and `sessions.dashboard_id`
- [x] 1.4 Migration tests on a 0002 fixture (with a deleted highest id): rows/ids/hierarchy/order/fields/sessions preserved, next id > deleted id, `foreign_key_list` correct, parent and dashboard cascades work, invalid parent and bad `placement`/`target` rejected; also fresh install and repeated migrate

## 2. Domain (`internal/menu`)

- [x] 2.1 Add `Dashboard` model, slug validation (regex, reserved names, lower-casing) and `DashboardService` (List/Get/BySlug/Create/Update/Delete with force) returning `*ValidationError` / `ErrNotFound`
- [x] 2.2 Scope item repository and service methods (List/Tree/Get/Create/Update/Delete/Reorder, `maxRootPosition`, `countChildren`) by `dashboardID`; items of another dashboard are `ErrNotFound`; reject cross-dashboard parents and moves
- [x] 2.3 Scope Export/Import by dashboard (replace deletes only `WHERE dashboard_id = ?`, append positions per dashboard, export keeps `version: 1`); seed only when the default dashboard is empty
- [x] 2.4 Table-driven tests for slugs (incl. reserved `default`), isolation, cross-dashboard parent/reorder/update/delete, replace import preserving other dashboards, append positions, old export files, delete with/without force

## 3. Auth (`internal/auth`)

- [x] 3.1 Replace `Lookup` with `LookupAdmin` / `LookupViewer(dashboardID)` filtering kind (and dashboard) in SQL; admin `SetPassword` revokes only admin sessions; logout/lock revoke only matching kind
- [x] 3.2 Add atomic `Unlock` (bcrypt, then conditional insert on `password_version`), `SetDashboardPassword`, `ClearDashboardPassword` (transaction: hash, version bump, revoke that dashboard's viewer sessions); 8–72 byte validation shared by API and CLI
- [x] 3.3 Tests: viewer sessions scoped per dashboard, revocation rules, length limits, hashes never exposed, deterministic unlock-vs-password-change race (no session created)

## 4. HTTP server (`internal/server`)

- [x] 4.1 `requireSession` and `sessionStatus` use `LookupAdmin`; tests swap tokens between `dashboard_session` and `dashboard_view_<id>` in both directions
- [x] 4.2 `requireDashboardAccess` middleware (public / admin bypass / `LookupViewer` on `dashboard_view_<id>`); cache policy per design (protected menus: full 200, `private, no-store`, no ETag, never 304; status/unlock/lock and their errors `no-store`)
- [x] 4.3 Public routes: `/api/dash/:slug/status|menu|unlock|lock`, `/api/menu` alias for `default`; 4 KiB body limit on unlock/lock; order same-origin → rate limit → bind → `Unlock`; separate unlock limiter keyed by client IP only (aggregate across dashboards)
- [x] 4.4 Admin routes exactly as in the design table with `:dashboardID` / `:itemID` (replace `pathID` with `pathInt64(c, name)` + `dashboardParam`); legacy item/reorder/export/import routes as default-dashboard aliases
- [x] 4.5 Map new errors (`dashboard locked`, `dashboard not found`, 409 on non-empty delete); canonical `/dash` routing per design (redirects for `/dash`, `/dash/default`, trailing slash, upper-case; SPA only for one valid slug segment; 404 otherwise; GET and HEAD)
- [x] 4.6 `httptest` handler tests for every scenario in `specs/multi-dashboard` and `specs/dashboard-access` (incl. 413 unlock, cycling-dashboards rate limit, unlock limiter independent from login, `If-None-Match: *`, public→protected transition on both menu aliases)

## 5. CLI (`cmd/`)

- [x] 5.1 `dashboard dash list|add|update|remove|set-password|clear-password` (`RunE`, arg validators, `--stdin`, `--force`, `--json`)
- [x] 5.2 Persistent `--dash` flag on `menu` subcommands (default `default`)
- [x] 5.3 Command tests with fresh command trees and temp databases

## 6. Frontend

- [x] 6.1 API client and types: dashboard status/menu/unlock/lock, admin dashboards and password endpoints, scoped item endpoints
- [x] 6.2 Router `/dash/:slug([a-z0-9-]+)`, `DashboardView` keyed by slug; dashboard store with status and unlock; slug-aware `useMenuStore` with `reset()` + generation counter/AbortController dropping stale responses; 401/404 clear the tree (only network/5xx keep it)
- [x] 6.3 `UnlockView` and "Dashboard not found" state (semantic theme classes, accessible focus/labels); on lock/401/404/switch destroy tabs, iframe panes and widgets before showing them; `action-lock` toolbar item
- [x] 6.4 Toolbar/document title per dashboard; Home tab per dashboard
- [x] 6.5 Card order key per dashboard with one-time migration of the old key (pure function + Vitest)
- [x] 6.6 Admin: dashboard selector, dashboards panel (create/rename/delete with confirmation, set/clear password, protected indicator); titles rendered as text only
- [x] 6.7 Translations in `src/lib/messages.ts` for new API errors; Vitest for new pure logic (store reset, stale-response guard, card order key migration)

## 7. E2E and docs

- [x] 7.1 Playwright: named dashboard routing and canonical redirects, unknown slug, unlock success/failure, scoped sessions, admin bypass, password change re-locks with open tabs discarded, default dashboard protection
- [x] 7.2 Update `docs/README.md` (usage, CLI, upgrade/backup note), `docs/architecture.md` (data model, routes, diagrams) and `AGENTS.md` (route/toolbar IDs, layout)
- [x] 7.3 Regenerate screenshots (`make screenshots`) for the unlock screen and admin dashboards panel
- [x] 7.4 Run `make fmt && make lint && make test && make e2e`
