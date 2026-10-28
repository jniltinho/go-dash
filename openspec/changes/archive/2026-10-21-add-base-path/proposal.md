## Why

The binary assumes it is served at the root of a host: assets, API calls,
client routes, redirects and cookies all start with `/`. Behind nginx we often
need to publish it under a sub-path (e.g. `https://intranet.example.com/panel/`)
next to other applications on the same host, which is impossible today without
rewriting HTML and responses in the proxy.

## What Changes

- New configuration key **`base_path`** (flag `--base-path`, env
  `DASHBOARD_BASE_PATH`, TOML `base_path`), default `/`. Examples: `/`,
  `/panel/`, `/ops/dash/`.
- The server works under that prefix whether nginx **keeps** it
  (`proxy_pass http://127.0.0.1:8080;`) or **strips** it
  (`proxy_pass http://127.0.0.1:8080/;`): it removes a leading `base_path`
  from the request path when present.
- Every URL the server emits is prefixed: SPA `index.html` (a `<base href>`
  injected at serve time), redirects (`/dash/...` canonical routes,
  `/dash/default` → base), and cookie `Path` (admin and viewer sessions).
- The frontend is built once with relative asset URLs and reads the base from
  the document (`<base href>`): vue-router history base, API client prefix,
  links and the favicon all follow it. No rebuild per base path.
- Per-viewer `localStorage` keys are namespaced by base path when it is not
  `/`, so two instances on the same host do not share theme or card order.
- Docs: nginx examples for both proxy styles, `public_url` consistency, and
  the new key in `dashboard.toml.example` and the configuration table.

No behaviour changes with the default `base_path = "/"`.

## Capabilities

### New Capabilities
- `base-path`: serving the whole application (SPA, API, dashboards, admin,
  health check, cookies, redirects) under a configurable path prefix behind a
  reverse proxy.

### Modified Capabilities
<!-- The add-link-dashboard and add-multi-dashboards specs are not archived
     into openspec/specs/ yet; the changed behaviour is written as ADDED
     requirements of the new capability. -->

## Impact

- **Go**: `internal/config` (key, validation, normalization),
  `internal/server` (pre-routing prefix strip, cookie path, redirects,
  `index.html` base injection and its ETag), `cmd/serve.go` (flag).
- **Frontend**: `vite.config.ts` (`base: './'`), `index.html` (relative
  favicon), new `src/lib/basePath.ts`, `router.ts`, `api/client.ts`,
  `lib/theme.ts` and `lib/cardOrder.ts` storage keys, e2e helpers.
- **Deploy/docs**: `dashboard.toml.example`, `deploy/systemd/go-dash.env.example`,
  `docs/README.md` (configuration table), `docs/install.md` (nginx section C),
  `docs/architecture.md`.
- **Tests**: Go config and handler tests for both proxy styles, Vitest for the
  base-path helpers, a Playwright run against a binary started with
  `--base-path /panel/`.
