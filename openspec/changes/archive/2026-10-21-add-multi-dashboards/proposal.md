## Why

Today the binary serves exactly one menu at `/`. Different teams (Acme,
Globex, Initech…) need their own set of links without running one binary
per team, and some of those sets must not be visible to everyone on the
network. We need several independent dashboards in one instance, each one
optionally protected by its own password.

## What Changes

- New **dashboard** entity: the **default dashboard** (always exists, served at
  `/`, no slug) plus any number of **named dashboards** served at
  `/dash/<slug>` (e.g. `/dash/acme`, `/dash/globex`, `/dash/initech`).
- Every menu item belongs to exactly one dashboard. Existing items are migrated
  to the default dashboard, so current installations keep working unchanged.
- Each dashboard (including the default one) has an **optional access
  password** (bcrypt), independent from the admin password and from the other
  dashboards. Without a password the dashboard stays public, as today.
- Protected dashboards show an unlock screen. A successful unlock
  creates a server-side **viewer session** scoped to that dashboard only;
  changing or removing the password revokes those sessions. Unlock attempts
  use the login rate limit.
- A logged-in admin can open every dashboard without its viewer password.
- New public API scoped by dashboard: `GET /api/dash/:slug/menu`,
  `GET /api/dash/:slug/status`, `POST /api/dash/:slug/unlock`,
  `POST /api/dash/:slug/lock`. The existing `GET /api/menu` remains as the
  default dashboard's menu (now subject to its password).
- Admin API and UI: list/create/rename/delete dashboards, set/clear each
  password, and manage items, reorder and import/export **per dashboard**
  (dashboard selector in the Admin area).
- CLI: new `dashboard dash` command group (`list`, `add`, `update`, `remove`,
  `set-password`, `clear-password`); `menu` subcommands gain `--dash SLUG`
  (default: the default dashboard).
- **BREAKING (admin API only)**: admin item endpoints move under
  `/api/admin/dashboards/:dash/...`; the old `/api/admin/items` paths keep
  working as aliases for the default dashboard during this release.

## Capabilities

### New Capabilities
- `multi-dashboard`: dashboard entity, slugs and reserved names, routing
  (`/` and `/dash/<slug>`), per-dashboard menus, admin/CLI management,
  migration of existing data.
- `dashboard-access`: optional per-dashboard password, viewer sessions,
  unlock/lock flow, rate limiting, admin bypass and session revocation.

### Modified Capabilities
<!-- The base specs from add-link-dashboard are not archived into
     openspec/specs/ yet, so the changed behaviour is expressed as ADDED
     requirements in the two new capabilities above. -->

## Impact

- **Database**: migration `0003_dashboards.sql` (new `dashboards` table,
  `menu_items.dashboard_id`, viewer columns on `sessions`).
- **Go**: `internal/menu` (dashboard scoping, new dashboard service),
  `internal/auth` (dashboard password + viewer sessions), `internal/server`
  (routes, middleware, SPA fallback for `/dash/*`), `cmd/` (`dash` group,
  `--dash` flag), seed/import target the default dashboard.
- **Frontend**: router (`/dash/:slug`), API client, stores (current dashboard),
  new `UnlockView`, dashboard not found state, Admin dashboard selector and
  dashboard management, per-dashboard `localStorage` keys (card order), new
  messages in `messages.ts`.
- **Docs**: `docs/README.md`, `docs/architecture.md`, `dashboard.toml.example`
  unchanged unless new config keys appear (none planned).
- **Tests**: Go unit/handler tests, Vitest for pure logic, Playwright e2e for
  routing and unlock flows.
