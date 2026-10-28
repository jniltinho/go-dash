# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/) (tags `vX.Y.Z`).

## [Unreleased]

—

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
