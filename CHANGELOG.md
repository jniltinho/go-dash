# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/) (tags `vX.Y.Z`).

## [Unreleased]

—

## [0.2.0] — 2026-10-29

**A slimmer, quieter interface and a one-command Docker quick start.**

### Changed

- Slimmer bars at every scale (Full HD, QHD, 4K): toolbar 40 → 32 px, tab bar 32 → 26 px, home card headers
  36 → 28 px, item context bar 36 → 30 px.
- Smaller toolbar typography: 12 px labels, 15 px icons, 13 px title (scaled by `--ui-scale`).
- "Open in new tab" and "Reload" in the item context bar are icon-only buttons with a tooltip and an accessible name.
- Minimalist language picker on the login, unlock and error screens: inline muted text instead of a boxed field.
- README quick start runs the published `ghcr.io/jniltinho/go-dash` image with `docker run` or Docker Compose.

## [0.1.0] — 2026-10-28

**First public release of Go Dash: a link aggregator dashboard in one Go binary.**

### Added

- Header toolbar with menus and nested submenus; links open in a sandboxed iframe tab or a new browser tab.
- Multiple dashboards (`/` and `/dash/<slug>`), optional per-dashboard passwords.
- Admin area: menu editor, icon picker, import/export, dashboard management.
- 25 WCAG AAA themes, flat and 3D styles.
- Single static binary (frontend embedded), SQLite, configurable base path behind a reverse proxy.
- CLI (`go-dash serve|migrate|menu|dash|admin|backup|version`), systemd installer and Docker image.
- CI, release workflow and Docker image on `ghcr.io/jniltinho/go-dash`.
