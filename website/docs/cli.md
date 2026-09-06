# CLI Reference

Commands for local engineering context. Task handoff and adapters are [integrations](/integrations).

## `list`

```bash
vibeporter list <agent>
vibeporter list <agent> --json
vibeporter list <agent> --paths
```

**Agents:** `claudecode`, `opencode`, `gemini`, `antigravity` (`ag`), `kimicode` (`kimi`), `dsh` (`dhs`), `cursor`

Prints a table of **title**, **project**, **updated**, and **id** (newest first). Titles come from the agent's own name when it has one (Claude `ai-title`, OpenCode `session.title`), otherwise from the first user message.

`--json` is for scripts (includes the on-disk path). `--paths` adds that column to the table. `handoff` accepts the id from this list.

## `handoff`

Create a context packet and deliver it as a fresh native target session. It never overwrites the source session and uses no cloud service or LLM.

```bash
vibeporter handoff --from claudecode --source abc123 --to opencode --compact 200k
vibeporter handoff --from cursor --source /path/to/chat --to gemini --compact 100k --strategy recent --dry-run
```

`--compact` is required and accepts `50k`, `100k`, `200k`, or a positive integer token budget. Token counts are heuristic and are displayed as `tokens~`.

- `--strategy smart` (default) retains the system prompt, early user intent when possible, and useful recent context while dropping heavy noise.
- `--strategy recent` keeps the newest valid context and preserves message ordering.
- `--dry-run` reports the projected packet without writing a target session.
- `--json` writes the structured report (includes `packet` when present) without human output.

Each created handoff writes packet JSON under `~/.vibeporter/handoffs/` and a provenance header into the target session. See [Context model](/context).

## `migrate`

Raw copy of a source session into the target agent's format (no compacting). Prefer `handoff` for a budgeted context packet. The command name is kept for compatibility.

```bash
vibeporter migrate --from <agent> --to <agent> --source <id>
vibeporter migrate --from <agent> --to <agent> --source <id> --target /tmp/out.jsonl
```

**Flags:**
- `--from` — Source agent name
- `--to` — Target agent name
- `--source` — Session id from `list`, or a file path
- `--target` — Optional. When omitted, writes into the target agent's native store.

## `diff`

Compare the original source session with what the target agent would store after a raw copy — before writing one.

```bash
vibeporter diff --from claudecode --to gemini --source <id>
vibeporter diff --from claudecode --to gemini --source <id> --json
```

It extracts the source, does a temp round-trip to the target format, and reports counts and dropped parts. No real data is written to the target agent's store.

- `--from` / `--to` / `--source` — same meaning as `handoff`.
- `--json` — machine-readable report instead of the human summary.

## `export`

Extract a source session and render it as Markdown or HTML, for sharing or docs — not a handoff to another agent.

```bash
vibeporter export --from claudecode --source <id> --format markdown --output context.md
vibeporter export --from gemini --source ~/.gemini/tmp/.../chats/session.jsonl --format html
```

- `--from` / `--source` — session id from `list`, or a file path.
- `--format` — `markdown` or `html`. Defaults to `markdown`.
- `--output` — output file. Defaults to stdout; pass `-` for stdout explicitly.

## `serve`

```bash
vibeporter serve
```

Starts the local-only web app. Select a **source**, create a **context packet** via Handoff, set a compact budget and strategy, dry-run, then deliver a native target session. All data stays on your device. There is no account.

It binds to loopback, but loopback is reachable from any page open in your browser, not just this one -- so every API route refuses a request that doesn't look like it came from the app itself (checked by `Sec-Fetch-Site`, `Origin`, and requiring `Content-Type: application/json` on writes, which a cross-site request cannot set without a preflight this server never approves).

## `search`

Full-text search across source sessions of all agents.

```bash
vibeporter search "fix database bug"
vibeporter search "auth" --agent gemini --limit 20
vibeporter search "panic" --json | jq
```

Scans titles, projects, and all message parts (text, thinking, tool calls/results). `--agent` limits to one agent, otherwise searches `claudecode`, `opencode`, `gemini`, `antigravity` (`ag`), `kimicode`, `dsh`, `cursor`. `--limit` caps results (default 20). `--json` emits `agent/id/title/project/path/snippet/matches`. Human output shows a table sorted by updated time plus snippet preview.

## `stats`

Analytics per agent.

```bash
vibeporter stats
vibeporter stats --agent gemini --json | jq
```

Shows source-session counts, messages, text/thinking/tool counts, total chars and estimated tokens (`chars/4`), plus a bar graph of distribution. JSON field `chats` is unchanged for compatibility.

## `port-config`

Translate project configuration files between agent conventions.

```bash
vibeporter port-config --from <agent> --to <agent> --dir <path>
```

`cursor` is a `list` / `handoff` agent (agent transcripts). Config-file mapping still works independently.

**Supported mappings:** any pair among `claudecode`, `gemini`, `cursor`, `opencode`, and `kimicode`/`kimi`. Instruction files and ignore files are copied to the target names (see [Config Porting](/config-porting)). Existing target files are never overwritten.
