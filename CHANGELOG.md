# Changelog

## [Unreleased]

- npm wrapper (`npx vibeporter@latest`) downloads the GitHub Release binary on first run
- Release workflow publishes to npm when `NPM_TOKEN` is set
- `list` shows title, project, and date (use `--json` / `--paths` for file paths); `migrate --source` accepts a list id

## [0.1.0] - 2026-08-25

Initial release of Vibeporter — a CLI for migrating chat histories and project configs between AI coding agents.

- Commands: `list`, `migrate`, `port-config`
- Extract adapters: Claude Code (JSONL), OpenCode (SQLite), Gemini CLI (JSONL)
- Inject adapter: Gemini CLI (JSONL session format)
- Config porting: `CLAUDE.md` / `.cursorrules` / `OPENCODE.md` → `GEMINI.md`
- Pure-Go SQLite via `modernc.org/sqlite` (no CGO)
- VitePress documentation site with per-agent adapter pages
