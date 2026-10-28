# CLAUDE.md

@AGENTS.md

Claude Code specifics:

- Go skills live in `.claude/skills/golang-*`; OpenSpec workflow via `/opsx:propose`, `/opsx:apply`, `/opsx:archive`.
- Validate significant changes with Codex in read-only mode, e.g.
  `codex exec --skip-git-repo-check -s read-only "Review ..."`.
