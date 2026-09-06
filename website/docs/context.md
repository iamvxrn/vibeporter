# Context model

A **context packet** is the unit Vibeporter stores and delivers. A source session is an input. A native agent chat is one output channel.

## Shape

```
Project
  ├── decisions
  ├── current state
  ├── constraints
  ├── open questions
  ├── artifacts
  ├── handoffs
  └── source sessions
```

| Field | Meaning | Today |
|---|---|---|
| **Project** | Workspace path when the source session recorded `cwd` / project metadata | Copied from session metadata when present |
| **Decisions** | Choices the team or agent already made | Schema only — not extracted from prose |
| **Current state** | What is true now | Last useful user/assistant text, clipped |
| **Constraints** | Must-not-break rules | Schema only |
| **Open questions** | Unresolved issues | Schema only |
| **Artifacts** | Paths, PRs, docs worth attaching | Schema only |
| **Handoffs** | How the packet was delivered | `native_session` + target agent/path after `handoff` |
| **Source sessions** | Provenance: which agent and id were read | Always set on `handoff` |

Kind in JSON: `vibeporter.context_packet` (version `1`).

## What is selected

`handoff --compact` keeps a budgeted slice of the source session (`smart` or `recent`). Token counts are heuristic (`tokens~`). A system message records provenance in the delivered session.

The packet file is metadata plus those measurements. It does **not** dump the full transcript into `~/.vibeporter/handoffs/` — the selected messages live in the new native session.

## What this is not

Packets are not embeddings, not a second source of truth besides git, and not a multi-user document store. They exist so engineering context can move between agents without pretending a whole chat log is the product.

## Team and project context

- **Project context (now):** local sessions scoped by agent project directories, plus `port-config` for instruction files.
- **Team context (planned):** shared packets across people and machines. See [For teams](/teams).
