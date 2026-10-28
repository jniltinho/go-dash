# base-path Specification

## Purpose
TBD - created by archiving change add-base-path. Update Purpose after archive.
## Requirements
### Requirement: Configurable base path
The server SHALL accept a `base_path` setting (flag `--base-path`, env
`DASHBOARD_BASE_PATH`, TOML `base_path`), default `/`. The value SHALL be
normalized to start and end with `/` (`panel` → `/panel/`). Each segment
SHALL match `[A-Za-z0-9._~-]+` and SHALL NOT be `.` or `..`; the value SHALL
NOT contain `//`, query, fragment or `%`-encoded characters, SHALL have at
most 100 characters, and its first segment SHALL NOT be a name the
application serves at its root: `api`, `admin`, `assets`, `dash`, `healthz`,
`index.html` or any file in the root of the frontend build (e.g.
`favicon.svg`). An invalid value SHALL stop the start with a configuration
error.

#### Scenario: Default
- **WHEN** no base path is configured
- **THEN** the application behaves exactly as before, served at `/`

#### Scenario: Normalization
- **WHEN** the operator sets `DASHBOARD_BASE_PATH=panel`
- **THEN** the effective base path is `/panel/`

#### Scenario: Invalid value
- **WHEN** the operator sets `--base-path /admin/`, `/a/../b/`, `/ops/./dash/`, `/favicon.svg/` or `/pai%6Eel/`
- **THEN** `dashboard serve` exits with an error naming `base_path`

#### Scenario: public_url with another path
- **WHEN** `public_url` is `https://host/other/` and `base_path` is `/panel/`
- **THEN** `dashboard serve` exits with an error explaining that the path of `public_url` must equal `base_path`

### Requirement: Both reverse proxy styles
With a base path other than `/`, the server SHALL serve the same responses
whether the proxy keeps the prefix (request `/panel/api/menu`) or strips it
(request `/api/menu`). The prefix SHALL be removed only on a segment
boundary: when the path equals the base path without its trailing slash, or
starts with the full base path (with the slash); `/panel-x/...` SHALL NOT be
touched. When it is removed, the escaped form of the path (`RawPath`) SHALL be
rewritten consistently, so encoded characters after the prefix keep their
meaning. A request whose path is exactly the base path without the trailing
slash (`/panel`) SHALL be redirected (301) to the base path (`/panel/`),
keeping the query string.

#### Scenario: Prefix kept by the proxy
- **WHEN** nginx uses `location /panel/ { proxy_pass http://127.0.0.1:8080; }` and a browser opens `/panel/dash/acme`
- **THEN** the SPA for dashboard `acme` is served

#### Scenario: Prefix stripped by the proxy
- **WHEN** nginx uses `location /panel/ { proxy_pass http://127.0.0.1:8080/; }` and a browser opens `/panel/dash/acme`
- **THEN** the same SPA is served and every asset and API call succeeds

#### Scenario: Sibling prefix
- **WHEN** the base path is `/panel/` and a request comes for `/panel-x/api/menu`
- **THEN** the path is not stripped and the request is handled as `/panel-x/api/menu` (404)

#### Scenario: Missing trailing slash
- **WHEN** a browser opens `/panel?x=1`
- **THEN** it is redirected to `/panel/?x=1`

### Requirement: Prefixed URLs emitted by the server
Every URL the server emits SHALL carry the base path: the `<base href>`
injected into `index.html`, `Location` headers of its redirects (canonical
`/dash/...` routes and `/dash/default`), and the `Path` attribute of the admin
session and dashboard viewer cookies. `/healthz` SHALL answer both with and
without the prefix. API error bodies and JSON payloads SHALL NOT change.

#### Scenario: Redirect under a base path
- **WHEN** the base path is `/panel/` and a browser requests `/panel/dash/Acme/`
- **THEN** the response is 301 with `Location: /panel/dash/acme`

#### Scenario: Cookie scope
- **WHEN** an admin logs in on an instance with base path `/panel/`
- **THEN** the session cookie has `Path=/panel/` and is not sent to other applications on the same host

#### Scenario: Two instances on one host
- **WHEN** instances run at `/panel/` and `/ops/` on the same host
- **THEN** logging into one does not log into or out of the other

### Requirement: Single frontend build for any base path
The frontend SHALL be built once with relative asset URLs and SHALL derive the
base path at runtime from the `<base href>` element injected by the server.
Client routes, API requests, links and the favicon SHALL resolve under it.
When the document has no `<base>` element (the Vite dev server), the base
path SHALL be `/`, never the current route. The UI SHALL NOT contain
root-absolute links (`href="/..."`); in-app links SHALL go through the
router. No inline script SHALL be added (CSP stays `script-src 'self'`).

#### Scenario: Deep link under a base path
- **WHEN** a browser loads `/panel/admin` directly
- **THEN** scripts, styles, fonts and icons load from `/panel/assets/...` and the Admin opens

#### Scenario: In-app navigation
- **WHEN** a viewer on `/panel/` clicks "Admin" and then "Dashboard"
- **THEN** the URL goes to `/panel/admin` and back to `/panel/`, never to `/admin` or `/`

#### Scenario: Empty menu screen
- **WHEN** the default dashboard has no items under `/panel/` and the viewer follows the link to the Admin
- **THEN** the browser goes to `/panel/admin`

#### Scenario: Dev server deep link
- **WHEN** a developer opens `http://localhost:9000/admin/login` on the Vite dev server
- **THEN** the base path is `/` and API calls go to `/api/...`

#### Scenario: No CSP violation
- **WHEN** the application runs under `/panel/`
- **THEN** the browser reports no Content Security Policy violation and makes no request outside the base path except to iframe targets

### Requirement: Per-instance browser storage
The frontend SHALL namespace every per-viewer `localStorage` key (theme,
card order) by the base path when it is not `/`, so instances on the same
origin keep separate preferences. With base path `/` the keys SHALL stay as
they are. Migrations of legacy keys (such as the pre-dashboards card order)
SHALL run only for base path `/`, so an instance under a prefix never reads
or deletes the preferences of the root instance.

#### Scenario: Legacy card order stays with the root instance
- **WHEN** the legacy key `dashboard.cardOrder` exists and a viewer opens `/panel/`
- **THEN** the key is neither read nor removed

#### Scenario: Separate themes
- **WHEN** a viewer picks a dark theme on `/panel/` and opens `/ops/` on the same host
- **THEN** `/ops/` still uses its own saved or default theme

