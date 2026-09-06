# Vibeporter

Vibeporter preserves engineering context locally and makes it portable across AI agents, developers, teams, and projects.

It is a local CLI: no cloud account, no telemetry. A **context packet** is the unit of transfer. Task handoff and chat adapters are integrations that deliver that packet into another coding agent.

<div style="display: flex; gap: 12px; margin-top: 1.5rem; margin-bottom: 1.5rem; flex-wrap: wrap; align-items: center;">
  <a href="/quickstart" style="background-color: var(--vp-button-brand-bg); color: var(--vp-button-brand-text); padding: 8px 16px; border-radius: 8px; text-decoration: none; font-weight: 600; font-size: 14px; transition: background-color 0.2s;">Quickstart</a>
  <a href="https://github.com/iamvxrn/vibeporter" style="background-color: var(--vp-button-alt-bg); color: var(--vp-button-alt-text); padding: 8px 16px; border-radius: 8px; text-decoration: none; font-weight: 600; font-size: 14px; border: 1px solid var(--vp-button-alt-border); transition: background-color 0.2s;">GitHub</a>
  
  <div style="background-color: #161618; border: 1px solid #3c3f44; border-radius: 8px; padding: 6px 12px; display: flex; align-items: center; gap: 8px; font-family: monospace; font-size: 13px;">
    <span style="color: #8b949e;">$</span> curl -fsSL https://vibeporter.pages.dev/install.sh | sh
    <button style="background: #2c2e33; color: #c9d1d9; border: none; padding: 4px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; margin-left: 8px;">Copy</button>
  </div>
</div>

<div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 2rem;">
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Portable context</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Context packets</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Local only</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Claude Code</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">OpenCode</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Gemini CLI</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Cursor</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">No telemetry</span>
</div>

[Install options →](/install) · [Scenarios →](/scenarios) · [Context model →](/context)

---

## Why this exists

Agent sessions hold the real working memory of a task: what was decided, what is true right now, what must not change, what is still unknown. That memory dies when the window closes, when you switch tools, or when a teammate starts a new clone of the repo.

Vibeporter keeps that layer local and portable. You select useful context, record provenance, and deliver a packet — instead of pasting a transcript and hoping.

## Context packet

The packet is the product unit. Source sessions are inputs. Native agent sessions are one delivery channel. The three team-facing flows are in [Scenarios](/scenarios).

```bash
vibeporter list claudecode

vibeporter handoff --from claudecode --to opencode \
  --source <session-id-from-list> --compact 200k
```

JSON for each created packet is written under `~/.vibeporter/handoffs/`.

## Local, on purpose

There is no Vibeporter cloud and no account. If you run `vibeporter serve`, it binds to loopback on your machine. Team-wide shared context is [planned](/teams), not shipping in this release.

## Integrations

Need a raw copy of a session, a Markdown export, or `CLAUDE.md` → `GEMINI.md`? Those are [integrations](/integrations), including task handoff.
