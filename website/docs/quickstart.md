# Quickstart

Move a conversation between agents in three steps.

## 1. Discover your chats

See what's available for a given agent:

```bash
vibeporter list claudecode
vibeporter list opencode
vibeporter list gemini
```

Each row shows a chat ID and where it lives on disk.

## 2. Migrate a conversation

Pick a source chat and choose where it should go:

```bash
vibeporter migrate \
  --from claudecode \
  --to gemini \
  --source ~/.claude/projects/.../session-id.jsonl \
  --target /tmp/migrated.jsonl
```

Behind the scenes, Vibeporter:
1. Parses the source agent's storage format (JSONL, SQLite).
2. Converts the messages into a common intermediate representation.
3. Serializes that into the target agent's format.

## 3. Port your project configs

Bring your workspace rules and ignore files along too:

```bash
cd /path/to/your/project
vibeporter port-config --from claudecode --to gemini --dir .
```

This copies `CLAUDE.md` → `GEMINI.md` and `.claudeignore` → `.geminiignore`, and it never overwrites a file that already exists. See [Config Porting](/config-porting) for the full mapping.
