# vibeporter

Thin npm wrapper around the [Vibeporter](https://vibeporter.pages.dev) Go CLI. On first run it downloads the matching GitHub Release binary and caches it.

The primary install is still the shell script:

```bash
curl -fsSL https://vibeporter.pages.dev/install.sh | sh
```

npm is an alternative:

```bash
npx vibeporter@latest list claudecode
```
