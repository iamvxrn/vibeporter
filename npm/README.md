# vibeporter

Thin npm wrapper around the [Vibeporter](https://vibeporter.pages.dev) CLI. Vibeporter preserves engineering context locally and makes it portable across AI agents, developers, teams, and projects. On first run this package downloads the matching GitHub Release binary and caches it.

The primary install is still the shell script:

```bash
curl -fsSL https://vibeporter.pages.dev/install.sh | sh
```

npm is an alternative:

```bash
npx vibeporter@latest handoff --from claudecode --source <id> --to opencode --compact 200k
```
