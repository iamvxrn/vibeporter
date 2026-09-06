# Integrations

These commands and adapters **deliver** engineering context. They are not the product headline.

## Task handoff

`vibeporter handoff` selects local context to a budget and writes a **new** native session. That write is a context packet delivery (`native_session`), with packet JSON under `~/.vibeporter/handoffs/`.

```bash
vibeporter handoff --from claudecode --source abc123 --to opencode --compact 200k
```

See [CLI](/cli#handoff) and [Context model](/context).

## Source sessions (adapters)

Each coding agent has an extractor (read) and, where supported, an injector (write). The shared IR is documented in [Overview](/overview). Per-agent pages:

- [Claude Code](/claudecode)
- [OpenCode](/opencode)
- [Gemini CLI](/gemini)
- [Kimi Code](/kimicode)
- [DeepSeek Harness](/dsh)
- [Cursor](/cursor)

`list`, `search`, and `stats` operate on these stores. They are how you find **sources**, not a chat-converter product of their own.

## Raw session copy

Un-compacted transfer of a full source session into another agent format. Use when you need fidelity over a budgeted packet. The CLI name is still `migrate` so existing scripts keep working.

```bash
vibeporter migrate --from claudecode --to opencode --source <id>
```

`diff` is the dry run for this path.

## Export

Render a source session as Markdown or HTML. Sharing a document is not the same as a handoff into an agent.

```bash
vibeporter export --from claudecode --source <id> --format markdown --output context.md
```

## Config porting

Copy project instruction and ignore files between agent conventions (`CLAUDE.md` → `GEMINI.md`, …). Details: [Config porting](/config-porting).

```bash
vibeporter port-config --from claudecode --to gemini --dir .
```
