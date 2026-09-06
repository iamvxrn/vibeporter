# Scenarios

These are the three workflows to try with a real team. Vibeporter does not sync between laptops: you prepare a **context packet** locally, then deliver it into an agent store or a file the other person can open.

Shared workspace across machines is [Planned](/teams).

## 1. Unfinished task: developer → developer

Developer A stops mid-task. Developer B should continue from decisions and current state, not from a 200k transcript dump.

**A — select and package**

```bash
vibeporter list claudecode
vibeporter search "auth middleware" --agent claudecode

# Preview how much context fits the budget
vibeporter handoff --from claudecode --source <session-id> \
  --to claudecode --compact 100k --dry-run --json
```

**Same machine:** inject a new native session B can open in their agent:

```bash
vibeporter handoff --from claudecode --source <session-id> \
  --to cursor --compact 100k
```

**Different machines:** write a portable file (no Vibeporter account). Send the file; B places it where their agent reads sessions, or opens the Markdown.

```bash
# Native session file for the target agent
vibeporter handoff --from claudecode --source <session-id> \
  --to gemini --compact 100k --target ./task-handoff.jsonl

# Readable briefing if B is not using that agent yet
vibeporter export --from claudecode --source <session-id> \
  --format markdown --output ./task-handoff.md
```

Tell B: task, budget used, and what is still open. Packet metadata lands in `~/.vibeporter/handoffs/` on A's machine only.

## 2. Claude Code → OpenCode

Switch tools without re-explaining the task. This is the local agent integration path.

```bash
vibeporter list claudecode

vibeporter handoff --from claudecode --source <session-id> \
  --to opencode --compact 200k
```

Open OpenCode: a **new** session exists (source Claude session is untouched). Provenance is in the system header.

Tighter window:

```bash
vibeporter handoff --from claudecode --source <session-id> \
  --to opencode --compact 50k --strategy recent
```

Same pattern for any supported pair (`cursor`, `gemini`, `kimicode`, `dsh`, …). See [Integrations](/integrations).

## 3. Old project, new agent — without the full history

An agent should pick up a dormant repo from selected context, not by ingesting every old session.

```bash
# Find the project and the useful session
vibeporter list cursor
vibeporter search "payment retry" --agent cursor

# Budgeted packet into the agent you will use now
vibeporter handoff --from cursor --source <session-id> \
  --to claudecode --compact 100k --strategy smart
```

`smart` keeps early intent when it fits and recent useful context. `recent` keeps only the newest slice. Token counts are heuristic (`tokens~`).

If you only need a briefing in git or Slack, skip inject:

```bash
vibeporter export --from cursor --source <session-id> \
  --format markdown --output ./legacy-project.md
```

---

If a team shrugs at these three, the gap is the workflow — not the landing-page sentence.
