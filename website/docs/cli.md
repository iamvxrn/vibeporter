# CLI Reference

Here are the commands you can use with `vibeporter`.

## `list`

```bash
vibeporter list <agent>
vibeporter list <agent> --json
vibeporter list <agent> --paths
```

**Agents:** `claudecode`, `opencode`, `gemini`, `antigravity` (`ag`), `kimicode` (`kimi`), `dsh` (`dhs`), `cursor`

Prints a table of **title**, **project**, **updated**, and **id** (newest first). Titles come from the agent's own name when it has one (Claude `ai-title`, OpenCode `session.title`), otherwise from the first user message.

`--json` is for scripts (includes the on-disk path). `--paths` adds that column to the table. `migrate` and `handoff` accept the id from this list.

## `handoff`

Create a fresh native target session from compacted local context. It never overwrites the source chat and uses no cloud service or LLM.

```bash
vibeporter handoff --from claudecode --source abc123 --to opencode --compact 200k
vibeporter handoff --from cursor --source /path/to/chat --to gemini --compact 100k --strategy recent --dry-run
```

`--compact` is required and accepts `50k`, `100k`, `200k`, or a positive integer token budget. Token counts are heuristic and are displayed as `tokens~`.

- `--strategy smart` (default) retains the system prompt, early user intent when possible, and useful recent context while dropping heavy noise.
- `--strategy recent` keeps the newest valid context and preserves message ordering.
- `--dry-run` reports the projected handoff without writing a target session.
- `--json` writes the structured handoff report without human output.

Each created handoff has a provenance header and a metadata-only local manifest under `~/.vibeporter/handoffs/`.

## `migrate`

Extract a chat from the source agent and inject it into the target agent's format.

```bash
vibeporter migrate --from <agent> --to <agent> --source <id>
vibeporter migrate --from <agent> --to <agent> --source <id> --target /tmp/out.jsonl
```

**Flags:**
- `--from` — Source agent name
- `--to` — Target agent name
- `--source` — Chat id from `list`, or a file path
- `--target` — Optional. When omitted, writes into the target agent's native store.

## `serve`

```bash
vibeporter serve
```

Starts the local-only web app. Select a chat, choose **Handoff**, set a compact budget and strategy, run a dry preview, then create a native target session. All chat data stays on your device.

## `search`

Full-text search across all chats of all agents.

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

Shows chats, messages, text/thinking/tool counts, total chars and estimated tokens (`chars/4`), plus a bar graph of chat distribution. Sorted by agent.

## `port-config`

Translate project configuration files between agent conventions.

```bash
vibeporter port-config --from <agent> --to <agent> --dir <path>
```

`cursor` is a `list` / `migrate` agent (agent transcripts). Config-file mapping still works independently.

**Supported mappings:** any pair among `claudecode`, `gemini`, `cursor`, `opencode`, and `kimicode`/`kimi`. Instruction files and ignore files are copied to the target names (see [Config Porting](/config-porting)). Existing target files are never overwritten.
