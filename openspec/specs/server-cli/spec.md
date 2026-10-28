# server-cli Specification

## Purpose
TBD - created by archiving change add-link-dashboard. Update Purpose after archive.
## Requirements
### Requirement: Single self-contained binary
The project SHALL compile to a single Go binary with `CGO_ENABLED=0`, embedding (via `go:embed`) the frontend build (`web/dist`, produced by `vp build`) and the SQL migrations. The binary MUST NOT require Node.js at runtime.

#### Scenario: Run without extra files
- **WHEN** only the binary is copied to an empty directory and `dashboard serve` is executed
- **THEN** the server starts, creates the SQLite database and serves the complete interface

### Requirement: Cobra command structure
The binary SHALL use Cobra and expose the commands `serve`, `migrate`, `menu`, `admin`, `backup` and `version`, with `--help` available on all of them.

#### Scenario: Version command
- **WHEN** the user runs `dashboard version`
- **THEN** the version, commit and build date injected via `ldflags` are printed

#### Scenario: Unknown command
- **WHEN** the user runs `dashboard foo`
- **THEN** the binary prints an error and suggestions and exits with a non-zero code

### Requirement: Configuration precedence
The system SHALL read configuration with the precedence flags > `DASHBOARD_*` environment variables > TOML file (`./dashboard.toml` read automatically when it exists, or `--config file.toml`) > default values. The defaults MUST be `addr=:8080` and `db=./dashboard.db`.

#### Scenario: Env overrides default
- **WHEN** `DASHBOARD_ADDR=:9090` is set and `serve` is executed without `--addr`
- **THEN** the server listens on port 9090

#### Scenario: Flag overrides env
- **WHEN** `DASHBOARD_ADDR=:9090` is set and `serve --addr :7070` is executed
- **THEN** the server listens on port 7070

### Requirement: Automatic database migrations
When `serve` or any command that accesses the database starts, the system SHALL create the SQLite file if it does not exist and apply pending migrations in order, recording them in `schema_migrations`. The `migrate` command SHALL apply the migrations and exit. The database MUST use `journal_mode=WAL`.

#### Scenario: First run
- **WHEN** the database path does not exist
- **THEN** the file is created and all migrations are applied

#### Scenario: Idempotent migrations
- **WHEN** `dashboard migrate` is executed twice
- **THEN** the second run does not change the schema and finishes successfully

### Requirement: Per-connection SQLite pragmas
Every pool connection to SQLite SHALL have `foreign_keys` enabled and `busy_timeout` configured.

#### Scenario: Pragmas on every connection
- **WHEN** several pool connections are opened simultaneously
- **THEN** `PRAGMA foreign_keys` returns `1` on each of them

### Requirement: Admin authentication
Web administration SHALL require login with the administrator password, obtained with the precedence `DASHBOARD_ADMIN_PASSWORD_HASH` (bcrypt) > `DASHBOARD_ADMIN_PASSWORD` (plain text) > bcrypt hash stored by `dashboard admin set-password`. With no source configured, web administration MUST be disabled. Login MUST use a generic error message and MUST be limited, by default, to 5 attempts per minute per IP (configurable via `login_rate`/`DASHBOARD_LOGIN_RATE`/`--login-rate`, between 1 and 1000, with a global limit of 4x that value), responding HTTP 429 above the limit.

#### Scenario: Wrong password
- **WHEN** a login is attempted with an incorrect password
- **THEN** the system responds with a generic error and does not create a session

#### Scenario: Env hash overrides stored hash
- **WHEN** `DASHBOARD_ADMIN_PASSWORD_HASH` is set and another hash is stored in the database
- **THEN** only the password matching the environment hash is accepted

#### Scenario: Login rate limit
- **WHEN** the same IP makes its 6th login attempt within one minute
- **THEN** the system responds HTTP 429 without checking the password

### Requirement: Server-side sessions
Sessions SHALL be stored on the server. The cookie MUST contain only a random 32-byte identifier, with `HttpOnly`, `SameSite=Strict` and `Secure` when served over HTTPS (directly, via a trusted proxy or with `--cookie-secure`); the database MUST store only the SHA-256 hash of the identifier. Sessions MUST expire after 2 hours of inactivity or 12 hours since creation, validated on the server. Logout and password change MUST revoke sessions. The identifier MUST be new on every login.

#### Scenario: Idle expiration
- **WHEN** a session goes 2 hours without requests and is used again
- **THEN** the system responds HTTP 401 and the session is removed

#### Scenario: Logout revokes session
- **WHEN** the administrator logs out and the old cookie is sent again
- **THEN** the system responds HTTP 401

#### Scenario: Password change revokes sessions
- **WHEN** `dashboard admin set-password` is executed
- **THEN** all existing sessions are no longer valid

#### Scenario: Password change with server running
- **WHEN** the server is running without password variables and `dashboard admin set-password` sets a new password
- **THEN** the next login accepts the new password and rejects the old one, without restarting the server

#### Scenario: Set password while env overrides
- **WHEN** `DASHBOARD_ADMIN_PASSWORD_HASH` is set and `dashboard admin set-password` is executed
- **THEN** the command stores the new hash and prints a warning that it will have no effect while the environment variable exists

#### Scenario: Secure cookie under HTTPS
- **WHEN** login is done over HTTPS
- **THEN** the session cookie is issued with the `Secure` attribute

### Requirement: CSRF protection
Each session SHALL have a random CSRF token stored on the server and delivered by `GET /api/admin/session`, which is public and MUST respond `{"admin_enabled": bool, "authenticated": bool, "csrf_token": string|null}`. `POST`, `PUT` and `DELETE` requests to `/api/admin/*`, except login, MUST send `X-CSRF-Token` equal to the session token, compared in constant time. Login MUST reject requests whose `Origin` is not the dashboard's origin.

#### Scenario: Session status when admin disabled
- **WHEN** no administrator password is configured and `GET /api/admin/session` is called
- **THEN** the response is `{"admin_enabled": false, "authenticated": false, "csrf_token": null}`

#### Scenario: Session status without login
- **WHEN** a password is configured and `GET /api/admin/session` is called without a session cookie
- **THEN** the response is `{"admin_enabled": true, "authenticated": false, "csrf_token": null}`

#### Scenario: Missing CSRF token
- **WHEN** an authenticated `DELETE /api/admin/items/{id}` request is sent without `X-CSRF-Token`
- **THEN** the system responds HTTP 403 and the item is not removed

#### Scenario: Token from another session
- **WHEN** an authenticated request sends the CSRF token of another valid session
- **THEN** the system responds HTTP 403

#### Scenario: Cross-origin login
- **WHEN** `POST /api/admin/login` is received with `Origin: https://evil.example`
- **THEN** the system responds HTTP 403 without checking the password

### Requirement: Security headers
The server SHALL send on every HTML response the headers `X-Frame-Options: SAMEORIGIN`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin` and the `Content-Security-Policy` `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'; frame-src <configurable>`, with `frame-src` defaulting to `http: https:`. Under HTTPS it SHALL also send `Strict-Transport-Security`. The interface MUST NOT use inline scripts. `/api/admin/*` routes MUST limit the body to 1 MiB.

#### Scenario: Headers present
- **WHEN** `GET /` is requested
- **THEN** the response contains the configured security headers, with `script-src 'self'` and `object-src 'none'`

#### Scenario: No CSP violations
- **WHEN** the main page and the administration are loaded in a test browser
- **THEN** no CSP violation is logged to the console

### Requirement: Health check and graceful shutdown
The server SHALL expose `GET /healthz` returning HTTP 200 when the database responds, and SHALL shut down gracefully on SIGINT/SIGTERM, waiting up to 10 seconds for in-flight requests; after that deadline, the remaining connections MUST be forcibly closed and the process MUST exit logging a warning.

#### Scenario: Healthy
- **WHEN** `GET /healthz` is called with the database reachable
- **THEN** the response is HTTP 200 with `{"status":"ok"}`

#### Scenario: Graceful stop
- **WHEN** the process receives SIGTERM
- **THEN** the server stops accepting connections, completes the in-flight ones and closes the database before exiting

#### Scenario: Shutdown timeout exceeded
- **WHEN** a request is still in flight 10 seconds after SIGTERM
- **THEN** the connection is closed, a warning is logged and the process exits

### Requirement: Consistent backup
The binary SHALL provide `dashboard backup -o <file>`, which produces a consistent copy of the database using `VACUUM INTO`, even with the server running and transactions still in the WAL.

#### Scenario: Backup while serving
- **WHEN** an item is created through the API and then `dashboard backup -o b.db` is executed with the server running
- **THEN** `b.db` is a sound SQLite database (`PRAGMA integrity_check` = `ok`) containing the created item

#### Scenario: Restore
- **WHEN** the service is stopped, `dashboard.db` is replaced by `b.db` and the `-wal`/`-shm` files are removed
- **THEN** the service restarts and shows the same menu tree as at the time of the backup

### Requirement: Initial menu seed
The `serve` command SHALL accept `--seed <file>` (`DASHBOARD_SEED`), which imports a menu export document at startup only when no item exists.

#### Scenario: Seed on first start
- **WHEN** the server starts with `--seed examples/menu.json` and the database has no items
- **THEN** the items in the file are imported and the log records the imported count

#### Scenario: Seed is ignored afterwards
- **WHEN** the server restarts with the same `--seed` and items already exist
- **THEN** nothing is imported

### Requirement: Development mode
The `serve` command SHALL accept `--dev`, which serves the assets from the `web/dist` directory on disk instead of the embedded ones. The `frontend/` project SHALL be developed with Vite+ (`vp dev`), whose development server proxies `/api` and `/healthz` to the Go server.

#### Scenario: Rebuilt assets in dev mode
- **WHEN** the server runs with `--dev` and `vp build` regenerates `web/dist`
- **THEN** the next request returns the new assets without recompiling the binary

#### Scenario: Frontend dev server
- **WHEN** the Go server runs on `:8080` and `vp dev` runs on `:9000`
- **THEN** the interface at `http://localhost:9000` loads the menu through the `/api/menu` proxy and reflects changes to Vue components without reloading the page

### Requirement: SPA routing
The server SHALL respond with the SPA's `index.html` for `/`, `/admin` and any path under `/admin/` that is neither a static file nor an API route, and MUST respond with a JSON 404 for nonexistent paths under `/api/`.

#### Scenario: Direct admin URL
- **WHEN** the browser directly opens `/admin/login`
- **THEN** the server responds HTTP 200 with `index.html` and the SPA shows the login screen

#### Scenario: Unknown API path
- **WHEN** `GET /api/nonexistent` is requested
- **THEN** the server responds HTTP 404 with a JSON body

### Requirement: Static asset caching
Embedded static assets SHALL be served with `ETag` and with gzip compression when the client accepts it. Files with a hash in the name (produced by the build) MUST use `Cache-Control: public, max-age=31536000, immutable`; `index.html` MUST use `Cache-Control: no-cache`.

#### Scenario: Conditional request
- **WHEN** the browser requests an asset from `web/dist/assets/` with `If-None-Match` equal to the current ETag
- **THEN** the server responds HTTP 304 without a body

