# CLI Reference

Here are the commands you can use with `vibeporter`.

## `list`

```bash
vibeporter list <agent>
vibeporter list <agent> --json
vibeporter list <agent> --paths
```

**Agents:** `claudecode`, `opencode`, `gemini`, `kimicode` (`kimi`), `dsh` (`dhs`), `cursor` (extract only)

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
- `--to` — Target agent name (not `cursor`; extract-only)
- `--source` — Chat id from `list`, or a file path
- `--target` — Optional. When omitted, writes into the target agent's native store.

## `port-config`

Translate project configuration files between agent conventions.

```bash
vibeporter port-config --from <agent> --to <agent> --dir <path>
```

`cursor` is a `list` / `migrate --from` agent (agent transcripts). It is not a `migrate --to` target. Config-file mapping still works.

**Supported mappings:** any pair among `claudecode`, `gemini`, `cursor`, `opencode`, and `kimicode`/`kimi`. Instruction files and ignore files are copied to the target names (see [Config Porting](/config-porting)). Existing target files are never overwritten.
