# Cursor Adapter

The Cursor adapter lists, extracts, and injects agent transcripts.

## Storage Format

Agent chats live as JSONL under:

```
~/.cursor/projects/<project>/agent-transcripts/<id>/<id>.jsonl
```

Override the projects root with `CURSOR_PROJECTS_DIR`. Lines look like `{ "role", "message": { "content": [ { "type": "text"|"tool_use"|"tool_result"|"thinking", ... } ] } }`. Roles are `user`, `assistant`, and `system`. Consecutive same-role lines are merged into one IR message (Cursor streams one JSONL line per chunk). Subagent transcripts are skipped.

`list` titles come from the first user message (`<user_query>` wrappers are stripped). Inject wraps user text in `<user_query>` so Cursor can display it, and writes a **new** session under the project encoded from `cwd`.

## CLI

```bash
vibeporter list cursor
vibeporter migrate --from gemini --to cursor --source <id>
vibeporter migrate --from cursor --to gemini --source <id>
```

## Supported Operations

| Operation | Status |
|---|---|
| Extraction (Read) | ✅ Agent transcripts |
| Injection (Write) | ✅ New session only |
| Config Porting | ✅ `.cursorrules`, `.cursorignore` |
