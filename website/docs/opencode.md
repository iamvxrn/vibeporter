# OpenCode Adapter

The OpenCode adapter reads chat data from a local SQLite database at `~/.local/share/opencode/opencode.db`. `list` uses `session.title`, `directory`, and `time_updated`.

## Storage Format

OpenCode uses a normalized SQLite schema with `session`, `message`, and `part` tables. Messages are joined with parts to reconstruct conversation content.

## Pure Go SQLite

Vibeporter uses `modernc.org/sqlite` — a pure Go SQLite implementation with zero CGO dependencies. This means the binary cross-compiles to any platform without needing a C compiler.

## Supported Operations

| Operation | Status |
|---|---|
| Extraction (Read) | ✅ Supported |
| Injection (Write) | ❌ Coming Soon |
| Config Porting | ✅ `OPENCODE.md`, `.opencodeignore` |
