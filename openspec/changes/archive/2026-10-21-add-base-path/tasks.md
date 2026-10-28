## 1. Configuration

- [x] 1.1 Add `base_path` to `config.Config`, `SetDefaults` (`/`), the `serve` flag `--base-path` and `DASHBOARD_BASE_PATH`
- [x] 1.2 Normalize (leading/trailing slash) and validate in `config.Load`/`Validate` (segment charset, no `.`/`..`/`//`/`%`, max 100, reserved first segments incl. root build files, `public_url` path must match)
- [x] 1.3 Table-driven config tests for normalization and every rejection

## 2. Server

- [x] 2.1 `e.Pre` middleware stripping the base path on a segment boundary only, rewriting `Path`/`RawPath` consistently; redirect `/panel` → `/panel/` keeping the query
- [x] 2.2 `s.url()` helper; prefix `staticHandler` redirects (`/dash`, `/dash/default`, canonical slugs) and cookie `Path` (admin and viewer)
- [x] 2.3 Inject `<base href>` into `index.html` (always, escaped), cache it, ETag over the injected content
- [x] 2.4 Handler tests for `/` and `/panel/` with the prefix kept and stripped: SPA, API, redirects, cookie paths, `/healthz`, ETag; sibling prefix, `%2F` after the prefix, encoded prefix, nested base

## 3. Frontend

- [x] 3.1 Vite `base: './'` for builds only; `index.html` favicon relative
- [x] 3.2 `src/lib/basePath.ts` (`basePath` from `base[href]`, `/` without it; `apiUrl`, `storageKey`) with Vitest incl. no-`<base>` deep route
- [x] 3.3 Router history base, API client through `apiUrl`, theme and card order keys through `storageKey`; legacy card-order migration only at base `/`
- [x] 3.4 Replace `href="/admin"` in `HomePane.vue` with `RouterLink`; Vitest scan of `.vue` sources rejecting `href="/` and `href="#`

## 4. E2E

- [x] 4.1 Playwright web servers: binary with `--base-path /panel/` and a `node:http` stripping proxy in front of it (`e2e/strip-proxy.mjs`)
- [x] 4.2 Spec run on both (kept and stripped): deep links, Admin ↔ Dashboard, empty-menu Admin link, icon picker (lazy chunk, MDI font), unlock and cookie path, hash/back navigation, conditional `index.html`, no CSP violation, no request outside `/panel/`
- [x] 4.3 Existing suite still passes at `/`

## 5. Docs and deploy

- [x] 5.1 `dashboard.toml.example`, `deploy/systemd/go-dash.env.example`, README configuration table
- [x] 5.2 `docs/install.md` section C: nginx blocks for prefix kept and prefix stripped, `public_url` and `trust_proxy` notes
- [x] 5.3 `docs/architecture.md` (request flow under a prefix) and `AGENTS.md` (no absolute `/` URLs in the frontend; use `apiUrl`/router)
- [x] 5.4 Run `make fmt && make lint && make test && make e2e`
