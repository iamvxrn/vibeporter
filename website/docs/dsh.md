# DeepSeek Harness Adapter

The DSH adapter reads and writes [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) session logs under `~/.dsh/sessions/` (or `$DSH_HOME`).

CLI names: `dsh` or `dhs`.

## Storage Format

```
~/.dsh/sessions/<workspace>/<sessionId>/session.jsonl
```

The first line is a `session` header (`id`, `cwd`, `delegationDepth`). Later lines are events; extract keeps `user/message` and `assistant/message`.

Compressed `session.jsonl.zstd` files are listed but not extracted yet — export an uncompressed log, or pass a `.jsonl` path.

Inject writes a **new uncompressed** `session.jsonl` (header plus surface messages). It does not modify existing logs.

## Supported Operations

| Operation | Status |
|---|---|
| Extraction (Read) | ✅ Uncompressed JSONL |
| Injection (Write) | ✅ New session only |
| Config Porting | ❌ Not mapped |
