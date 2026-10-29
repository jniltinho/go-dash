# <img src="frontend/public/favicon.svg" width="36" height="36" alt="" align="top"> Go Dash

**Every system your team uses, one click away.** A link dashboard with a header
menu, tabs that keep each system open side by side, and a Home of cards —
shipped as a single binary.

![Go Dash — Ocean Light theme](docs/screenshots/theme-ocean-light-flat.png)

- **Menus and submenus** in a slim header toolbar; links open in **tabs**
  (sandboxed iframe) or in a **new browser tab**
- **Home of cards** that admins rearrange by drag and drop, for everyone
- **25 themes, all WCAG AAA** — Ocean Light/Dark, macOS, Ubuntu, Windows 10/7/XP/98,
  Linux KDE, xterm, Matrix, Brasil… each analyst picks their own, **Flat** or **No Flat**
- **Built-in admin** with a visual icon picker, plus a full **CLI**
- **One static binary** (Go + Vue embedded, SQLite), Docker image ~50 MB,
  Full HD to 4K

| | | |
| --- | --- | --- |
| ![macOS](docs/screenshots/theme-macos-flat.png) | ![Windows 10](docs/screenshots/theme-win10-flat.png) | ![xterm](docs/screenshots/theme-xterm-flat.png) |
| ![Ubuntu](docs/screenshots/theme-ubuntu-flat.png) | ![Windows XP — No Flat](docs/screenshots/theme-xp-3d.png) | ![Matrix](docs/screenshots/theme-matrix-flat.png) |

## Quick start

Run the published image ([`ghcr.io/jniltinho/go-dash`](https://github.com/jniltinho/go-dash/pkgs/container/go-dash)),
no clone or build needed:

```bash
docker run -d --name go-dash -p 8080:8080 \
  -e DASHBOARD_ADMIN_PASSWORD='change-me' \
  -v go-dash-data:/data \
  ghcr.io/jniltinho/go-dash:latest
```

Open <http://localhost:8080>; the first start imports the example menu. Sign in
at `/admin` with the password above to edit menus, cards and dashboards. Data
(the SQLite database) lives in the `go-dash-data` volume, so upgrading is
`docker pull` plus recreating the container.

With Docker Compose, using the release [`docker-compose.yml`](deploy/docker-compose.yml):

```bash
curl -fsSLO https://raw.githubusercontent.com/jniltinho/go-dash/main/deploy/docker-compose.yml
DASHBOARD_ADMIN_PASSWORD='change-me' docker compose up -d
```

From a clone, `cp .env.example .env` (set `DASHBOARD_ADMIN_PASSWORD`) and
`make docker-up` builds the image locally.

Without Docker (Go 1.26+, Node.js 24+):

```bash
make build                                   # bin/go-dash (frontend embedded)
./bin/go-dash menu import examples/menu.json
./bin/go-dash admin set-password           # enables /admin
./bin/go-dash serve                        # http://localhost:8080
```

**On a Linux server** (systemd service in `/opt/go-dash`, or Docker Compose,
including how to install Docker): see the **[installation guide](docs/install.md)**.

## Learn more

- **[Installation guide](docs/install.md)** — Linux server: systemd service or
  Docker + Compose, HTTPS proxy, backup, upgrade
- **[Guide](docs/README.md)** — all themes, menu and cards, CLI, configuration,
  iframes, security, backup and development
- [Architecture](docs/architecture.md) — diagrams
- [Specification](openspec/specs) — OpenSpec specs (history in [`openspec/changes/archive`](openspec/changes/archive))
- [AGENTS.md](AGENTS.md) — conventions for contributors and AI agents

DHTMLX Suite is used under GPL v2; a proprietary distribution requires a DHTMLX
commercial license. Inter is bundled under the SIL OFL 1.1.
