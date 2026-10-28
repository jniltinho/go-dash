---
name: create-release
description: Cut a release of go-dash on GitHub with the gh CLI — CHANGELOG, checks, tag, the release workflow (linux amd64/arm64 tarballs), its verification and the GHCR Docker image (ghcr.io/jniltinho/go-dash). Use when asked to release, tag or publish a version.
---

# Releasing go-dash (GitHub, gh CLI)

The repository is **github.com/jniltinho/go-dash**. A release is: bump `CHANGELOG.md`, commit, push `main`, wait for
the CI, push the tag. The tag starts two workflows:

- `.github/workflows/release.yml` runs the CI again (`ci.yml`), builds `make dist-bin` for linux amd64 and arm64 and
  publishes them in one release with `gh release create`, with the notes of that version's section in
  `CHANGELOG.md` and one `checksums.txt`.
- `.github/workflows/docker-image-publish.yml` builds the `Dockerfile` and pushes `ghcr.io/jniltinho/go-dash:X.Y.Z`,
  `:X.Y` and `:latest` with the workflow's `GITHUB_TOKEN` (no registry secrets).

> The version comes from the **git tag** (`Makefile`: `git describe --tags` → ldflags into `cmd.Version`; the
> workflows pass the tag explicitly). There is nothing to bump in a file.

Release only when the user asks for it.

## Facts

```bash
gh auth status                                  # logged in, with access to jniltinho/go-dash
REPO=jniltinho/go-dash
LAST=$(git describe --tags --abbrev=0 2>/dev/null || git rev-list --max-parents=0 HEAD)   # last tag, or the first commit
```

Versioning (0.x, tags with `v`): a user-visible feature bumps the **minor** (`v0.1.0` → `v0.2.0`); fixes, docs or CI
only bump the **patch** (`v0.1.0` → `v0.1.1`).

## Process

### 1. Prerequisites

```bash
git checkout main && git pull --ff-only
git status                                      # clean, besides the release changes
```

### 2. What changed

```bash
git log "$LAST"..HEAD --oneline
```

### 3. `CHANGELOG.md`

Keep a Changelog, in English. Move the items of `## [Unreleased]` into a new section **above** the previous one —
the release notes are taken from it, by its exact heading:

```markdown
## [Unreleased]

—

## [0.2.0] — YYYY-MM-DD

**One or two lines of summary.**

### Added
### Changed
### Fixed
### Security
```

Omit empty sections.

### 4. Check, commit, push `main`, wait for the CI

```bash
make fmt && make lint && make test && make build
make e2e                                           # when UI or HTTP behaviour changed
git add -A && git diff --cached --name-only        # nothing from bin/, dist/, web/dist, .env, *.db
git commit -m "release: vX.Y.Z"
git push origin main
SHA=$(git rev-parse HEAD)
until RUN=$(gh run list --repo "$REPO" --workflow ci.yml --commit "$SHA" --limit 1 --json databaseId -q '.[0].databaseId') && [ -n "$RUN" ]; do sleep 5; done
gh run watch --repo "$REPO" --exit-status "$RUN"
```

Never tag a commit whose CI is red.

### 5. Tag (starts the release and the Docker image)

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
for wf in release.yml docker-image-publish.yml; do
  until RUN=$(gh run list --repo "$REPO" --workflow "$wf" --commit "$SHA" --event push --limit 1 --json databaseId -q '.[0].databaseId') && [ -n "$RUN" ]; do sleep 5; done
  gh run watch --repo "$REPO" --exit-status "$RUN"
done
```

If a job fails: `gh run view --repo "$REPO" "$RUN" --log-failed`, fix on `main`, and release a new patch version
(do not move a published tag).

### 6. Verify

```bash
gh release view vX.Y.Z --repo "$REPO" --json name,assets -q '.name, (.assets[].name)'
docker pull ghcr.io/jniltinho/go-dash:X.Y.Z && docker run --rm ghcr.io/jniltinho/go-dash:X.Y.Z version
```

Expect `go-dash-vX.Y.Z-linux-amd64.tar.gz`, `go-dash-vX.Y.Z-linux-arm64.tar.gz` and `checksums.txt`; the image
prints `go-dash vX.Y.Z`. The first time, make the GHCR package public if wanted
(github.com/users/jniltinho/packages/container/go-dash/settings).

### 6b. Format the GitHub release notes (rustXterm style)

The workflow publishes with the raw `CHANGELOG.md` section. Keep `CHANGELOG.md` in Keep a Changelog, but give the
**GitHub release** the project's release-notes style: a `# Release vX.Y.Z` title, emoji sections, a
`## 📦 Downloads` list and a `**Full Changelog**` compare link. Map `### Added` → `## ✨ New Features`,
`### Changed`/`### Fixed` → `## 🔧 Improvements`, `### Security` → `## 🔒 Security` (keep the bullets; omit empty
sections; drop the one-line summary).

```markdown
# Release vX.Y.Z

## ✨ New Features
- …

## 🔧 Improvements
- …

## 📦 Downloads
- `go-dash-vX.Y.Z-linux-amd64.tar.gz` — Linux x86_64: binary, `install.sh`, systemd unit, docs
- `go-dash-vX.Y.Z-linux-arm64.tar.gz` — Linux ARM64: same contents
- `checksums.txt` — SHA-256 checksums of every file above
- Docker: `docker pull ghcr.io/jniltinho/go-dash:X.Y.Z`

**Full Changelog**: https://github.com/jniltinho/go-dash/compare/$LAST...vX.Y.Z
```

Write it to `notes.md` and apply: `gh release edit vX.Y.Z --repo "$REPO" --notes-file notes.md` (then `rm notes.md`;
never commit it).

## Guardrails

- Never push a tag before check, tests and the CI of that commit are green.
- Never commit secrets, `bin/`, `dist/`, `web/dist/*`, `.env` or databases.
- The tag is the version; to fix a published release, cut a new patch.
- A release is not done until `ghcr.io/jniltinho/go-dash:X.Y.Z` and `:latest` exist.
