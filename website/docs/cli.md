# CLI Reference

Here are the commands you can use with `vibeporter`.

## `list`

Discover available chat histories for a specific agent.

```bash
vibeporter list <agent>
```

**Agents:** `claudecode`, `opencode`, `gemini`

## `migrate`

Extract a chat from the source agent and inject it into the target agent's format.

```bash
vibeporter migrate --from <agent> --to <agent> --source <path> --target <path>
```

**Flags:**
- `--from` — Source agent name
- `--to` — Target agent name
- `--source` — Path to source chat (file path or session ID)
- `--target` — Path to write the converted output

## `port-config`

Translate project configuration files between agent conventions.

```bash
vibeporter port-config --from <agent> --to <agent> --dir <path>
```

**Supported mappings:**

| From | To | Files |
|---|---|---|
| `claudecode` | `gemini` | `CLAUDE.md` → `GEMINI.md`, `.claudeignore` → `.geminiignore` |
| `cursor` | `gemini` | `.cursorrules` → `GEMINI.md`, `.cursorignore` → `.geminiignore` |
| `opencode` | `gemini` | `OPENCODE.md` → `GEMINI.md`, `.opencodeignore` → `.geminiignore` |
