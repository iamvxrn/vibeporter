# Claude Code Adapter

The Claude Code adapter reads JSONL conversation logs from the `~/.claude/projects/` directory. `list` uses the session's `ai-title` when present, otherwise the first user message, plus the `cwd` as the project.

## Storage Format

Claude Code stores conversations as JSONL files at:
```
~/.claude/projects/<project-path>/<session-id>.jsonl
```

Each line is a JSON object. Messages have a `type` field (`user`, `assistant`, or `system`) and a nested `message.content` array. Extract keeps **text**, **thinking**, **tool_use**, and **tool_result** parts in the IR, plus `system` prompts (Claude `init` / status lines are skipped). Inject writes them back the same way.

## Supported Operations

| Operation | Status |
|---|---|
| Extraction (Read) | ✅ Supported |
| Injection (Write) | ✅ New session only |
| Config Porting | ✅ `CLAUDE.md`, `.claudeignore` |
