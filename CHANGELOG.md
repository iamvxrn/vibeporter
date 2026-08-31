# Changelog

## [Unreleased]

## [0.4.1] - 2026-08-30

- Vibrant CLI: `list`/`search`/`stats`/`diff` with agent colors/icons, `HighlightMatch`, bright palette (`internal/cmd/color.go`, `list.go`, `search.go`, `stats.go`, `diff.go`)
- Web muted palette: `internal/web/static/style.css` monochrome `#0f0f0f`, remove purple gradients (fix `gofmt` and `staticcheck QF1003` for `fidelity_test.go`)
- Lint: `gofmt` for `fidelity_test.go` imports/structs and `server.go` alignment, `golangci-lint` clean

## [0.4.0] - 2026-08-30

The search + stats + hub release.

- **Search** — `vibeporter search "fix database bug" [--agent gemini] [--limit 20] [--json]` full-text across all agents' titles, projects, and message parts (text, thinking, tool calls) with centered snippet and match count (`internal/cmd/search.go`)
- **Stats** — `vibeporter stats [--agent X] [--json]` per-agent `chats/messages/text/thinking/tools/chars/tokens~` with bar graph (`internal/cmd/stats.go`)
- **Antigravity** — new adapter for `~/.gemini/antigravity/brain/*/transcript.jsonl` (`antigravity`/`ag`) with `List`/`Extract`/`Inject` and 0-diff to `opencode`/`claudecode` (`internal/adapters/antigravity`)
- **Windsurf** — new adapter for `~/.windsurf`/`~/.codeium/windsurf` (`windsurf`/`wind`) with JSONL `role/message.content` handling (`internal/adapters/windsurf`)
- **Web UI prototype** — `vibeporter serve` (`--addr :8080`) serves `landing + opencode-style hub` at `internal/web` with `/api/agents|chats|conversation|search|diff|migrate|stats`, muted palette, drag, preview, and real `POST /api/migrate` (`internal/web/server.go`, `static/*`)
- **Fidelity** — preserve tool calls in `opencode` even without `tool_result` output (fix `cursor → opencode` `1259 → 0` → `0 diff`), update `simulateOpencodeRoundTrip`, synthetic round-trip tests for 7 adapters (`internal/cmd/fidelity_test.go`)
- **Docs** — `README` and `website/docs/cli.md` list `antigravity`/`windsurf`, `search`/`stats`/`serve` sections

Fixes `opencode` tool-call loss that caused `что мы делали` to return garbage after `cursor → opencode` migration. Previous unreleased fix remains: cursor merges same-role chunks, opencode inject behavior now documented.

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
