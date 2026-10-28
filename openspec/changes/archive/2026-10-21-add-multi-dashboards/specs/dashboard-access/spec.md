## ADDED Requirements

### Requirement: Optional dashboard password
Each dashboard, including the default one, SHALL have an optional access
password, independent from the admin password and from other dashboards. It
SHALL be stored only as a bcrypt hash, SHALL have 8 to 72 bytes (same rule in
API and CLI), and
SHALL never be returned by any API or CLI output. A dashboard without a
password SHALL be public.

#### Scenario: Public dashboard
- **WHEN** `acme` has no password
- **THEN** `GET /api/dash/acme/menu` returns its tree without any cookie

#### Scenario: Short password
- **WHEN** an admin sets a 5-character password on `initech`
- **THEN** the request is rejected with HTTP 400 and `field` = `password`

#### Scenario: Admin listing hides hashes
- **WHEN** an admin lists dashboards
- **THEN** each entry has `protected: true|false` and no hash or password field

### Requirement: Protected dashboard content
For a protected dashboard, the menu endpoint SHALL return HTTP 401
`{"error":"dashboard locked"}` unless the request carries a valid viewer
session for that dashboard or a valid admin session. Menu responses of a
protected dashboard SHALL be full responses with
`Cache-Control: private, no-store`, without `ETag` and never 304, regardless
of `If-None-Match`. Status, unlock, lock and all their error responses SHALL
use `Cache-Control: no-store`. The status endpoint
`GET /api/dash/:slug/status` (and `GET /api/dash/default/status` for the
default dashboard) SHALL return `{slug, title, protected, unlocked}` without
any menu content. Titles of protected dashboards SHALL be visible in the
status response.

#### Scenario: Locked menu
- **WHEN** `initech` is protected and a browser without a session calls `GET /api/dash/initech/menu`
- **THEN** it receives 401 `{"error":"dashboard locked"}` and no item data

#### Scenario: No conditional reuse of protected menus
- **WHEN** an unlocked viewer requests a protected menu with `If-None-Match: *` or a previous ETag
- **THEN** the response is a full 200 with `Cache-Control: private, no-store` and no `ETag`

#### Scenario: Dashboard becomes protected
- **WHEN** a viewer polls the public `/api/menu` with a cached ETag and the admin then sets a password on the default dashboard
- **THEN** the next poll returns 401 `dashboard locked` (not 304) on both `/api/menu` and `/api/dash/default/menu`

#### Scenario: Locked UI forgets the menu
- **WHEN** a viewer has protected-dashboard tabs open and the next poll returns 401 `dashboard locked`
- **THEN** all tabs, iframe panes and the cached tree are discarded before the unlock screen is shown

#### Scenario: Switching dashboards drops stale responses
- **WHEN** a viewer navigates from `/dash/initech` to `/dash/acme` while a menu request for `initech` is still pending
- **THEN** the late `initech` response is ignored and never rendered on `acme`

#### Scenario: Protected default dashboard
- **WHEN** the default dashboard is protected and a browser calls `GET /api/menu` without a session
- **THEN** it receives 401 `{"error":"dashboard locked"}`

### Requirement: Unlock flow
A protected dashboard SHALL show an unlock screen (title, password
field, "Unlock" button) instead of the menu. `POST /api/dash/:slug/unlock`
with `{"password"}` SHALL verify the password and, on success, create a
viewer session for that dashboard only and set an `HttpOnly`,
`SameSite=Strict` cookie (`Secure` under HTTPS) named
`dashboard_view_<dashboard id>`. A wrong password SHALL return 401
`{"error":"invalid password"}`. Unlock requests SHALL be rejected when
cross-origin, like admin login. `POST /api/dash/:slug/lock` SHALL revoke the
viewer session and clear the cookie.

#### Scenario: Successful unlock
- **WHEN** a viewer submits the correct password on `/dash/initech`
- **THEN** the menu loads and reloading the page does not ask for the password again

#### Scenario: Wrong password
- **WHEN** a viewer submits a wrong password
- **THEN** the unlock screen shows "Incorrect password." and no cookie is set

#### Scenario: Session is scoped to one dashboard
- **WHEN** a viewer unlocked `initech` and opens the protected `globex`
- **THEN** `globex` asks for its own password

#### Scenario: Viewer session is not an admin session
- **WHEN** a request to an authenticated admin route (items, dashboards, export, import, logout) carries only a viewer cookie
- **THEN** it receives 401 `authentication required`

#### Scenario: Viewer token in the admin cookie
- **WHEN** the raw token of a viewer session is sent as `dashboard_session` to `GET /api/admin/session` or any admin route
- **THEN** the session status reports `authenticated: false` without a CSRF token, and admin routes return 401

#### Scenario: Token in the wrong viewer cookie
- **WHEN** an admin token, or a viewer token of `globex`, is sent as the viewer cookie of `initech`
- **THEN** `GET /api/dash/initech/menu` returns 401 `dashboard locked` (unless a valid admin cookie is also present)

#### Scenario: Oversized unlock body
- **WHEN** a client posts an unlock body larger than 4 KiB
- **THEN** it receives 413 and no session is created

#### Scenario: Unlock racing a password change
- **WHEN** an unlock request verifies the old password and the admin changes the password before the session is stored
- **THEN** no viewer session is created and the request returns 401 `invalid password`

### Requirement: Unlock rate limit
Unlock attempts SHALL be rate limited per client IP (aggregate across all
dashboards) and globally, using the same limits as admin login
(`login_rate` per IP, four times that globally) but a limiter instance
separate from admin login. The client IP SHALL be derived as for login
(honouring `trust_proxy`). Exceeding the limit SHALL return HTTP 429.

#### Scenario: Brute force
- **WHEN** a client exceeds `login_rate` unlock attempts per minute on `initech`
- **THEN** further attempts return 429 until the window passes, even with the right password

#### Scenario: Cycling dashboards
- **WHEN** a client spreads more than `login_rate` attempts per minute across `initech`, `globex` and `default`
- **THEN** attempts beyond `login_rate` return 429

#### Scenario: Unlock spam does not block admin login
- **WHEN** a client is rate limited on unlock
- **THEN** admin login from the same IP is still governed only by its own limiter

### Requirement: Viewer session lifetime and revocation
Viewer sessions SHALL be stored server-side (only the id hash), SHALL use the
same idle and absolute timeouts as admin sessions and SHALL be purged with
them. Setting, changing or clearing a dashboard's password, or deleting the
dashboard, SHALL revoke all viewer sessions of that dashboard and no others.
Changing the admin password SHALL NOT revoke viewer sessions.

#### Scenario: Password change revokes viewers
- **WHEN** an admin changes the password of `initech`
- **THEN** viewers of `initech` must unlock again, while viewers of `globex` stay unlocked

#### Scenario: Clearing password
- **WHEN** an admin clears the password of `initech`
- **THEN** `/dash/initech` is public again and its viewer sessions are removed

### Requirement: Admin bypass
A request with a valid admin session SHALL be allowed to read every
dashboard's menu without the dashboard password; the status response SHALL
then report `unlocked: true`.

#### Scenario: Admin previews protected dashboard
- **WHEN** a logged-in admin opens `/dash/initech`
- **THEN** the menu loads without the unlock screen

### Requirement: Password management
Admins SHALL set or clear a dashboard password via
`PUT /api/admin/dashboards/:id/password` (`{"password"}`) and
`DELETE /api/admin/dashboards/:id/password`, behind session and CSRF checks,
and via the CLI `dashboard dash set-password SLUG [--stdin]` (interactive
prompt without echo by default) and `dashboard dash clear-password SLUG`; the
default dashboard is addressed as `default` in the CLI. The Admin UI SHALL
show whether each dashboard is protected and offer "Set password" and
"Remove password" actions with confirmation for removal.

#### Scenario: Set password from CLI on the default dashboard
- **WHEN** an operator runs `dashboard dash set-password default --stdin` with a valid password
- **THEN** `/` becomes protected, also for a server that is already running

#### Scenario: Clear password via API without CSRF
- **WHEN** an authenticated admin sends `DELETE /api/admin/dashboards/3/password` without `X-CSRF-Token`
- **THEN** it receives 403 and the password stays set
