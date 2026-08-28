# Kimi Code Adapter

The Kimi Code adapter reads and writes [Kimi Code CLI](https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/data-locations.html) sessions under `~/.kimi-code/` (or `$KIMI_CODE_HOME`).

CLI names: `kimicode` or `kimi`.

## Storage Format

```
~/.kimi-code/sessions/<workDirKey>/<sessionId>/
  state.json
  agents/main/wire.jsonl
```

`wire.jsonl` is an event stream (`turn.prompt`, `context.append_loop_event`), not a flat message log. Consecutive assistant events (text, thinking, `tool.call`, `tool.result`) merge into one IR message. `turn.prompt` with `origin.kind` `system` is a system prompt. `list` uses `state.json` title and `workDir` when present.

Inject writes a **new** session and appends `session_index.jsonl`. It does not edit existing sessions.

## Supported Operations

| Operation | Status |
|---|---|
| Extraction (Read) | ✅ Supported |
| Injection (Write) | ✅ New session only |
| Config Porting | ✅ `AGENTS.md` → `GEMINI.md` |
