# AGENTS.md

Guidance for AI coding agents (Claude Code, Codex, …) and humans working on
this repository. Read it fully before changing code.

## What this project is

A **link aggregator dashboard**: a header toolbar with menus and nested
submenus; each link opens either in a **tab of the central area (sandboxed
iframe)** or in a **new browser tab**. Delivered as **one Go binary** with the
frontend embedded and a local **SQLite** database.

- Specs (source of truth for behaviour): `openspec/specs/` (one folder per
  capability); finished changes, with their proposal, design and tasks, live in
  `openspec/changes/archive/`.
  (`proposal.md`, `design.md`, `specs/*/spec.md`, `tasks.md`).
- Architecture and diagrams: `docs/architecture.md`.

## Non-negotiable premises

1. **All code and the UI are in English**: identifiers, comments, commit
   messages, log messages, API error messages, test names and every
   user-visible string in Vue/DHTMLX. API errors are mapped to friendly
   messages in `frontend/src/lib/messages.ts` — add an entry when you add an
   API error.
2. **Single self-contained binary**: `CGO_ENABLED=0`, frontend embedded via
   `//go:embed all:web/dist`. No runtime dependency on Node, CDNs or the
   internet. Never load scripts, styles or fonts from external hosts.
3. **Security baseline must not regress**: CSP (`script-src 'self'`, no inline
   scripts, no `eval`), iframe sandbox without `allow-top-navigation`,
   server-side sessions, CSRF token on mutating admin requests, bcrypt, login
   rate limit, `http/https`-only URLs, same-origin URLs never in iframes,
   titles rendered as text (never DHTMLX `html` fields with user data).
4. **No rounded corners** in any theme. **WCAG AAA**: every text/background
   pair of every theme (including 3D gradient stops) is ≥ 7:1, icons ≥ 4.5:1,
   focus ring/scrollbar/accent ≥ 3:1 — enforced by `src/lib/contrast.ts` tests.
5. **DHTMLX vendor files are never edited**. Theme through CSS variables and
   wrapper classes (`frontend/src/theme/theme.css`). The CDN `@import` in
   `suite.min.css` is stripped by a Vite plugin; icons live in
   `frontend/src/vendor/dhx-icons`.
6. **Spec-driven changes**: behaviour changes start with an OpenSpec change
   (`/opsx:propose`), then implementation (`/opsx:apply`). Keep `tasks.md`
   checkboxes accurate.

## Stack and versions

| Area | Choice |
| --- | --- |
| Go | 1.26, module `github.com/jniltinho/go-dash` |
| CLI / config | `spf13/cobra`, `spf13/viper` — **TOML** file + `DASHBOARD_*` env |
| HTTP | **Echo v5** (`*echo.Context`, `log/slog`, `echo.StartConfig`) |
| Database | SQLite via `modernc.org/sqlite` + `database/sql` (no ORM) |
| Logging | `log/slog` (text or JSON) |
| Frontend | Vue 3 + TypeScript, Pinia, vue-router (history mode) |
| Toolchain | **Vite+** (`vite-plus`, CLI `vp`: dev/check/test/build) + `vue-tsc` |
| UI widgets | DHTMLX Suite 9 (`@dhtmlx/suite`, GPL edition) |
| Styles | Tailwind CSS v4 **without preflight** + `theme.css` |
| Tests | `go test`, Vitest (`vp test`), Playwright e2e |

## Layout

```
main.go                 embed web/dist, call cmd.Execute
cmd/                    Cobra commands: serve, migrate, menu, dash, admin, backup, version
internal/config         viper: flags > env > TOML > defaults; Validate()
internal/database       Open (DSN pragmas), embedded migrations, Backup (VACUUM INTO)
internal/menu           domain: dashboards, validation, tree, cycles, reorder, import/export
internal/auth           password sources, dashboard passwords, admin/viewer sessions, CSRF, rate limiter
internal/server         Echo app, middleware, handlers, error mapping, SPA/static
frontend/src/api        typed fetch client (sends X-CSRF-Token)
frontend/src/lib        pure, unit-tested logic (toolbar mapping, tabs LRU, theme, messages, contrast)
frontend/src/stores     Pinia stores (config, menu, auth, theme)
frontend/src/views      DashboardPage (+DashboardView), UnlockView, AdminView, LoginView, AdminDisabledView
frontend/src/theme      Tailwind entry + themes (blue-black / xp, flat / 3d)
frontend/e2e            Playwright suite run against the real binary
web/dist                build output (only .gitkeep is versioned)
deploy/                 install.sh, systemd unit + env example (/opt/go-dash), release docker-compose.yml
docs/                   README (guide), install.md (Linux server), architecture and diagrams
openspec/               specs/ (current behaviour), changes/ (open), changes/archive/ (done)
```

## Commands

```bash
make build          # frontend (vp check + vue-tsc + vp build) then Go binary -> bin/go-dash
make test           # go test ./... + vp test
make lint           # go vet, gofmt check, vp check
make fmt            # gofmt + vp check --fix
make e2e            # Playwright against bin/go-dash (PW_CHROME_PATH=/path/to/chrome if needed)
make screenshots    # regenerate docs/screenshots (docs/README.md gallery) after visual changes
make dev            # Go server reading web/dist from disk (:8080)
make dev-frontend   # Vite+ dev server :9000, proxies /api to :8080
make docker-up      # docker compose up -d --build (port 8080, volume /data)
make dist           # dist/: binary, release tar.gz (install.sh + systemd unit), Docker image tar.gz (never committed)
make dist-server    # dist/: ready server package (SERVER_ADDR, ADMIN_PASSWORD hashed, MENU) with `sudo make install`
```

Before finishing any change run: `make fmt && make lint && make test`, and
`make e2e` when UI or HTTP behaviour changed. Formatting failures break
`make build` on purpose.

## Go conventions

Follow the project skills in `.claude/skills/golang-*` (also in
`.codex/skills`). In particular:

- Early returns, no `else` after `return`; wrap errors with `%w` and context
  (`fmt.Errorf("reading seed %q: %w", path, err)`); handle each error once.
- `RunE` only; write output via `cmd.OutOrStdout()`/`ErrOrStderr()`; validate
  args with `cobra.ExactArgs`/`NoArgs`; build a fresh command tree per test.
- Every config key is a flag (dash-case) bound to viper (snake_case) and to
  `DASHBOARD_<KEY>`; add new keys to `config.SetDefaults`, `Config`,
  `dashboard.toml.example` and docs/README.md.
- SQL: parameterized queries only; multi-step writes in a transaction;
  migrations are new files `internal/database/migrations/NNNN_name.sql`
  (never edit an applied one).
- Domain validation returns `*menu.ValidationError{Field, Message}` (mapped to
  HTTP 400 `{"error","field"}`); not found → `menu.ErrNotFound` (404).
- Initialize slices/maps (`[]T{}`) so JSON never returns `null` for lists.
- Tests: table-driven where useful, temp databases via `t.TempDir()`,
  `httptest` for handlers. Keep `go vet` clean and code `gofmt`-ed.

## Frontend conventions

- DHTMLX widgets are created in `onMounted` and registered with
  `useDhx().own(...)` so they are destroyed on unmount; pass `DETACHED` as
  container for widgets attached to Layout cells/Windows.
- UI labels: the home tab and button are **"Home"**, the admin area is **"Admin"**.
- The initial theme is the viewer's saved choice, else the server `default_theme`
  (`/api/config`, applied in `main.ts` before mounting), else `DEFAULT_THEME`.
- Toolbar item IDs: `item-<id>`, `open-<id>`, `action-home`, `action-admin`,
  `action-lock`, `theme-*`, `style-*` (Admin: `dash-<id>`, `dash-manage`).
  Tab IDs: `home`, `tab-<id>`.
- Dashboards: `/` is the default one (API/CLI sentinel `default`), named ones
  are `/dash/<slug>`. `DashboardPage` resolves the status (unlock / not
  found) and mounts `DashboardView` keyed by slug; the menu store is shared, so
  every request is tied to its slug and `reset()` runs on switch/lock/401/404.
  Per-dashboard `localStorage` keys use the slug (`dashboard.cardOrder.<slug>`).
- Iframe panes are Vue elements teleported into the Layout content cell and
  toggled with `v-show` — never re-create them on tab switch.
- Put logic in `src/lib` as pure functions with Vitest tests; keep views thin.
- **Themes are data**: one file per theme in `frontend/src/theme/registry/`
  (colors, font, 3D gradients, optional bevel/glow), registered in
  `registry/index.ts`. `applyTheme` turns tokens into `--c-*`/`--g-*` CSS
  variables; `theme.css` must never contain per-theme rules. A new theme only
  needs a registry entry (and an e2e group mapping in `e2e/helpers.ts`).
- Sizes scale with `--ui-scale` (1 / 1.25 at ≥2300px / 1.6 at ≥3400px);
  pixel values configured in JS (Tabbar, Layout columns) use `uiScale()` and
  `onScaleChange()` from `src/lib/scale.ts`.
- Use semantic classes from `theme.css` (`btn`, `card`, `context-bar`,
  `link-item`, `field`, `panel-title`) so every theme/style applies; Tailwind
  utilities for layout/spacing.
- **Base path**: the app may be served under a prefix (`base_path`, e.g.
  `/painel/`). Never write root-absolute URLs in the frontend (`href="/..."`,
  `fetch('/api/...')`): use `apiUrl()` from `src/lib/basePath.ts` for requests
  and the router (`RouterLink`, `router.push`) for links; per-viewer
  `localStorage` keys go through `storageKey()`. A Vitest scan rejects
  `href="/` and `href="#` in `.vue` files. Server redirects use `joinBase()`.
- **Modals** always open 10% below the top: Vue modals use
  `components/AppModal.vue` (backdrop, focus trap, Escape, inert app) with
  styles in `theme/modal.css` (`--modal-top`, also moving DHTMLX
  `confirm`/`alert`); a `dhx.Window` is shown with `modalPosition()` from
  `src/lib/modal.ts`. Login and unlock cards use the same `--modal-top`.
- Motion uses the `--motion-*`/`--ease-out` tokens in `theme.css` (short,
  decelerating) and is switched off under `prefers-reduced-motion`.
- `localStorage` only for per-viewer conveniences (theme, home card order), always in
  `try/catch`.
- Icons are Material Design Icons (`mdi mdi-<name>`). The picker reads names
  from `virtual:mdi-icons` (Vite plugin in `vite.config.ts`, parsed from
  `@mdi/font`); keep `SUGGESTED_ICONS` valid (unit-tested) and add search
  synonyms to the icon synonyms map in `src/lib/icons.ts`.

## Git

- Remote: GitHub `jniltinho/go-dash` (`gh` CLI).
- Conventional commits in English (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
- Never commit `.env`, `*.db`, `bin/`, `dist/`, `web/dist/*` (except `.gitkeep`),
  `node_modules/`.

## Releases

- CI: `.github/workflows/ci.yml` (push to `main`, pull requests).
- Pushing a tag `vX.Y.Z` triggers `release.yml` (CI, linux amd64/arm64
  binaries, GitHub release with notes from `CHANGELOG.md`) and
  `docker-image-publish.yml`, which pushes `ghcr.io/jniltinho/go-dash`.
- Cut releases with the `create-release` skill.
