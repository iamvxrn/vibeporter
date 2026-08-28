<p align="center">
  <img src="website/docs/public/social.png" alt="vibeporter — Migrate chats between agents" width="720">
</p>

# Vibeporter

A CLI utility for migrating chat histories and configuration files between different agent interfaces.

[![Documentation](https://img.shields.io/badge/docs-vibeporter.pages.dev-8b5cf6.svg)](https://vibeporter.pages.dev)
[![OS Matrix](https://img.shields.io/badge/OS-Linux%20%7C%20macOS%20%7C%20Windows-8b5cf6.svg)](#)

## Features

- Converts chat histories from JSONL or SQLite into a standard format.
- Supports Claude Code, OpenCode, Gemini CLI, Kimi Code, and DeepSeek Harness (read and write).
- Copies configuration files (e.g., `CLAUDE.md` to `GEMINI.md`).

## Quick Start

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

vibeporter migrate --from claudecode --to gemini \
  --source <chat-id-from-list>

vibeporter port-config --from claudecode --to gemini --dir .
```

## Documentation

[https://vibeporter.pages.dev](https://vibeporter.pages.dev)

## License

MIT
