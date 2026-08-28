# Gemini CLI Adapter

The Gemini CLI adapter reads and writes the session transcripts that [Gemini CLI](https://github.com/google-gemini/gemini-cli) records on disk. Extract and inject are both supported, as they are for Claude Code, OpenCode, Kimi Code, and DSH.

## Storage Format

Gemini CLI stores each session as an append-only **JSON Lines** file, one per project:

```
~/.gemini/tmp/<project_hash>/chats/session-<timestamp>-<id>.jsonl
```

`<project_hash>` is derived from the directory you ran Gemini CLI in, so history is project-scoped. Older versions of the CLI wrote a single monolithic `.json` file; this adapter reads both.

Each line is one JSON record:

- **Metadata** (first line): `{ "sessionId", "projectHash", "startTime", "lastUpdated", ... }`.
- **Messages**: `{ "id", "timestamp", "type", "content", ... }` where `type` is `user`, `gemini`, `info`, or `error`. A `gemini` message may also carry `thoughts`, `model`, `toolCalls`, and a `tokens` summary. `content` is a string or an array of parts (`{ "text": ... }`, `{ "functionCall": ... }`, `{ "functionResponse": ... }`).
- **Updates** (`{ "$set": { ... } }`): patch session metadata.
- **Rewinds** (`{ "$rewindTo": "<message-id>" }`): drop that message and everything after it.

Because a message line is re-appended as its content and tool calls grow, records are de-duplicated by `id` (last write wins). `info` and `error` records are UI notices and are skipped during extraction. Extract keeps `thoughts` as thinking parts and `functionCall` / `toolCalls` as tool_call parts.

## CLI token

Use `gemini` wherever an agent name is expected:

```bash
vibeporter list gemini
vibeporter migrate --from claudecode --to gemini --source <path> --target <path>
```

`vibeporter list gemini` scans `~/.gemini/tmp/*/chats/` and shows the first user message as the title.

## Supported Operations

| Operation | Status |
|---|---|
| Extraction (Read) | ✅ Supported |
| Injection (Write) | ✅ Supported |
| Config Porting | ✅ `GEMINI.md`, `.geminiignore` |
