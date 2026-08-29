# Changelog

## [Unreleased]

Cursor extract merges consecutive same-role JSONL chunks (one IR turn per reply). OpenCode inject omits tool parts with no matching result so imported sessions are not padded with empty tool calls.

## [0.3.0] - 2026-08-28

IR messages carry structured parts (text, thinking, tool_call, tool_result). Adapters round-trip tools, thinking, and system prompts. Cursor agent transcripts can be listed, extracted, and injected as a new session.

Extract DSH `.zstd` sessions. OpenCode inject creates the SQLite schema when missing and honors `--target`. Adapter docs match extract+inject support.

OpenCode locates `opencode.db` via XDG / macOS Application Support / `%APPDATA%`. Gemini extract reads legacy pretty-printed `.json` sessions. Claude project dirs replace `:` so Windows drive letters hash. `port-config` maps configs between any supported agents, not only toward Gemini.

CI, lint, and tests: gofmt, golangci-lint, govulncheck, and unit coverage for models, adapters helpers, OpenCode, and CLI commands.

## [0.2.0] - 2026-08-27

Round-trip migrate among the agents we actually support, plus Kimi Code and DeepSeek Harness.

- Inject writes a **new** session for Claude Code, OpenCode, Gemini CLI, Kimi Code, and DSH (never updates an existing one)
- `migrate --target` is optional; default is the target agent's native store
- `list` shows title, project, and date; `--json` / `--paths` for file paths; `migrate --source` accepts a list id
- Extract keeps timestamps; Claude skips meta/slash-command noise; OpenCode includes tool-use markers
- Adapters: `kimicode`/`kimi` (`~/.kimi-code`), `dsh`/`dhs` (`~/.dsh`)
- npm wrapper (`npx vibeporter@latest`) downloads the GitHub Release binary on first run

## [0.1.0] - 2026-08-25

Initial release of Vibeporter — a CLI for migrating chat histories and project configs between AI coding agents.

- Commands: `list`, `migrate`, `port-config`
- Extract adapters: Claude Code (JSONL), OpenCode (SQLite), Gemini CLI (JSONL)
- Inject adapter: Gemini CLI (JSONL session format)
- Config porting: `CLAUDE.md` / `.cursorrules` / `OPENCODE.md` → `GEMINI.md`
- Pure-Go SQLite via `modernc.org/sqlite` (no CGO)
- VitePress documentation site with per-agent adapter pages
