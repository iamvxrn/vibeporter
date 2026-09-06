# Config Porting

Config porting is an **integration**: it copies project instruction files so another agent can load the same project context. It is not a chat converter.

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

`cursor` is included here as a filename mapping. Session extract/inject is a separate adapter (`vibeporter list cursor`, `handoff --to cursor`).
