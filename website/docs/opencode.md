# OpenCode Adapter

The OpenCode adapter reads chat data from a local SQLite database. The default path is `~/.local/share/opencode/opencode.db` (Linux), `~/Library/Application Support/opencode/opencode.db` (macOS), or `%APPDATA%\opencode\opencode.db` (Windows). `$XDG_DATA_HOME/opencode/opencode.db` is used when that file exists. `list` uses `session.title`, `directory`, and `time_updated`.

## Storage Format

OpenCode uses a normalized SQLite schema with `session`, `message`, and `part` tables. Extract maps `text`, `reasoning`, and `tool` parts into the IR. Inject writes those part types back.

## Pure Go SQLite

Vibeporter uses `modernc.org/sqlite` — a pure Go SQLite implementation with zero CGO dependencies. This means the binary cross-compiles to any platform without needing a C compiler.

## Supported Operations

| Operation | Status |
|---|---|
| Extraction (Read) | ✅ Supported |
| Injection (Write) | ✅ New session only |
| Config Porting | ✅ `OPENCODE.md`, `.opencodeignore` |
