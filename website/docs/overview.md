# Overview

Vibeporter is a CLI tool that migrates chat histories and workspace configurations between AI coding agents. Switched editors? Trying a new agent? Take your conversations with you — no vendor lock-in.

## Why Vibeporter?

- **No lock-in:** Your conversations belong to you. Move them between agents freely.
- **Single binary:** Compiled Go with zero runtime dependencies. Pure Go SQLite (no CGO), so it cross-compiles anywhere.
- **One common format:** Every agent is described by a small adapter that reads into — and writes out of — a shared intermediate representation. Adding a new agent is just one adapter, not N×N converters.
- **Agent-friendly:** Designed to be invoked by AI agents themselves, not just humans.

## How it works

Vibeporter never converts one agent's format directly into another's. Instead, each agent has an **extractor** (reads its native format) and an **injector** (writes it). Both sides speak a common intermediate representation (IR), so any source can reach any target through one hop.

```mermaid
graph LR
    A[Claude Code] -->|Extractor| IR((Common Format))
    B[OpenCode] -->|Extractor| IR
    C[Gemini CLI] -->|Extractor| IR
    IR -->|Injector| D[Gemini CLI]
    IR -->|Injector| E[Other agents]
```

Today Claude Code and OpenCode can be read, and Gemini CLI can be both read and written. As more injectors land, every existing extractor gets those targets for free.
