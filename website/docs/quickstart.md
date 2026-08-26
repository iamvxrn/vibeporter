# Quickstart

Move a conversation between agents in three steps.

## 1. Discover your chats

See what's available for a given agent:

```bash
vibeporter list claudecode
vibeporter list opencode
vibeporter list gemini
vibeporter list kimi
vibeporter list dsh
```

Each row is a title, project directory, last update, and id (newest first). Use `--json` if you need the file path.

## 2. Migrate a conversation

Pick a source chat by its id (from `list`). Omit `--target` to write into the destination agent's own store:

```bash
vibeporter migrate \
  --from claudecode \
  --to gemini \
  --source 203f4afc-2fd1-40e9-be7b-fc26d8fc0759
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
