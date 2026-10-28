## Context

Today every URL assumes the root: Vite emits `/assets/...`, `index.html`
links `/favicon.svg`, the Vue router uses `createWebHistory()` without a base,
the API client calls `/api/...`, redirects in `internal/server/static.go` use
`/` and `/dash/...`, and cookies are set with `Path=/`. Published behind nginx
at `/panel/`, the browser would ask `/assets/...` and `/api/...` from the host
root and fail. The binary must stay a single, self-contained build
(`CGO_ENABLED=0`, frontend embedded), so the base path cannot be baked into
the frontend at build time.

## Goals / Non-Goals

**Goals:**
- Serve the whole application under one configurable prefix, set at runtime.
- Work with both nginx styles (prefix kept or stripped) without extra proxy
  rewrites (`sub_filter`, `proxy_redirect`, cookie path rewriting).
- Keep the default (`/`) byte-for-byte compatible, except the `<base href="/">`
  tag that is now always injected.

**Non-Goals:**
- Serving several base paths from one process.
- Deriving the prefix from `X-Forwarded-Prefix` per request (spoofable,
  inconsistent between proxies); the prefix is configuration.
- Changing iframe targets or menu item URLs (those are external).

## Decisions

### 1. Configuration
`base_path` joins `config.Config`, `SetDefaults` (`"/"`), the `serve` flags
(`--base-path`), `dashboard.toml.example`, `go-dash.env.example` and the
README table. `Config.Validate()` normalizes it (adds the leading/trailing
slash) and checks the rules in the spec; normalization happens once in
`config.Load`, so the rest of the code reads `cfg.BasePath` always ending
with `/`. If `public_url` has a non-root path, it must equal `base_path`.
Reserved first segments (`api`, `admin`, `assets`, `dash`, `healthz`,
`index.html` and every file in the root of the embedded build, read from the
assets FS at start) avoid the ambiguity of decision 2: with base `/admin/` a
stripped request `/admin/login` could not be told apart from a kept one, and
with base `/favicon.svg/` the icon itself would be redirected. Segments `.`
and `..` are rejected because browsers normalize them, which would make the
emitted URLs and cookie paths differ from the configuration. `%` is rejected
so the configured value equals its escaped form.

### 2. Prefix handling on the server: strip when present
An `e.Pre` middleware (runs before routing) removes the prefix only on a
segment boundary: the path equals `base_path` without its trailing slash
(→ redirect, below) or starts with the full `base_path` (→ the rest, with a
leading `/`). `/panel-x/...` is left alone. Because `base_path` has no
encoded characters, its escaped form is identical; `RawPath`, when set, is
checked for the same prefix and rewritten with it, and if the two do not both
carry the prefix the request is left unchanged (it then 404s). Tests cover
sibling prefixes, `%2F` after the prefix, encoded prefix characters
(`/pai%6Eel/` is not the prefix) and nested bases (`/ops/dash/`). Other paths
pass unchanged. All routes stay registered at the root, so the
router, handlers, `isSPARoute` and tests do not change. `/panel` exactly is
redirected to `/panel/`. *Alternative considered:* mounting every route under
an Echo group for the prefix — rejected, it breaks the "prefix stripped" nginx
style and duplicates route registration.

### 3. URLs emitted by the server
A helper `s.url(p string) string` joins `BasePath` and a root-relative path
(`s.url("/dash/acme")` → `/panel/dash/acme`). It is used by:
- `staticHandler` redirects (it gets the base path in its constructor);
- `setCookie` (`Path: cfg.BasePath`);
- the `index.html` injection (decision 4).
`Location` values stay path-only (no scheme/host), so they are correct
behind any proxy.

### 4. `index.html` and assets
Vite builds with `base: './'` (build only; the dev server keeps `/`), so the
emitted HTML, CSS `url()`s and dynamic imports are relative. The static
handler injects `<base href="{BasePath}">` right after `<head>` when it loads
`index.html` — **always**, also for `/`, because with relative assets a deep
route like `/dash/acme` would otherwise resolve `./assets/...` to
`/dash/assets/...`. The injected file is cached and its ETag is computed on
the injected content. `index.html` gets `href="favicon.svg"` (relative).
`base_path` is validated to safe characters, so it needs no HTML escaping,
but it is escaped anyway (`html.EscapeString`). CSP already has
`base-uri 'self'`, which allows a same-origin `<base>`.

### 5. Frontend
`src/lib/basePath.ts` (pure, unit-tested) exposes `basePath()`, `apiUrl(path)`
and `storageKey(name)`. `basePath()` reads `document.querySelector('base[href]')`
and resolves it against the document; with no `<base>` element (Vite dev
server) it returns `/` — never `document.baseURI`, which would then be the
current route:
- `router.ts`: `createWebHistory(basePath())`; route paths stay `/...`.
- `api/client.ts`: every request goes through `apiUrl('/api/...')`.
- `lib/theme.ts`, `lib/cardOrder.ts`: keys via `storageKey()`; unchanged for
  `/`, `${basePath}` + key otherwise (e.g. `/panel/dashboard.theme`). The
  legacy card-order migration runs only when the base path is `/`.
- `components/HomePane.vue` links to the Admin with `href="/admin"` (empty
  menu screen); it becomes `<RouterLink to="/admin">`. A unit test scans the
  `.vue` sources for `href="/` and `href="#` so new root-absolute or bare hash
  links fail the build.
- Links that use `router.push`/`RouterLink` are already relative to the
  router base. `DashboardView.syncHash` keeps using `location.pathname`, which
  already contains the prefix.
- Risk with `<base>`: plain `href="#..."` anchors would resolve against the
  base. The app uses none today (hash changes are programmatic); a unit/e2e
  check guards it.

### 6. Tests
- Go: config normalization/validation table; handler tests running the
  same requests with prefix kept and stripped; redirects and cookie paths
  under `/panel/`; `/healthz` with and without prefix; ETag of the injected
  `index.html`.
- Vitest: `basePath`, `apiUrl`, `storageKey` with and without `<base>`.
- Playwright: a binary started with `--base-path /panel/`, reached in two
  ways — directly (prefix kept) and through a tiny stripping proxy written
  with `node:http` in `e2e/` (no new dependency), which forwards
  `/panel/...` as `/...` like `proxy_pass http://upstream/;`. The same spec
  runs on both: deep links, Admin ↔ Dashboard, the icon picker (lazy chunk and
  MDI font), unlock and cookie path, hash and back navigation, a conditional
  `index.html` request, no CSP violation and no request outside `/panel/`.

## Risks / Trade-offs

- [The injected `<base>` changes `index.html` for `/` too] → content and ETag
  differ once after the upgrade; browsers revalidate (`no-cache`).
- [A proxy that both strips and adds another prefix] → not supported; the
  docs show the two supported nginx forms.
- [Strip-when-present accepts both forms] → a request to `/api/...` still
  works when the proxy keeps the prefix; harmless, the same handlers answer.
- [Third-party code building absolute `/` URLs] → DHTMLX loads no files at
  runtime (icons are vendored CSS); verified by the "no request outside the
  base path" e2e check.

## Migration Plan

Nothing to migrate: the default stays `/`. To publish under a sub-path, set
`DASHBOARD_BASE_PATH=/panel/` (and `public_url` with the same path, if used),
restart, and add one of the nginx `location /panel/` blocks from
`docs/install.md`. Rollback: unset the key and restart.

## Open Questions

- Should `/` on the root host redirect to the base path when someone reaches
  the backend directly? Not needed behind nginx; left out.
