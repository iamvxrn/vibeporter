# Quickstart

Build a context packet from a local source session and deliver it to another agent.

## 1. Discover source sessions

```bash
vibeporter list claudecode
vibeporter list opencode
vibeporter list gemini
vibeporter list kimi
vibeporter list dsh
vibeporter list cursor
```

Each row is a title, project directory, last update, and id (newest first). Use `--json` if you need the file path.

## 2. Create a context packet (task handoff)

Pick a source session by its id (from `list`). `--compact` is the context budget. Omit `--target` to write into the destination agent's own store:

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
3. Records provenance on a **context packet** and serializes a new session into the target agent's format.
4. Writes packet JSON under `~/.vibeporter/handoffs/` (not on `--dry-run`).

This is local. The other person only sees the result if they use the same machine (or you copy the target session yourself). There is no Vibeporter account.

Walk through the three team-facing flows in [Scenarios](/scenarios).

## 3. Optional: local hub

```bash
vibeporter serve
```

Opens a loopback UI labeled Context / Sources / Handoffs. Same APIs as the CLI; same-origin protections stay on.

## 4. Project instruction files

```bash
cd /path/to/your/project
vibeporter port-config --from claudecode --to gemini --dir .
```

This copies `CLAUDE.md` → `GEMINI.md` and `.claudeignore` → `.geminiignore`, and never overwrites a file that already exists. See [Config porting](/config-porting) and [Integrations](/integrations).
