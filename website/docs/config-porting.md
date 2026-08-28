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

| Agent | Instruction file | Ignore file |
|---|---|---|
| Claude Code (`claudecode`) | `CLAUDE.md` | `.claudeignore` |
| Gemini CLI (`gemini`) | `GEMINI.md` | `.geminiignore` |
| Cursor (`cursor`) | `.cursorrules` | `.cursorignore` |
| OpenCode (`opencode`) | `OPENCODE.md` | `.opencodeignore` |
| Kimi Code (`kimicode` / `kimi`) | `AGENTS.md` | — |

Any of these agents can be `--from` or `--to`. Matching files are copied (instruction file to instruction file, ignore file to ignore file). Agents without an ignore file skip that copy. Existing target files are never overwritten.

`cursor` is included here as a filename mapping. Chat extract/list is separate (`vibeporter list cursor`); there is no Cursor inject.
