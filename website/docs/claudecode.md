# Claude Code Adapter

The Claude Code adapter reads JSONL conversation logs from the `~/.claude/projects/` directory.

## Storage Format

Claude Code stores conversations as JSONL files at:
```
~/.claude/projects/<project-path>/<session-id>.jsonl
```

Each line is a JSON object. Messages have a `type` field (`user` or `assistant`) and a nested `message.content` array containing text and tool-use parts.

## Supported Operations

| Operation | Status |
|---|---|
| Extraction (Read) | ✅ Supported |
| Injection (Write) | ❌ Coming Soon |
| Config Porting | ✅ `CLAUDE.md`, `.claudeignore` |
