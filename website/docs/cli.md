# CLI Reference

Here are the commands you can use with `vibeporter`.

## `list`

```bash
vibeporter list <agent>
vibeporter list <agent> --json
vibeporter list <agent> --paths
```

**Agents:** `claudecode`, `opencode`, `gemini`, `kimicode` (`kimi`), `dsh` (`dhs`), `cursor`

Prints a table of **title**, **project**, **updated**, and **id** (newest first). Titles come from the agent's own name when it has one (Claude `ai-title`, OpenCode `session.title`), otherwise from the first user message.

`--json` is for scripts (includes the on-disk path). `--paths` adds that column to the table. `migrate --source` accepts the id from this list.

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

## `search`

Full-text search across all chats of all agents.

```bash
vibeporter search "fix database bug"
vibeporter search "auth" --agent gemini --limit 20
vibeporter search "panic" --json | jq
```

Scans titles, projects, and all message parts (text, thinking, tool calls/results). `--agent` limits to one agent, otherwise searches `claudecode`, `opencode`, `gemini`, `kimicode`, `dsh`, `cursor`. `--limit` caps results (default 20). `--json` emits `agent/id/title/project/path/snippet/matches`. Human output shows a table sorted by updated time plus snippet preview.

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
