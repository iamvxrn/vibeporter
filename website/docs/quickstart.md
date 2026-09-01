# Quickstart

Hand off useful context to another agent in three steps.

## 1. Discover your chats

See what's available for a given agent:

```bash
vibeporter list claudecode
vibeporter list opencode
vibeporter list gemini
vibeporter list kimi
vibeporter list dsh
vibeporter list cursor
```

Each row is a title, project directory, last update, and id (newest first). Use `--json` if you need the file path.

## 2. Create a context handoff

Pick a source chat by its id (from `list`). Choose a context budget; omit `--target` to write into the destination agent's own store:

```bash
vibeporter handoff \
  --from claudecode \
  --to opencode \
  --source 203f4afc-2fd1-40e9-be7b-fc26d8fc0759 \
  --compact 200k

vibeporter handoff --from gemini --to cursor --source <id> --compact 100k --strategy recent --dry-run
```

Behind the scenes, Vibeporter:
1. Parses the source agent's storage format (JSONL, SQLite).
2. Selects local context to the requested token budget using `smart` or `recent`.
3. Adds provenance and serializes a new session into the target agent's format.

## 3. Port your project configs

Bring your workspace rules and ignore files along too:

```bash
cd /path/to/your/project
vibeporter port-config --from claudecode --to gemini --dir .
```

This copies `CLAUDE.md` → `GEMINI.md` and `.claudeignore` → `.geminiignore`, and it never overwrites a file that already exists. See [Config Porting](/config-porting) for the full mapping.
