# For teams

Vibeporter's direction is a **shared engineering context layer** between teams, projects, people, and coding agents.

Today the product is a **local, single-machine** CLI. Nothing below marked **Planned** is implemented. There is no organization account, SSO, or billing in this codebase.

## What you can do now

- Keep source sessions on each developer's machine.
- Build a context packet with `handoff` and deliver it into another local agent store.
- Search and list sessions on that same machine.
- Run `vibeporter serve` on loopback for a local UI.

If two people need the same packet, they must share the machine, copy the target session file, or use whatever their agent already syncs. Vibeporter does not sync for them. The portable flows are in [Scenarios](/scenarios).

## Planned

Each item is a product intent, not a shipping feature.

| Capability | Status | Notes |
|---|---|---|
| Shared project context across a team | **Planned** | One packet visible to more than one laptop without ad-hoc file copy |
| Access boundaries | **Planned** | Who can read which project context |
| Audit trail | **Planned** | Who created or delivered a packet, when |
| Retention | **Planned** | How long packets and source indexes live |
| Self-hosting | **Planned** | Private deployment of a team context service |
| Hosted team context | **Planned** | Optional managed hosting; see [Business](/business) |

Not planned in the near term as part of the open-source CLI: turning Vibeporter into a generic wiki, chat archive, or vector database.

## Workflow we want later

1. A developer or agent records decisions and constraints into project context.
2. A teammate or another agent pulls a packet within access rules.
3. Task handoff injects that packet into the tool they actually use.
4. Provenance and retention stay inspectable.

Until that exists, treat `handoff` as a **local delivery integration**, and treat this page as the B2B north star — not a feature checklist you can enable.
