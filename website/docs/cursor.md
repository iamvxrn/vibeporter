# Cursor Adapter

The Cursor adapter **lists and extracts** agent transcripts. It does **not** inject: Cursor's session format is not a stable import target.

## Storage Format

Agent chats live as JSONL under:

```
~/.cursor/projects/<project>/agent-transcripts/<id>/<id>.jsonl
```

Override the projects root with `CURSOR_PROJECTS_DIR`. Lines look like `{ "role", "message": { "content": [ { "type": "text"|"tool_use"|"tool_result"|"thinking", ... } ] } }`. Subagent transcripts are skipped.

`list` titles come from the first user message (`<user_query>` wrappers are stripped).

## CLI

```bash
vibeporter list cursor
vibeporter migrate --from cursor --to gemini --source <id>
```

`--to cursor` is not supported.

## Supported Operations

| Operation | Status |
|---|---|
| Extraction (Read) | ✅ Agent transcripts |
| Injection (Write) | ❌ Not supported |
| Config Porting | ✅ `.cursorrules`, `.cursorignore` |
