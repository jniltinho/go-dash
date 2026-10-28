## Why

Today the links to internal tools (monitoring, wikis, dashboards, administrative systems) are scattered across bookmarks and documents, without a single point of access. An aggregator dashboard, distributed as a single Go binary with no external dependencies, makes it possible to centralize and organize these links in menus and open them inside the dashboard itself or in a new tab.

## What Changes

- New `dashboard` binary in Go with a Cobra-based CLI (`serve`, `migrate`, `menu ...`, `admin`, `backup`, `version`).
- Persistence in SQLite (pure Go driver, no CGO) with embedded migrations and automatic database creation.
- **Vue 3 + TypeScript** frontend with the **Vite+** toolchain (`vite-plus`, CLI `vp`: dev, check, test, build), using DHTMLX Suite (npm package `@dhtmlx/suite`, GPL edition, bundled in the build, no CDN — including the icon font, copied into the project) and Tailwind CSS v4 (without preflight) on the custom screens; the build (`web/dist`) is embedded in the binary via `//go:embed`.
- Header bar with a hierarchical **Menu** and **submenus** using the **DHTMLX Toolbar** component (`dhx.Toolbar`, items with nested dropdowns), loaded dynamically from the database.
- Central area with **DHTMLX Tabbar** (`dhx.Tabbar`): a fixed "Home" tab and one closable tab per opened link, each with its own iframe — several systems open at the same time on the same page.
- Each menu item with a URL defines how it opens: `iframe` (in a Tabbar tab in the central area of the same page) or `blank` (new blank browser tab/window).
- JSON REST API to read the menu tree and for administrative CRUD of items.
- **Blue and Black** and **Windows XP** visual themes, each in the **Flat** and **No Flat** styles, without rounded corners, selectable in the toolbar and remembered per browser.
- Administration screen (DHTMLX Tree + Form) to create, edit, reorder and remove menus, protected by a configurable credential.
- CLI commands to manage menus without the web interface (add, list, remove, import/export JSON).

## Capabilities

### New Capabilities
- `menu-management`: hierarchical data model of menus/links in SQLite, validation rules, REST API and administration CLI commands (CRUD, ordering, import/export).
- `dashboard-ui`: embedded web interface with a DHTMLX Toolbar header with menus/submenus, central area with a Tabbar of iframes, opening in a new tab, home page and administration screen.
- `server-cli`: Cobra binary, `serve` command, configuration (flags/env/file), embedded assets, automatic migrations, health check and security headers.

### Modified Capabilities
<!-- None: new project, no existing specs. -->

## Impact

- **Security**: server-side sessions, CSRF via synchronizer token, complete CSP and a restricted iframe sandbox.
- **New code**: `main.go`, `cmd/`, `internal/{config,database,menu,auth,server}`, `frontend/` (Vue 3 + Vite+), `web/dist/` (build output, embedded).
- **Go dependencies**: `github.com/spf13/cobra`, `github.com/spf13/viper`, `github.com/labstack/echo/v5`, `modernc.org/sqlite`, `log/slog` (Echo v5 default), `golang.org/x/crypto/bcrypt`. No ORM (`database/sql`).
- **Frontend dependencies**: `vue`, `vue-router`, `pinia`, `@dhtmlx/suite` (9.3.x), `@mdi/font`; dev: `vite-plus`, `@vitejs/plugin-vue`, `tailwindcss` + `@tailwindcss/vite`, `typescript`, `vue-tsc`, Playwright.
- **Build**: requires Node.js (managed by `vp env`) only to compile; the final binary still has no runtime dependencies.
- **Licensing**: DHTMLX Suite has a GPL edition and a commercial edition — the choice affects the distribution of the binary (see design.md).
- **Operations**: single binary + `dashboard.db` file; no external services.
