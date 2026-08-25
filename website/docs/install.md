# Installation

Install Vibeporter using the shell script:

```bash
curl -fsSL https://vibeporter.pages.dev/install.sh | sh
```

This downloads the binary for your OS and puts it in `~/.local/bin/vibeporter`.

### Check your PATH

Make sure your shell can find the `vibeporter` command:

```bash
which vibeporter
```

If it's not found, add this to your `~/.bashrc` or `~/.zshrc`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Manual Install

Don't want to use `curl | sh`? Just grab the binary from the [GitHub Releases](https://github.com/iamvxrn/vibeporter/releases) page and put it in your path manually.

## Build from Source

```bash
git clone https://github.com/iamvxrn/vibeporter.git
cd vibeporter
go build -o vibeporter
```
