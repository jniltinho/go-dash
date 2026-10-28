## 0. Decision gate

- [x] 0.1 Confirm the DHTMLX Suite edition for distribution (implemented with `@dhtmlx/suite` 9.3.12, **GPL v2** edition; proprietary distribution requires a commercial license — trial not accepted) — closed by the owner on 2026-10-21, keeping the GPL v2 edition

## 1. Project setup

- [x] 1.1 `go mod init` and the `main.go`, `cmd/`, `internal/{config,database,menu,auth,server}`, `web/` structure
- [x] 1.2 Go dependencies: cobra, viper, echo/v5, modernc.org/sqlite, golang.org/x/crypto/bcrypt, golang.org/x/term (logs with `log/slog`)
- [x] 1.3 `frontend/` with Vite+ and dependencies: `vue`, `vue-router`, `pinia`, `@dhtmlx/suite`, `@mdi/font`, `tailwindcss` + `@tailwindcss/vite`, `@vitejs/plugin-vue`, `vue-tsc`, Playwright
- [x] 1.4 `frontend/vite.config.ts` with `defineConfig` from `vite-plus`: vue plugin, `build.outDir: '../web/dist'`, dev server `:9000` proxying `/api` and `/healthz` → `:8080`, `test`/`lint`/`fmt` sections
- [x] 1.5 Versioned `web/dist/.gitkeep`, `//go:embed all:web/dist` in `main.go` and a notice page when the frontend build does not exist
- [x] 1.6 `Makefile` (`frontend` with `vp check && vue-tsc && vp build`, `build` with `CGO_ENABLED=0` + ldflags, `test`, `e2e`, `dev`, `dev-frontend`, `docker-*`) and `.gitignore`
- [x] 1.7 `frontend/go.mod` isolating `node_modules` from `go test ./...`

## 2. CLI and configuration

- [x] 2.1 `cmd/root.go` with Cobra + viper (flags > env `DASHBOARD_*` > TOML > defaults) and slog logger (text/json)
- [x] 2.2 `version` with version/commit/date via ldflags
- [x] 2.3 Configuration precedence tests
- [x] 2.4 `dashboard.toml.example` documenting every key

## 3. Database

- [x] 3.1 `internal/database`: SQLite with DSN pragmas (`foreign_keys(1)`, `busy_timeout(5000)`, `journal_mode(WAL)`) applied per connection
- [x] 3.2 Embedded migration runner with a `schema_migrations` table
- [x] 3.3 Migration `0001_init.sql`: `menu_items` + index, `settings` and `sessions`
- [x] 3.4 `migrate` and idempotency tests
- [x] 3.5 Test of `PRAGMA foreign_keys = 1` on several simultaneous pool connections
- [x] 3.6 `backup` with `VACUUM INTO`; backup test with data in the WAL and `integrity_check`

## 4. Menu domain

- [x] 4.1 `internal/menu`: model and SQL repository (CRUD, ordered listing)
- [x] 4.2 Service with validations (title, http/https URL, target, existing parent, same-origin URL forbidden in `iframe`)
- [x] 4.3 In-memory tree building (public filtering disabled branches; complete for admin)
- [x] 4.4 Cycle detection and batch reordering validating the final tree, transactional
- [x] 4.5 Cascading removal (API) and blocking without `--force` in the CLI
- [x] 4.6 Export/import in the `dashboard-menu` v1 format (no IDs; `append`/`replace`; full validation before writing; single transaction)
- [x] 4.7 Unit tests covering the `menu-management` scenarios

## 5. Menu and admin CLI commands

- [x] 5.1 `menu list` (`--tree`, `--json`), `menu add`, `menu update` (only the given flags), `menu remove` (`--force`)
- [x] 5.2 `menu export` (`-o`) and `menu import` (`--replace`)
- [x] 5.3 `admin set-password` (no echo or `--stdin`, bcrypt cost 12, revokes sessions, warning when env prevails)
- [x] 5.4 Command tests with a temporary database
- [x] 5.5 `serve --seed` importing the initial menu only with an empty database

## 6. HTTP server and security

- [x] 6.1 `serve` + `internal/server` with Echo v5 (recover, request-id, slog request logger, gzip, 1 MiB limit on `/api/admin/*`)
- [x] 6.2 Embedded `web/dist` with ETag, `immutable` on `assets/`, `no-cache` on `index.html`; SPA fallback on `/` and `/admin*`; JSON 404 on `/api/*`; `--dev` reading from disk
- [x] 6.3 Headers: complete CSP with configurable `frame-src`, XFO, nosniff, Referrer-Policy, Permissions-Policy, HSTS under HTTPS
- [x] 6.4 `GET /api/menu`, `GET /api/config` and `GET /healthz`
- [x] 6.5 `internal/auth`: password precedence (database hash re-read on every login), server-side sessions (32-byte id, SHA-256, 2 h idle / 12 h absolute, rotation on login, periodic cleanup), `Secure` cookie under HTTPS/`--trust-proxy`/`--cookie-secure`
- [x] 6.6 Login/logout/session with `Origin` check, rate limit 5/min per IP (429) and disabled mode without a password
- [x] 6.7 CSRF synchronizer token per session on `POST/PUT/DELETE`, constant-time comparison
- [x] 6.8 Routes `/api/admin/items` (GET/POST/PUT full replacement/DELETE), `/reorder`, `/export`, `/import?mode=`
- [x] 6.9 TLS (`--tls-cert`/`--tls-key`) and a log warning without HTTPS
- [x] 6.10 Graceful shutdown (10 s via `echo.StartConfig`, warning when exceeded)
- [x] 6.11 Handler tests with `httptest` (401, 403 missing CSRF/from another session/Origin, 400, 404, 413, 429, logout, SPA, ETag/304)

## 7. Public frontend (Vue 3 + DHTMLX)

- [x] 7.0 SPA base: `main.ts`, `App.vue`, `router.ts` (history mode, admin guard), Pinia stores (`config`, `menu`, `auth`, `theme`), `api/` client with `X-CSRF-Token`
- [x] 7.0a `useDhx` composable (creation in `onMounted`, `markRaw`, `destructor()` in reverse order on unmount)
- [x] 7.1 `DashboardView.vue` with a full-screen `dhx.Layout` (`header`, `tabs`, `content` cells)
- [x] 7.2 Pure function tree → `dhx.Toolbar` items (nested navItem/menuItem, title, spacer, Theme, Home, Admin) with tests
- [x] 7.3 `click` dispatch by ID prefix (`item-`, `open-`, `action-`, `theme-`, `style-`)
- [x] 7.3a `TabTracker` (open/reuse/close/LRU, tested) + `dhx.Tabbar` with `noContent` and Vue panes via `<Teleport>`/`v-show` (iframe never re-created)
- [x] 7.3b `closable` as a list of open IDs (without `home`) + `beforeClose` guard; `max_tabs` from `/api/config`
- [x] 7.3c DHTMLX APIs checked against the 9.3.12 types and exercised in e2e
- [x] 7.4 `blank` action: `window.open` with `noopener,noreferrer`
- [x] 7.5 Context bar with "Open in new tab" and "Reload"; "Open <title>" entry for groups with a URL
- [x] 7.6 Active tab ↔ `location.hash`; deep links (valid, nonexistent/disabled/group, `blank`) and back/forward
- [x] 7.7 Home page ("Home" tab) with cards and an error state with "Try again"
- [x] 7.8 Local DHTMLX icon font (`src/vendor/dhx-icons`) + Vite plugin removing the CDN `@import`
- [x] 7.9 User-facing messages for API errors (`src/lib/messages.ts`)

## 8. Themes

- [x] 8.1 `theme.css` with the blue and black theme over the DHTMLX `dark` theme variables
- [x] 8.2 Global `border-radius: 0` (`--dhx-border-radius: 0` + universal rule)
- [x] 8.3 Toolbar/dropdowns/Tabbar/login/admin with the theme applied; semantic classes (`btn`, `card`, `context-bar`, `link-item`, `field`, `panel-title`)
- [x] 8.4 Automated contrast test ≥ 4.5:1 for every approved pair
- [x] 8.5 Windows XP (Luna) theme over the DHTMLX `light` theme
- [x] 8.6 Flat and No Flat styles for both themes
- [x] 8.9 Tab close button with per-theme colors (visible on the active and inactive tab), 22 px area and space reserved in the title
- [x] 8.10 Theme registry (`src/theme/registry`): tokens as data, `applyTheme` generates CSS variables, `theme.css` without per-theme rules
- [x] 8.11 Per-theme contrast test (text ≥ 7:1 AAA, icons ≥ 4.5:1, UI ≥ 3:1, including 3D gradient stops)
- [x] 8.12 Dark, Graphite, Midnight, Light, Ocean Light/Dark (local Inter), Windows 98, Classic Mac OS, Matrix and High Contrast themes; grouped menu
- [x] 8.13 Slim bars (toolbar 40 px, tabs 32 px) and scaling for QHD/4K (`--ui-scale` + `lib/scale.ts`), e2e tests at 1920/2560/3840
- [x] 8.14 "Choose…" button aligned with the icon field at every resolution
- [x] 8.8 Thin scrollbars with theme colors (standard + WebKit fallback), checked in e2e
- [x] 8.7 "Theme" menu in the dashboard and administration toolbar; preference in `localStorage` applied before the first paint

## 9. Administration frontend

- [x] 9.1 `LoginView.vue` and `AdminDisabledView.vue`
- [x] 9.2 `AdminView.vue` with `dhx.Layout` (header + resizable columns), `dhx.Tree` + `dhx.Form` + `dhx.Toolbar`
- [x] 9.3 Drag-and-drop in the tree calling `/reorder` with the complete hierarchy
- [x] 9.4 Validation errors shown on the form field (translated)
- [x] 9.5 `dhx.Window` to import (`append`/`replace`) and export by download
- [x] 9.7 `placement` field (menu/card/both): migration 0002, validation, import/export, CLI `--placement`, "Show in" in the administration and a suffix in the tree
- [x] 9.8 Toolbar and Home filter by placement (recursive); "Arrange cards" for admins with drag and drop and arrows, order saved via `/reorder` preserving menu-only items
- [x] 9.9 Card header icon aligned with the link icon column (e2e with 1 px tolerance)
- [x] 9.10 `default_theme`/`default_theme_style` (default Ocean Light, flat) in config/ENV/flags and `/api/config`; applied before the first render when the analyst has no saved choice
- [x] 9.11 "Home" and "Admin" labels for the home tab and the administration area
- [x] 9.12 Google, Microsoft and Super Mario World inspired themes ("Inspired" group), AAA verified, screenshots in the README
- [x] 9.13 Brasil theme (flag colors), AAA verified, screenshots in the README
- [x] 9.31 `make dist-server`: ready server package (127.0.0.1:8585 behind nginx, bcrypt hash of the admin password, exported menu, `Makefile` with install/status/logs/backup/password/uninstall)
- [x] 9.30 `go-dash` systemd service in `/opt/go-dash` (dedicated user, hardening), `deploy/install.sh` (installs/upgrades with backup), `make dist-bin` package with everything, `DASHBOARD_BIND_IP` in Docker and the `docs/install.md` guide (systemd, Docker + Compose installation, HTTPS proxy, backup, upgrade)
- [x] 9.29 Custom icon in the Acme style (navy panel with a turquoise menu bar and light cards, without rounded corners) as favicon and brand mark in the bar
- [x] 9.28 Cards moved directly by the header (Grafana style), without an "Arrange cards" button; discreet arrows in the header (hover/focus) as an alternative to dragging
- [x] 9.27 Home card order per browser (`localStorage`), for any analyst and without login, without reset; replaces the server-saved order of 9.8 (the Admin defines the default order)
- [x] 9.26 Open dashboards follow admin changes (card order, items) without reloading: check on tab focus and every 30 s; `/api/menu` with ETag/304
- [x] 9.25 Super Mario World theme replaced by Super Mario Galaxy (`super-mario-world` alias kept)
- [x] 9.24 Admin: "Duplicate" item and "Copy" URL button; `deploy/docker-compose.yml` (prebuilt image) copied to `dist/` with `.env.example`
- [x] 9.23 Menus with a smooth transition (entry, hover, arrow), disabled with `prefers-reduced-motion`; `make dist` produces the binary, a tar.gz of the binary and a tar.gz of the Docker image in `dist/` (outside git)
- [x] 9.22 Ubuntu and macOS themes (Systems group); active option of the "Theme" menu in bold with an accent bar, without a check icon
- [x] 9.21 Windows 10, Linux KDE and xterm themes; Retro group renamed to Systems (Winamp moved to Inspired); lean README and complete guide in docs/README.md
- [x] 9.20 Windows 7 theme (Retro group); styles renamed to "Flat" and "No Flat" (IDs `flat`/`3d` kept)
- [x] 9.19 Windows Vista theme (Retro group), AAA verified, screenshots in the README
- [x] 9.18 Winamp theme (Retro group), AAA verified, screenshots in the README
- [x] 9.17 Light shadows on dark themes (`--shadow-card` in 3D and, very subtle, in flat; `--shadow-menu` on dropdowns and in the icon picker; optional `shadowRgb`, green in Matrix)
- [x] 9.16 "Ocean Light" and "Ocean Dark" labels (IDs `ocean-light`/`ocean-dark` kept for config and saved preferences)
- [x] 9.15 Theme renamed to "Bio Manager" (`bio-manager`), with an alias for the old name
- [x] 9.14 ad-manager theme (Criare pattern) and optional `cardHeaderBorder` token, AAA verified, screenshots in the README
- [x] 9.6 Visual icon picker (`IconPicker.vue`): suggested icons, search by name and synonyms, 240 limit, preview in the field; names generated from `@mdi/font` via a virtual module loaded on demand

## 10. Browser tests and delivery

- [x] 10.1 Dashboard e2e: 3-level menus, group with URL, fixed actions, iframe/blank, tab reuse without reloading, preserved state, close, LRU limit, deep links, back/forward
- [x] 10.1a Administration e2e: disabled, wrong password, CRUD, per-field error, drag-and-drop, import/export, deletion with confirmation, logout
- [x] 10.2 Theme e2e: `border-radius` `0px` in every combination, toolbar colors, persistence after reload
- [x] 10.3 Security/offline e2e: HTML title as text, same origin refused, `top.location` blocked, no CSP violation, no external request
- [x] 10.4 Binary in an empty directory creates the database, registers items via CLI and serves the interface
- [x] 10.5 Multi-stage Dockerfile (Node + Vite+ → Go → non-root Alpine, HEALTHCHECK) and `docker-compose.yml`
- [x] 10.6 README, `AGENTS.md`/`CLAUDE.md` and `docs/architecture.md` with diagrams
- [x] 10.7 `examples/menu.json` in the `dashboard-menu` v1 format
