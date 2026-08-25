# Config Porting

Vibeporter can migrate project-level configuration files between AI coding agents. These are the files that define agent behavior, ignore patterns, and project instructions.

## How it works

Run `port-config` in your project's root directory:

```bash
vibeporter port-config --from claudecode --to gemini --dir .
```

Vibeporter will:
1. Look for source agent config files (e.g. `CLAUDE.md`, `.claudeignore`).
2. Copy them to the target agent's expected filenames (e.g. `GEMINI.md`, `.geminiignore`).
3. Skip any files that already exist in the target to avoid overwriting your work.

## Mapping Table

| Source Agent | Source Files | Target Files |
|---|---|---|
| Claude Code | `CLAUDE.md`, `.claudeignore` | `GEMINI.md`, `.geminiignore` |
| Cursor | `.cursorrules`, `.cursorignore` | `GEMINI.md`, `.geminiignore` |
| OpenCode | `OPENCODE.md`, `.opencodeignore` | `GEMINI.md`, `.geminiignore` |
