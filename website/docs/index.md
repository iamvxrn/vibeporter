# Vibeporter: local context handoff

vibeporter is a single Go CLI for handing off long AI conversations into fresh native sessions. It selects useful local context to a budget, retains task intent and recent progress, and writes to another agent without cloud services or external accounts.

<div style="display: flex; gap: 12px; margin-top: 1.5rem; margin-bottom: 1.5rem; flex-wrap: wrap; align-items: center;">
  <a href="/quickstart" style="background-color: var(--vp-button-brand-bg); color: var(--vp-button-brand-text); padding: 8px 16px; border-radius: 8px; text-decoration: none; font-weight: 600; font-size: 14px; transition: background-color 0.2s;">Quickstart</a>
  <a href="https://github.com/iamvxrn/vibeporter" style="background-color: var(--vp-button-alt-bg); color: var(--vp-button-alt-text); padding: 8px 16px; border-radius: 8px; text-decoration: none; font-weight: 600; font-size: 14px; border: 1px solid var(--vp-button-alt-border); transition: background-color 0.2s;">GitHub</a>
  
  <div style="background-color: #161618; border: 1px solid #3c3f44; border-radius: 8px; padding: 6px 12px; display: flex; align-items: center; gap: 8px; font-family: monospace; font-size: 13px;">
    <span style="color: #8b949e;">$</span> curl -fsSL https://vibeporter.pages.dev/install.sh | sh
    <button style="background: #2c2e33; color: #c9d1d9; border: none; padding: 4px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; margin-left: 8px;">Copy</button>
  </div>
</div>

<div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 2rem;">
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Gemini CLI</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Claude Code</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">OpenCode</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Kimi Code</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Cursor</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Pure Go</span>
  <span style="background-color: #202127; color: #a1a1aa; padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: 500;">Zero Dependencies</span>
</div>

[Other install options →](/install)

---

## Try it

After you install Vibeporter, everything is a one-liner.

```bash
# List all your Claude Code conversations
vibeporter list claudecode

# Hand off a compact context to OpenCode
vibeporter handoff --from claudecode --to opencode \
  --source <chat-id-from-list> --compact 200k

# Port your project configs (CLAUDE.md → GEMINI.md)
vibeporter port-config --from claudecode --to gemini --dir .
```
