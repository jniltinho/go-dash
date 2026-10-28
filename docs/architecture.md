# Architecture

The dashboard is a single Go binary that serves a Vue 3 single-page app and a
JSON API, stores menu items in SQLite and embeds the whole frontend build.

## Components

```mermaid
flowchart LR
    subgraph Browser
        SPA["Vue 3 SPA<br/>(Pinia, vue-router)"]
        DHX["DHTMLX Suite 9<br/>Layout · Toolbar · Tabbar · Tree · Form"]
        IF["iframes<br/>(sandboxed, one per tab)"]
        SPA --- DHX
        SPA --> IF
    end

    subgraph Binary["dashboard (single Go binary)"]
        CLI["Cobra CLI<br/>serve · migrate · menu · admin · backup · version"]
        CFG["viper config<br/>flags > DASHBOARD_* env > TOML > defaults"]
        HTTP["Echo v5 server<br/>security headers · CSP · gzip · slog"]
        API["REST API<br/>/api/dash/:slug/* · /api/menu · /api/config · /api/admin/*"]
        AUTH["auth<br/>bcrypt · server-side sessions · CSRF · rate limit"]
        MENU["menu service<br/>validation · tree · reorder · import/export"]
        DBP["database<br/>embedded migrations · per-connection pragmas · backup"]
        EMB[("embedded web/dist<br/>go:embed")]
        CLI --> CFG
        CLI --> HTTP
        CLI --> MENU
        HTTP --> API
        HTTP --> EMB
        API --> AUTH
        API --> MENU
        AUTH --> DBP
        MENU --> DBP
    end

    SQL[("SQLite<br/>dashboard.db (WAL)")]
    EXT["External systems<br/>Grafana, Zabbix, wikis…"]

    SPA -- "HTTPS / JSON" --> HTTP
    DBP --> SQL
    IF -. "loads URL" .-> EXT
```

## Repository layout

```mermaid
flowchart TB
    root["/"] --> main["main.go<br/>go:embed all:web/dist"]
    root --> cmd["cmd/<br/>Cobra commands"]
    root --> internal["internal/"]
    internal --> cfg["config/ — viper (TOML + ENV)"]
    internal --> db["database/ — SQLite, migrations, backup"]
    internal --> menu["menu/ — domain rules"]
    internal --> auth["auth/ — password, sessions, CSRF"]
    internal --> server["server/ — Echo v5, handlers, static/SPA"]
    root --> fe["frontend/ — Vue 3 + TS + Vite+"]
    fe --> src["src/ api · lib · stores · views · components · theme · vendor"]
    fe --> e2e["e2e/ — Playwright"]
    root --> web["web/dist/ — build output (embedded)"]
    root --> os["openspec/ — proposal, design, specs, tasks"]
```

## Serving under a sub-path

With `base_path = "/panel/"` the server accepts both `/panel/api/menu`
(proxy keeps the prefix) and `/api/menu` (proxy strips it): an `e.Pre`
middleware removes the prefix on a segment boundary before routing. Every URL
it emits carries the prefix: `index.html` gets `<base href="/panel/">`
(always injected, also for `/`), redirects use `/panel/...` and cookies have
`Path=/panel/`. The frontend is built once with relative asset URLs and reads
the base from the `<base>` element (`src/lib/basePath.ts`) for the router
history, API calls and `localStorage` keys.

```mermaid
flowchart LR
    B["Browser<br/>/panel/dash/acme"] --> N["nginx<br/>location /panel/"]
    N -->|"keeps: /panel/dash/acme"| P["Pre middleware<br/>strip /panel"]
    N -->|"strips: /dash/acme"| P
    P --> R["routes at the root<br/>/dash/:slug, /api/*"]
    R --> I["index.html + &lt;base href=/panel/&gt;"]
```

## Request flow: opening a link in a tab

```mermaid
sequenceDiagram
    actor U as User
    participant T as DHTMLX Toolbar
    participant V as DashboardView (Vue)
    participant TB as DHTMLX Tabbar
    participant P as Pane (iframe)
    participant S as Go server

    U->>V: open /
    V->>S: GET /api/config, GET /api/menu
    S-->>V: config + enabled menu tree
    V->>T: parse(buildToolbar(tree))
    U->>T: click "Maps" ▸ "OpenStreetMap"
    T->>V: click("item-8")
    alt target = blank
        V->>U: window.open(url, noopener, noreferrer)
    else target = iframe
        V->>V: TabTracker.openItem(8) (LRU, max_tabs)
        V->>TB: addTab("tab-8") / setActive("tab-8")
        V->>P: render sandboxed iframe (v-show keeps state)
        V->>U: location.hash = #item-8
    end
```

## Admin authentication

```mermaid
sequenceDiagram
    actor A as Admin
    participant SPA
    participant S as Go server
    participant DB as SQLite

    SPA->>S: GET /api/admin/session
    S-->>SPA: {admin_enabled, authenticated:false}
    A->>SPA: password
    SPA->>S: POST /api/admin/login (Origin checked, 5/min per IP)
    S->>DB: read bcrypt hash (env hash > env plain > settings)
    S->>DB: INSERT sessions(sha256(id), csrf_token, expiry)
    S-->>SPA: 204 + cookie (HttpOnly, SameSite=Strict, Secure on HTTPS)
    SPA->>S: GET /api/admin/session
    S-->>SPA: {authenticated:true, csrf_token}
    SPA->>S: PUT /api/admin/items/5 + X-CSRF-Token
    S->>DB: validate session (2h idle / 12h absolute) and token
    S-->>SPA: 200 item
```

## Dashboards and viewer sessions

The default dashboard (id 1, no slug) is served at `/`; named dashboards at
`/dash/<slug>`. The SPA asks `GET /api/dash/:slug/status` first (`default` is
the API sentinel for `/`) and shows the unlock screen, "Dashboard not
found" or the dashboard. Viewer sessions live in `sessions` with
`kind = 'viewer'` and a `dashboard_id`; the cookie is `dashboard_view_<id>`.

```mermaid
sequenceDiagram
    actor V as Visitor
    participant SPA
    participant S as Go server
    participant DB as SQLite

    SPA->>S: GET /api/dash/initech/status
    S-->>SPA: {title, protected:true, unlocked:false}
    V->>SPA: password
    SPA->>S: POST /api/dash/initech/unlock (Origin checked, 4 KiB, per-IP limit)
    S->>DB: bcrypt check, then INSERT session ... WHERE password_version unchanged
    S-->>SPA: 204 + dashboard_view_4 cookie (HttpOnly, SameSite=Strict)
    SPA->>S: GET /api/dash/initech/menu
    S->>DB: session kind=viewer AND dashboard_id=4 (or a valid admin session)
    S-->>SPA: 200 tree, Cache-Control: private, no-store
    Note over SPA,S: password changed → 401 "dashboard locked" → tabs, iframes and tree are discarded
```

Admin routes are scoped by dashboard: `/api/admin/dashboards/:dashboardID/items…`
(`:dashboardID` is numeric or `default`); the old `/api/admin/items`,
`/export` and `/import` act on the default dashboard.

## Data model

```mermaid
erDiagram
    dashboards ||--o{ menu_items : "dashboard_id (ON DELETE CASCADE)"
    dashboards ||--o{ sessions : "viewer sessions (ON DELETE CASCADE)"
    menu_items ||--o{ menu_items : "parent_id (ON DELETE CASCADE)"
    dashboards {
        INTEGER id PK "1 = default"
        TEXT slug UK "NULL only for the default"
        TEXT title "empty on default = configured title"
        TEXT password_hash "bcrypt; NULL = public"
        INTEGER password_version "bumped on every change"
        INTEGER is_default
    }
    menu_items {
        INTEGER id PK
        INTEGER dashboard_id FK
        INTEGER parent_id FK "NULL = top level"
        TEXT title "1..120 chars"
        TEXT url "NULL = group; http/https only"
        TEXT target "iframe | blank"
        TEXT icon "MDI class"
        TEXT description "tooltip"
        INTEGER position
        INTEGER enabled
        TEXT created_at
        TEXT updated_at
    }
    settings {
        TEXT key PK "admin_password_hash"
        TEXT value
        TEXT updated_at
    }
    sessions {
        TEXT id_hash PK "sha256 of cookie id"
        TEXT csrf_token
        INTEGER created_at
        INTEGER last_seen_at
        INTEGER expires_at
        TEXT kind "admin | viewer"
        INTEGER dashboard_id FK "viewer sessions only"
    }
    schema_migrations {
        TEXT version PK
        TEXT applied_at
    }
```

## Build and deployment

```mermaid
flowchart LR
    subgraph S1["Stage 1 · node:24"]
        A1["npm ci"] --> A2["vp check<br/>vue-tsc<br/>vp test"] --> A3["vp build → web/dist"]
    end
    subgraph S2["Stage 2 · golang:1.26-alpine"]
        B1["go vet · go test"] --> B2["CGO_ENABLED=0 go build<br/>(embeds web/dist)"]
    end
    subgraph S3["Stage 3 · alpine:3.22"]
        C1["/usr/local/bin/go-dash<br/>non-root · /data volume<br/>HEALTHCHECK /healthz"]
    end
    A3 --> B2 --> C1
```

## Key decisions

| Decision | Why |
| --- | --- |
| `modernc.org/sqlite` (pure Go) | `CGO_ENABLED=0`, static binary, trivial cross-compile. |
| Pragmas in the DSN | `database/sql` pools connections; DSN pragmas apply to every connection. |
| Server-side sessions | Revocable on logout/password change; no signing secret to manage. |
| One viewer cookie per dashboard (by id) | Scoped revocation, survives slug renames, a viewer token is never an admin token. |
| `password_version` on unlock | A session verified with an old password can never be stored after a change. |
| Migration 0003 rebuilds `menu_items` | SQLite cannot add a `REFERENCES` column with a non-NULL default; the runner disables foreign keys only for that file and restores them before the connection returns to the pool. |
| Tabbar with `noContent` + Vue panes | Iframes are never re-created on tab switch, so each system keeps its state. |
| DHTMLX icons vendored | `suite.min.css` imports them from a CDN; the app must work offline under a `'self'` CSP. |
| Tailwind without preflight | Utilities for our own screens without resetting DHTMLX styles. |
| Themes as data (registry) | 12 themes, each flat or 3D, applied as CSS variables; adding a theme is one file, and a test enforces WCAG AAA contrast for all of them. |
| `--ui-scale` breakpoints | Same proportions on Full HD, QHD and 4K; JS-sized DHTMLX parts read the scale from CSS. |
| Inter bundled | Ocean themes typeface without external font requests. |
