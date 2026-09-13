> [!IMPORTANT]
> **This project is archived.** It is no longer maintained and will not
> receive updates. The code stays available and works as documented.

<p align="center">
  <img src="website/docs/public/social.png" alt="vibeporter — local, portable engineering context across agents, people, and projects" width="720">
</p>

# Vibeporter

**Vibeporter preserves engineering context locally and makes it portable across AI agents, developers, teams, and projects.**

It is a local context layer for AI-assisted development: project context, current state, and provenance stay on your machine, then travel as a **context packet** into another agent, export, or (later) a team workflow.

Porting and task handoff remain first-class **integrations**. They deliver context. They are not the product headline.

[![Documentation](https://img.shields.io/badge/docs-vibeporter.pages.dev-8b5cf6.svg)](https://vibeporter.pages.dev)
[![OS Matrix](https://img.shields.io/badge/OS-Linux%20%7C%20macOS%20%7C%20Windows-8b5cf6.svg)](#)

## The problem

Coding agents forget. A long session holds decisions, constraints, and open questions that never make it into git. The next agent, teammate, or project starts from a blank window.

Vibeporter treats that working memory as **shared engineering context**, not as a chat file to convert.

## What you get today

- **Context packets** — budgeted, provenance-tagged selections from local source sessions (`~/.vibeporter/handoffs/`).
- **Local only** — no cloud, no account, no telemetry, no daemon. The binary talks to agent stores on this machine.
- **Agent integrations** — Claude Code, OpenCode, Gemini CLI, Antigravity, Kimi Code, DeepSeek Harness, Cursor (read and write). Windsurf is listed where extract exists.
- **Task handoff** — `handoff` compacting a source session into a fresh native session.
- **Project files** — `port-config` copies instruction files (`CLAUDE.md` → `GEMINI.md`, and similar).

Honest limits: there is no shared team cloud, no login, and no automatic extraction of a “decisions” list from prose. Those fields exist on the packet schema for later use.

## Context packet

A packet is the unit of transfer:

```
Project
  decisions
  current state
  constraints
  open questions
  artifacts
  handoffs
  source sessions
```

Today a packet is produced by selecting context from one source session, tagging provenance, and optionally injecting it into another agent. Structured lists (decisions, constraints, questions, artifacts) are reserved in the JSON; they are not guessed from chat text.

See [Context model](https://vibeporter.pages.dev/context).

## Scenarios

The three workflows to show a team (local packet, then deliver — not a shared cloud):

1. Unfinished task from one developer to another
2. Claude Code → OpenCode
3. Old project into a new agent without the full history

Commands: [Scenarios](https://vibeporter.pages.dev/scenarios). Shared workspace across machines is [planned](https://vibeporter.pages.dev/teams).

## Quick start

### Installation

```bash
curl -fsSL https://vibeporter.pages.dev/install.sh | sh
```

Windows:

```powershell
Invoke-Expression (Invoke-WebRequest -Uri "https://vibeporter.pages.dev/install.ps1" -UseBasicParsing).Content
```

Alternative (downloads the same GitHub Release binary on first run):

```bash
npx vibeporter@latest list claudecode
```

### Usage

```bash
vibeporter list claudecode

vibeporter handoff --from claudecode --source <session-id-from-list> \
  --to opencode --compact 200k

vibeporter handoff --from cursor --source /path/to/chat --to gemini \
  --compact 100k --strategy recent --dry-run

vibeporter serve

vibeporter search "fix database bug" --agent gemini
vibeporter stats --json | jq
```

## Integrations

Handoff, export, adapters, and `port-config` are documented as [integrations](https://vibeporter.pages.dev/integrations) — mechanisms that move context, not the definition of the product. The `migrate` command still exists as a compatibility name for an un-compacted session copy.

```bash
vibeporter port-config --from claudecode --to gemini --dir .
```

## Documentation

[https://vibeporter.pages.dev](https://vibeporter.pages.dev)

## License

MIT
