# Quickstart

Use this reference for installation, first-run checks, and deciding whether to run the release binary or local source.

## Choose The Execution Mode

Use the installed binary when:

- The user wants to use WHOOP data.
- The task is about authentication, update, or end-user behavior.
- You are not actively changing source code.

Use local source from the repo root when:

- The user asked to modify or test the codebase.
- You need to verify behavior before committing.
- You are testing unreleased changes.

Commands:

```bash
whoop-cli version
whoop-cli agent-context
```

From source:

```bash
go run . version
go run . agent-context
```

## Install

The release installer downloads a GitHub Release archive, verifies `checksums.txt`, and installs `whoop-cli` to `~/.local/bin` by default.

```bash
curl -fsSL https://raw.githubusercontent.com/mohittilwani/whoop-cli/main/install.sh | sh
```

Install a specific version:

```bash
WHOOP_CLI_VERSION=v0.1.1 curl -fsSL https://raw.githubusercontent.com/mohittilwani/whoop-cli/main/install.sh | sh
```

Install to a custom directory:

```bash
WHOOP_CLI_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/mohittilwani/whoop-cli/main/install.sh | sh
```

Developer install:

```bash
go install github.com/mohittilwani/whoop-cli@latest
```

## First Checks

Run:

```bash
whoop-cli version
whoop-cli agent-context
whoop-cli auth status --json
```

If `whoop-cli` is not found, check whether `~/.local/bin` is on `PATH`:

```bash
printf '%s\n' "$PATH"
```

If using local source, run:

```bash
go test ./...
go run . auth status --json
```

## Important Conventions

- Use `--json` for data commands.
- Use bounded list calls first, for example `--limit 1`.
- Prefer `--config <dir>` for tests that should not touch the user's real token.
- Use `--api-base <url>` only for local fixtures or mock servers.
