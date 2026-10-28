## Context

New project (empty directory). The goal is a link aggregator dashboard delivered as **a single Go binary**, with a local SQLite database and an embedded frontend. It follows the conventions already used in other Go projects of the team (go-postfixadmin, pdfconverter-go): `main.go` at the root, Cobra commands in `cmd/`, packages in `internal/`, Echo v5 as the HTTP framework, `log/slog` for logs (Echo v5 adopted slog) and viper for configuration in **TOML** + environment variables.

The frontend is a **Vue 3 + TypeScript** SPA built with **Vite+** (https://viteplus.dev — `vite-plus`, CLI `vp`, which unifies Vite/Rolldown, Vitest, Oxlint and Oxfmt), following the same `frontend/` → `web/dist` organization as go-postfixadmin. It uses DHTMLX Suite (https://dhtmlx.com/docs/download/), in particular the `Layout`, `Toolbar`, `Tabbar`, `Tree`, `Form`, `Window` and `Message` components.

## Goals / Non-Goals

**Goals:**
- Single, static binary (`CGO_ENABLED=0`), cross-platform, with no network dependency at runtime (no CDN).
- Header with menus and submenus of unlimited depth, loaded from the database.
- Open each link in an iframe in the central area or in a new tab, according to the item's configuration.
- Administration via the web (protected) and via the CLI.
- First run without configuration: creates the database, applies migrations and starts on the default port.

**Non-Goals:**
- Multi-user with per-item permissions/RBAC (only one administrator profile in this version).
- SSO/OAuth/LDAP.
- Reverse proxy of the sites opened in the iframe (we will not work around the targets' `X-Frame-Options`/CSP).
- Monitoring/health check of the target links.
- High availability or an external database (Postgres/MySQL).

## Decisions

### 1. Project structure
```
main.go                    # asset embed + cmd.Execute()
cmd/                       # root.go, serve.go, migrate.go, menu.go, version.go
internal/config/           # viper: flags > env (DASHBOARD_*) > file > defaults
internal/database/         # SQLite opening, pragmas, embedded migrations
internal/menu/             # model, repository, service (validation, tree, ordering)
internal/auth/             # password, sessions, CSRF, rate limit
internal/server/           # Echo, routes, REST handlers, middlewares, SPA fallback
frontend/                  # Vue 3 + TS + Vite+ (vp)
  vite.config.ts           # defineConfig from 'vite-plus' (vite + test + lint + fmt)
  src/main.ts, App.vue, router.ts, stores/ (pinia), api/ (fetch + CSRF)
  src/components/dhx/      # Vue wrappers: DhxLayout, DhxToolbar, DhxTabbar, DhxTree, DhxForm
  src/views/               # DashboardView, AdminView, LoginView, AdminDisabledView
  src/theme/theme.css      # blue/black and Windows XP themes, flat and 3D (DHTMLX variables)
  src/vendor/dhx-icons/    # DHTMLX icon font copied from the CDN (offline use)
  e2e/                     # Playwright
web/dist/                  # `vp build` output (generated; embedded)
```
`//go:embed all:web/dist` lives in `main.go` (existing convention) and the `fs.FS` is injected into the server. Since `go:embed` requires the directory to exist, `web/dist/.gitkeep` is versioned and the `Makefile` always runs `vp build` before `go build`; a Go build without the frontend serves a page stating that the assets were not generated.

### 2. SQLite with `modernc.org/sqlite`
A pure Go driver allows `CGO_ENABLED=0` and trivial cross-compilation. The `mattn/go-sqlite3` alternative is faster, but requires CGO and a C toolchain — unnecessary for the expected volume (hundreds of items). The pragmas `foreign_keys(1)`, `busy_timeout(5000)` and `journal_mode(WAL)` are passed in the DSN (`file:dashboard.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)`), which the driver applies **on every new connection** of the `database/sql` pool — running `PRAGMA` only once does not cover connections opened later. A test checks `PRAGMA foreign_keys` on several simultaneous connections.

Migrations: versioned embedded SQL files (`internal/database/migrations/*.sql`) applied in order with a `schema_migrations` table. The `golang-migrate` alternative was discarded to keep dependencies minimal.

### 3. Data model — one self-referencing table
```sql
CREATE TABLE menu_items (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  parent_id   INTEGER NULL REFERENCES menu_items(id) ON DELETE CASCADE,
  title       TEXT    NOT NULL,
  url         TEXT    NULL,              -- NULL = group item (only opens a submenu)
  target      TEXT    NOT NULL DEFAULT 'iframe' CHECK (target IN ('iframe','blank')),
  icon        TEXT    NULL,              -- icon class (e.g. mdi mdi-chart-line)
  description TEXT    NULL,              -- tooltip
  position    INTEGER NOT NULL DEFAULT 0,
  enabled     INTEGER NOT NULL DEFAULT 1,
  created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_menu_items_parent ON menu_items(parent_id, position);
```
One table with `parent_id` covers menu → submenu → sub-submenu without limits and maps directly to the nested `items` format of `dhx.Toolbar`. The tree is built in memory (a single ordered query). Alternatives (separate `menus`/`links` tables, closure table) add complexity with no gain at this volume. An item can have a URL **and** children; in that case the click opens the submenu and the URL becomes the first item "Open <title>" generated in the frontend.

Cycles are prevented in the service: when moving an item, the new parent can be neither the item itself nor a descendant. In batch reorders, validation is done on the **final tree** resulting from all the changes of the batch (detecting cycles formed by two combined changes), before the commit.

### 4. REST API (Echo v5)
| Method | Route | Access |
|---|---|---|
| GET | `/api/menu` | public — tree with `enabled` items only |
| GET | `/api/admin/items` | admin — complete tree |
| POST | `/api/admin/items` | admin |
| PUT | `/api/admin/items/:id` | admin — full replacement |
| DELETE | `/api/admin/items/:id` | admin (cascades to children) |
| POST | `/api/admin/items/reorder` | admin — `{"moves":[{"id":1,"parent_id":null,"position":0}]}` in a transaction |
| GET | `/api/admin/export` | admin — export document (see below) |
| POST | `/api/admin/import?mode=append\|replace` | admin — body = export document; default `append` |
| GET | `/api/admin/session` | public — `{"admin_enabled":bool,"authenticated":bool,"csrf_token":"..."\|null}` |
| POST | `/api/admin/login` | `{"password":"..."}` → 204 + cookie |
| POST | `/api/admin/logout` | admin → 204, revokes the session |
| GET | `/healthz` | public |

**Contracts:**
- `POST`/`PUT` receive `{title, url, target, icon, description, enabled, parent_id, position}`. `PUT` is a **full replacement**: omitted optional fields become `null`/default (`target=iframe`, `enabled=true`, `position=0`). There is no partial update in this version.
- Item responses: the complete object with `id`, `created_at`, `updated_at`. `reorder` responds `204`; `import` responds `{"imported": N}`.
- Errors: `400 {"error","field"}` validation, `401` no session, `403` invalid CSRF, `404` nonexistent, `413` body > 1 MiB, `429` login rate limit.

**Import/export format (versioned):**
```json
{"format":"dashboard-menu","version":1,"items":[
  {"title":"Monitoring","url":null,"target":"iframe","icon":"mdi mdi-chart-line",
   "description":null,"enabled":true,"position":0,"items":[ ... ]}]}
```
IDs are **not** exported; the hierarchy is given by nesting. On import, IDs are always generated again. `append` adds the top-level items at the end of the root (positions shifted after the existing ones); `replace` deletes everything and inserts, in the same transaction. An unknown `version` is rejected. The whole import is validated before any write and runs in a transaction — a failure on any item rolls everything back, including `replace` over a populated database.

Echo is used for consistency with the team's other projects; plain `net/http` (Go 1.22+ routing) would be enough, but we would lose ready-made middlewares (recover, request-id, logger, CORS).

### 5. Administration authentication
**Password:** a single administrator password. Sources, in order of precedence: (1) `DASHBOARD_ADMIN_PASSWORD_HASH` (ready bcrypt hash, recommended), (2) `DASHBOARD_ADMIN_PASSWORD` (plain text, hashed in memory at startup; warning logged), (3) bcrypt hash (cost 12) stored in the `settings` table by `dashboard admin set-password`. The environment variables are read at startup; the database hash is **not** kept in memory — it is re-read from the `settings` table on every login attempt, so a change made through the CLI takes effect immediately on the running server (which shares the same SQLite file). When a password environment variable is set, `admin set-password` stores the hash but warns that it **will have no effect** while the variable exists. If none exists, web administration is **disabled** (the CLI keeps working) — preferable to opening without a password by default.

**Server-side sessions** (`sessions` table): login creates a random 32-byte id (`crypto/rand`, base64url); the `dashboard_session` cookie contains only that id, with `HttpOnly`, `SameSite=Strict`, `Path=/`, and `Secure` when the request is HTTPS (direct or via `X-Forwarded-Proto` with `--trust-proxy`) or with `--cookie-secure`. The database stores the **SHA-256** of the id, `csrf_token`, `created_at`, `last_seen_at` and `expires_at`. Expiration validated on the server: 2 h idle, 12 h absolute. Logout and `admin set-password` revoke (delete) sessions; expired sessions are removed periodically. The id is rotated on every login (no session fixation). With server-side sessions no HMAC secret is needed (`DASHBOARD_SECRET` removed).

**Login:** `POST /api/admin/login`, comparison via `bcrypt.CompareHashAndPassword`, generic error message, in-memory rate limit of 5 attempts/min per IP (and 20/min global) with a `429` response; failed attempts logged without the password.

**CSRF (synchronizer token):** each session has a random `csrf_token` (32 bytes) stored in the session row itself, delivered to the frontend by `GET /api/admin/session`. `POST/PUT/DELETE` requests to `/api/admin/*` (except `login`) require `X-CSRF-Token` equal to the session's, compared with `subtle.ConstantTimeCompare`; a token from another session is rejected by construction. The token is rotated on login. `POST /api/admin/login` is protected by `SameSite=Strict` + a check of the `Origin`/`Sec-Fetch-Site` header (must be the same origin).

**HTTPS:** the binary accepts `--tls-cert`/`--tls-key`; in production direct HTTPS or a reverse proxy is recommended. Without HTTPS, the startup log warns that the password travels in clear text.

Basic Auth alternative: simpler, but without logout and with poor browser UX; discarded.

### 6. Frontend: Vue 3 + Vite+ + DHTMLX Suite
- **Vite+ toolchain**: `vite.config.ts` with `defineConfig` from `vite-plus` containing `plugins: [vue()]`, `build.outDir: '../web/dist'`, `server.port: 9000` with a `proxy` of `/api` and `/healthz` to `http://localhost:8080`, plus the `test` (Vitest), `lint` (Oxlint) and `fmt` (Oxfmt) sections. Commands: `vp install`, `vp dev` (HMR), `vp check` (formatting + lint), `vue-tsc --noEmit` (type checking of `.vue` SFCs, run explicitly — `vp check` does not cover it), `vp test`, `vp build`. Versions pinned in `package.json` and a versioned lockfile.
- **Why Vue + Vite+ instead of plain HTML/JS**: state (menu tree, open tabs, admin session) and screens (login, admin, dashboard) are organized in typed components, stores and routes; Vite+ provides a dev server with HMR, unit tests, lint and formatting with a single CLI, while keeping the final artifact static and embeddable.
- **DHTMLX ↔ Vue integration**: DHTMLX has no official Vue bindings in this package. Each view creates its widgets in `onMounted` and registers them in the `useDhx().own(...)` composable, which marks them with `markRaw` and calls `destructor()` in `onBeforeUnmount` (reverse order). Widgets attached to cells get `null` as container (the `DETACHED` constant).
- **Tabs without re-creating iframes (implementation decision)**: the Tabbar uses `noContent: true` (tab bar only). The panes — home page and one iframe per tab — are Vue elements rendered via `<Teleport>` inside the Layout `content` cell and toggled with `v-show`. That way switching tabs never re-creates the iframe (the state and login of the opened system are preserved) and all text is rendered by Vue with automatic escaping. Validated in e2e (the state of a field inside the iframe survives switching tabs).
- **DHTMLX icon font**: `suite.min.css` does an `@import` of `https://cdn.dhtmlx.com/fonts/dxi/dhx-icons.css` (used in menu arrows, the tree and the tab close X). To work offline under CSP `'self'`, the `stripDhxCdnImport` Vite plugin removes that `@import` at build time and the project imports the local copy in `src/vendor/dhx-icons/`.
- **Messages**: the API responds in English (code); the frontend maps them to user-facing messages in `src/lib/messages.ts`.
- **Tailwind CSS v4 without preflight**: only Tailwind's `theme.css` + `utilities.css` are imported, so as not to reset DHTMLX styles; utility colors come from variables redefined per theme.
- **Routes (vue-router, history mode)**: `/` → `DashboardView`; `/admin` → `AdminView`; `/admin/login` → `LoginView`; `/admin/disabled` → `AdminDisabledView`. The `/admin*` guard queries `/api/admin/session`: `admin_enabled=false` → `/admin/disabled`; `authenticated=false` → `/admin/login`; otherwise it proceeds and stores the `csrf_token`. The Go server responds with `index.html` (SPA fallback) for `/` and `/admin*`. The `#item-<id>` fragment is still used for tab deep links inside `/`.
- **State (pinia)**: `menuStore` (tree, id→item map), `tabsStore` (open tabs, LRU order, active tab), `authStore` (session and CSRF token). The tree→Toolbar items logic and the tab manager are pure functions tested with `vp test`.
- **DHTMLX via npm**: `dhx-suite` (GPL edition published on npm, version 9.3.x pinned) imported as a module (`import * as dhx from 'dhx-suite'` + `dhx-suite/codebase/suite.min.css`) and bundled by the build — nothing is loaded from a CDN. For the commercial edition, the package is swapped for the one from DHTMLX's private registry without changing code.
- **DashboardView**: structure built with **DHTMLX Layout** (https://dhtmlx.com/docs/products/dhtmlxLayout/), filling 100% of the viewport:
  ```js
  const layout = new dhx.Layout("app", { type: "none", rows: [
    { id: "header",  height: "content" },   // dhx.Toolbar attached: layout.getCell("header").attach(toolbar)
    { id: "content" },                      // dhx.Tabbar attached:  layout.getCell("content").attach(tabbar)
    { id: "footer",  height: "content", hidden: true } // optional: version/footer
  ]});
  ```
  The Layout handles resizing (fixed-height header, content filling the rest) without manual positioning CSS. The **administration** uses the same Layout with an additional column: `rows: [header, { cols: [ { id: "tree", width: 320, resizable: true }, { id: "form" } ] }]`, with `dhx.Tree` and `dhx.Form` attached to the cells. The toolbar contains, on the left, the logo/title (`title` item), then one `menuItem` per top-level item with its children in `items` (dropdowns nested at any depth), a `spacer` and, on the right, fixed actions (Home, Admin). Toolbar ID mapping: database items → `item-<id>`; synthetic "Open <title>" entry of a group with a URL → `open-<id>`; fixed actions → `action-home`, `action-admin`; title → `title` (not clickable). Nodes with children become a `menuItem` with `items` (dropdown nested at any depth, per the Suite 9 `MenuItem` API); leaves become a `menuItem` without `items` at the first level or dropdown entries at the others. The text (`value`/`tooltip`) is treated as plain text (escaped before being handed to the component) to avoid XSS via the title. The `toolbar.events.on("click", (id, event) => ...)` event dispatches by prefix and decides between iframe and new tab. Dropdowns open on **click** (the Toolbar default behaviour), not on hover. `Toolbar` was chosen instead of `dhx.Menu` because it holds, in the same bar, title, menus with submenus, separators, spacer and action buttons, keeping a single component in the header.
- Click on an item with `target=iframe` → opens/activates the item's tab in the Tabbar (6.2) and sets `location.hash = #item-<id>`. `target=blank` → `window.open(url, '_blank', 'noopener,noreferrer')`.
- Home page: grid of cards with the top-level items and their links, shown in the fixed "Home" tab.
- **AdminView**: `dhx.Layout` (header + resizable columns) with `dhx.Tree` (drag-and-drop for hierarchy/order → `/reorder`) + `dhx.Form` (title, URL, target, icon, description, enabled) + New/Save/Delete buttons, and `dhx.Window` to import/export.
- DHTMLX, Vue and icons are bundled into `web/dist` by `vp build` — no CDN, the binary works offline/on a closed network.
- Icons: Material Design Icons via `@mdi/font` (classes `mdi mdi-*`, already supported by DHTMLX), bundled in the build.
- **Deep link**: a nonexistent, disabled or group (URL-less) `#item-<id>` → shows the home page and a `dhx.message` notice "Item not found"; `#item-<id>` with `target=blank` → does not open a popup automatically (blocked by the browser), shows the home page with the item highlighted.

### 6.2 Central area with DHTMLX Tabbar
The central area uses `dhx.Tabbar` (https://dhtmlx.com/docs/products/dhtmlxTabbar/) to allow several systems to be open simultaneously without leaving the page:
- Fixed `tab-home` tab ("Home", not closable, always the first) with the home page.
- Click on an `iframe` item: if the `tab-<id>` tab already exists, it is just activated with `tabbar.setActive(id)` (the iframe is **not** reloaded, preserving the state of the opened system); otherwise it is created with `tabbar.addTab({id: "tab-<id>", tab: <title>, icon}, index)` and the cell receives the HTML via `tabbar.getCell(id).attachHTML(html)`: context bar (title, **Open in new tab**, **Reload**) + sandboxed `<iframe>`. Iframes of inactive tabs stay in the DOM (hidden) to keep their state.
- **Closing**: `closable` is a **Tabbar** setting (`boolean | string[]`), not a tab setting. The list form is used: `closable` contains only the open `tab-<id>` IDs (never `tab-home`) and is updated on every open/close before repainting; as an extra safeguard, `beforeClose(id)` returns `false` for `tab-home`. Closing a tab (`removeTab`) removes the iframe (frees memory) and activates the neighbouring tab.
- Configurable limit of open tabs (`--max-tabs`, default 10, excluding "Home"): when the limit is reached, opening a new tab automatically closes the least recently used tab (LRU) and shows a `dhx.message` notice.
- URL synchronization: the `change(id, prev)` event updates `location.hash` (`#item-<id>`, or empty for "Home"); `hashchange` (back/forward, deep link) activates or opens the matching tab. Open tabs are not persisted across reloads (only the hash tab is reopened).
- The toolbar "Home" button activates `tab-home`.
- Tabbar was chosen instead of a single iframe because the typical use case is switching between several dashboards (e.g. Grafana and Zabbix) without losing the state/login of each one. Suite 9 contracts used (checked against the documentation): Tabbar — `closable` config, `addTab`, `removeTab`, `setActive`, `getCell(id).attachHTML` methods, `change(id, prev)` and `beforeClose(id)` events; Layout — `rows`/`cols`, `getCell(id).attach`; Toolbar — `MenuItem` with nested `items` and the `click(id, event)` event; theme — CSS variables including `--dhx-border-radius`. A smoke test on the exact version installed by the lockfile (task 7.3c) confirms nothing changed.

### 6.1 Themes — data registry, flat and 3D, WCAG AAA, no rounded corners
Each theme is a TypeScript object in `frontend/src/theme/registry/<theme>.ts` (`ThemeDefinition`): ~45 color tokens (page, surfaces, text, links, primary, selection, toolbar, tabs, buttons, cards, fields, scrollbar…), font, optional gradients of the **No Flat** style and optional effects (beveled `bevel` for retro themes, `glow` for Matrix, position of the tab highlight). `applyTheme()` (`src/lib/theme.ts`) turns the theme into CSS variables on `<html>` — `--c-*` (colors), `--g-*` (solid color in flat or gradient in 3D), `--shadow-card`, `--btn-shadow-3d`, `--text-glow`, `--tab-accent-shadow` — and into the DHTMLX variables (`--dhx-color-primary`, `--dhx-background-*`, `--dhx-font-*`, primary HSL). `theme.css` contains only components and **never** per-theme rules; the Tailwind utilities use the same tokens via `@theme inline`.

Registered themes (grouped **Theme** menu):

| Group | Themes |
| --- | --- |
| Dark themes | Blue and Black (default), Dark, Graphite, Midnight |
| Light themes | Light |
| Brand | Ocean Light, Ocean Dark — Ocean palette: navy `#1d3c55`, blue `#285783`, turquoise `#34cdd7` only as an accent, borders `#d6e3ea`, embedded **Inter** font (`src/vendor/fonts/inter`, SIL OFL) |
| Brand (Criare) | Bio Manager (`bio-manager`; the old name `ad-manager` is still accepted as an alias) — Criare pattern of the ad-manager project (`frontend/src/themes/default.css` + `style.css`): navy `#113058`, orange `#f58322` as an accent (bar under the header, bar on top of the active tab, line under the card titles via the `cardHeaderBorder` token), page `#d9e6f3`, header `#e7ecef`, dark tab strip, Inter |
| Inspired | Brasil (flag colors: green toolbar, indigo-blue tabs, yellow active tab, cards with a green header; green `#009c3b` only on focus, text on darkened greens); Google (Material, Roboto embedded by DHTMLX), Microsoft (Fluent, blue bar, Segoe UI), Super Mario Galaxy (deep indigo space, violet nebula bar, golden Power Star accents, Luma blue selection, golden glow and nebula gradient in No Flat), Winamp (classic 2.x skin: graphite frames, golden titles, green LCD text) — palettes only, without logos or brand assets; blues/reds/greens darkened for AAA |
| Systems | macOS (Big Sur to Sequoia, light appearance: light bar, white content on gray `#f5f5f7`, text `#1d1d1f`, system blue `#007aff` on focus and accent, system font — palette only), Ubuntu (Yaru: aubergine bar `#2c001e`, light surfaces, Ubuntu orange on focus and on the active tab, light orange selection, Ubuntu font — palette only), Windows 10 (dark taskbar bar, blue accent `#0078d7`, light flat surfaces, Segoe UI — palette only), Linux KDE (Breeze: panel `#31363b`, selection `#3daee9`, Noto Sans — palette only), xterm (black background, text `#e5e5e5`, ANSI colors, inverted selection, monospace), Windows 7 (Aero: translucent dark blue bar like the taskbar, light blue frames and tabs, card headers like the light Explorer command bar with a navy title, light blue selection, glows in No Flat — palette only), Windows Vista (Aero: dark "glass" bar, bluish light Explorer surfaces, Aero blue headers, light blue selection with black text, light buttons with blue highlight, gradient glows in 3D — palette only), Windows XP (Luna), Windows 98 (3D gray, navy titles, teal desktop, bevel), Classic Mac OS (Platinum 8/9, bevel) |
| Special | Matrix (phosphor green, monospace, glow in 3D), High Contrast |

**Contrast (AAA+)**: `src/lib/contrast.ts` lists every pair the interface renders (text/page, text/surface, secondary, links, error, fields, primary/secondary buttons and their states, selection, toolbar and hover, active/inactive tabs, card header and links, panel titles) and, for each theme, also each stop of the 3D gradients. The test requires **≥ 7:1 for text (WCAG AAA)**, ≥ 4.5:1 for control icons (tab close) and ≥ 3:1 for the focus ring, scrollbar and active tab highlight. Brand colors that do not reach AAA (turquoise on light, Luna blue `#0054E3`, Win98 title `#1084D0`) were darkened or restricted to decorative use. Adding a theme = create the file, register it in `registry/index.ts` and map the group in `e2e/helpers.ts`; the test validates the contrast automatically.

**Bars and scaling**: toolbar 40 px, tabs 32 px, font 13 px, controls 28 px (Full HD). `--ui-scale` = 1 / 1.25 (≥ 2300 CSS px, QHD) / 1.6 (≥ 3400 CSS px, 4K at 100%) multiplies every measure, including `--dhx-step-size` (DHTMLX fonts and inner spacing); sizes configured in JS (tab width/height, tree column) read the scale from `src/lib/scale.ts` and are updated when crossing a breakpoint. DHTMLX writes an inline height on the toolbar attached to the Layout; the CSS overrides it with the scaled height. Thin scrollbars with the theme colors; the administration form has a maximum width on very wide screens.

### 7. Iframe and security
- **Default sandbox**: `sandbox="allow-scripts allow-same-origin allow-forms allow-popups"` and `referrerpolicy="no-referrer"`. Rationale: `allow-scripts` + `allow-same-origin` are needed so that external dashboards (Grafana, Zabbix etc.) work with their own cookies/login; `allow-forms` for login screens; `allow-popups` for flows that open windows. `allow-top-navigation`, `allow-popups-to-escape-sandbox`, `allow-downloads` and `allow-modals` are **not** granted. The attribute is configurable (`--iframe-sandbox`) for specific cases.
- The risk of `allow-scripts` + `allow-same-origin` (content removing its own sandbox and accessing the dashboard) only exists for content **from the same origin as the dashboard**. That is why the backend **rejects** iframe URLs whose origin is the dashboard's own (compared with `--public-url`, when configured, and with the `Host` of the request in use), and the frontend repeats the check against `location.origin` before loading; in those cases the item can only have `target=blank`. The dashboard does not host user content on its origin (no uploads).
- URLs accepted only with the `http`/`https` scheme (backend validation) — prevents `javascript:`/`data:`.
- **Complete CSP** on HTML responses:
  `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'; frame-src <frame-src>`.
  `frame-src` is configurable (`--frame-src`), default `http: https:`. `style-src 'unsafe-inline'` is needed because DHTMLX applies inline styles at runtime; no inline script is used — `vp build` produces only `<script type="module" src=...>` and the Vue templates are precompiled (*runtime-only* build, without `eval`/`new Function`, compatible with `script-src 'self'`), validated by a browser test with no CSP violations in the console.
- Other headers: `X-Frame-Options: SAMEORIGIN`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, a restrictive `Permissions-Policy`; `Strict-Transport-Security` only under HTTPS.
- 1 MiB body limit on the `/api/admin/*` routes.
- It is not possible to reliably detect, via JS, whether a cross-origin site blocked the iframe; that is why the fallback is the bar with "Open in new tab" and the per-item `target` field.

### 8. CLI (Cobra + viper)
```
dashboard serve   [--addr :8080] [--db ./dashboard.db] [--dev] [--public-url URL] [--max-tabs 10] [--seed menu.json] [--title T]
                  [--tls-cert F --tls-key F] [--cookie-secure] [--trust-proxy]
                  [--frame-src SRC] [--iframe-sandbox TOKENS]
dashboard migrate [--db ...]
dashboard menu list [--tree|--json]
dashboard menu add --title T [--url U] [--target iframe|blank] [--parent ID] [--icon I] [--position N]
dashboard menu update ID [...flags]
dashboard menu remove ID [--force]
dashboard menu export [-o file.json]
dashboard menu import file.json [--replace]
dashboard admin set-password
dashboard backup -o backup.db
dashboard version
```
The `menu` commands use the same `internal/menu.Service` as the API, guaranteeing the same validations. Configuration: flags > env `DASHBOARD_*` > `dashboard.toml` > defaults.

### 9. Build
`Makefile` with `frontend` (`cd frontend && vp install && vp check && npx vue-tsc --noEmit && vp build`), `build` (depends on `frontend`; `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=..."`), `test` (`go test ./...` + `vp test`), `e2e` (Playwright against the binary), `dev` (Go on `:8080` + `vp dev` on `:9000`). Optional multi-stage Dockerfile: Node stage with Vite+ → Go stage → `distroless/static`.

## Risks / Trade-offs

- [Many tabs with iframes consume browser memory] → `--max-tabs` limit with LRU closing.
- [Many sites (Google, GitHub, Grafana with default config etc.) block iframes via `X-Frame-Options`/`frame-ancestors`] → per-item `target` field, "Open in new tab" bar and documentation; we will not proxy.
- [DHTMLX license: the GPL edition requires the distributed binary to also be GPL; closed commercial use requires a paid license] → decide before implementation (Open Questions); access to DHTMLX is isolated in the `components/dhx/` wrappers, and switching editions is just swapping the npm package.
- [SQLite with concurrent writes] → WAL + `busy_timeout`; writes are rare (admin only).
- [Single admin password] → acceptable for internal scope; the model leaves room for a future `users` table.
- [Binary size grows with DHTMLX (~1-2 MB), Vue and icons] → acceptable; assets with a hash in the name served with `Cache-Control: immutable`, `index.html` without cache, gzip compression.
- [Vite+ is recent (1.x) and evolves fast] → version pinned in the lockfile; the configuration is standard Vite, so migrating to plain `vite` + `vitest` is trivial if needed.
- [Vue wrappers for DHTMLX may leak instances] → `destructor()` mandatory in `onBeforeUnmount`, covered by a mount/unmount test.
- [`modernc.org/sqlite` slower than CGO] → irrelevant for the expected volume.

## Migration Plan

New project: there is no data migration. Deploy = copy the binary and run `dashboard serve`; the database is created on the first run. Rollback = replace with the previous binary (migrations are additive only in this version). Backup = `dashboard backup -o file.db`, which uses `VACUUM INTO` (consistent copy, including transactions still in the WAL, with the server running). Do **not** copy only `dashboard.db` with the server running — the contents of the `-wal` may be lost. Restore = stop the service, replace `dashboard.db` and remove `-wal`/`-shm`; covered by an automated test.

## Open Questions

- **Gate before implementation (task 0.1):** DHTMLX Suite edition — GPL (npm package `dhx-suite`, GPL license, which imposes GPL on the distributed project) or commercial (DHTMLX's private npm registry). The *trial* version is not acceptable for delivery. Target version: Suite 9.3.x (latest on npm: 9.3.2), pinned in the lockfile.
- Should the menus be the same for every visitor, or will there be items visible only after login? (this proposal assumes public menus, the same for everyone)
- Logo to show in the toolbar title (the blue/black theme is already defined).
- Desired default port and database path in production (e.g. `:8080`, `/var/lib/dashboard/dashboard.db`)?
