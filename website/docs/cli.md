# CLI Reference

Here are the commands you can use with `vibeporter`.

## `list`

```bash
vibeporter list <agent>
vibeporter list <agent> --json
vibeporter list <agent> --paths
```

**Agents:** `claudecode`, `opencode`, `gemini`

Prints a table of **title**, **project**, **updated**, and **id** (newest first). Titles come from the agent's own name when it has one (Claude `ai-title`, OpenCode `session.title`), otherwise from the first user message.

`--json` is for scripts (includes the on-disk path). `--paths` adds that column to the table. `migrate --source` accepts the id from this list.

## `migrate`

Extract a chat from the source agent and inject it into the target agent's format.

```bash
vibeporter migrate --from <agent> --to <agent> --source <path> --target <path>
```

**Flags:**
- `--from` — Source agent name
- `--to` — Target agent name
- `--source` — Chat id from `list`, or a file path
- `--target` — Path to write the converted output

## `port-config`

Translate project configuration files between agent conventions.

```bash
vibeporter port-config --from <agent> --to <agent> --dir <path>
```

`cursor` is config-file mapping only. It is not a `list` / `migrate` agent.

**Supported mappings:**

| From | To | Files |
|---|---|---|
| `claudecode` | `gemini` | `CLAUDE.md` → `GEMINI.md`, `.claudeignore` → `.geminiignore` |
| `cursor` | `gemini` | `.cursorrules` → `GEMINI.md`, `.cursorignore` → `.geminiignore` |
| `opencode` | `gemini` | `OPENCODE.md` → `GEMINI.md`, `.opencodeignore` → `.geminiignore` |
