# whoop-cli

Agent-native CLI for the WHOOP Developer API.

## Agent-native rules

- Use `--json` for data output. Do not use `--format=json`.
- Data goes to stdout; diagnostics and errors go to stderr.
- List commands are bounded by default and use `--limit`, `--cursor`, `--all`, `--start`, and `--end`.
- Destructive commands require `--force` and should support `--dry-run` as they mature.
- Run `whoop-cli agent-context` before using the CLI programmatically.
- Errors are structured JSON and include valid ranges/values where possible.

## Current milestone

This repo has the first working implementation:

- `agent-context` with versioned machine-readable CLI shape.
- `auth login` with WHOOP authorization URL generation, localhost callback capture, token exchange, and local token storage.
- `auth status`, `auth refresh`, `auth logout`, and real WHOOP token revocation through `DELETE /v2/user/access`.
- WHOOP API client path for `user get` and `user body get` using bearer token auth.
- Real paginated list support for workouts/sleep/cycles/recovery.
- Get support for workouts, sleep, cycles, cycle sleep, cycle recovery, and v1 activity mapping.
- Destructive `auth revoke` guarded by `--force` and supporting `--dry-run`.
- Local `feedback create/list` JSONL loop.
- Schema file that codifies vocabulary and endpoint coverage.
- Broad statement coverage in tests.

## Install

Install the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/mohittilwani/whoop-cli/main/install.sh | sh
```

By default, the installer downloads the matching GitHub Release binary, verifies `checksums.txt`, and installs `whoop-cli` to `~/.local/bin`.

Options:

```bash
WHOOP_CLI_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/mohittilwani/whoop-cli/main/install.sh | sh
WHOOP_CLI_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/mohittilwani/whoop-cli/main/install.sh | sh
```

Developer install with Go:

```bash
go install github.com/mohittilwani/whoop-cli@latest
```

Update an existing release install:

```bash
whoop-cli update --check --json
whoop-cli update --json
```

## Release

Create and push a version tag to publish release binaries:

```bash
git tag v0.1.1
git push origin v0.1.1
```

The release workflow builds macOS, Linux, and Windows archives, injects the tag into `whoop-cli version`, generates `checksums.txt`, and publishes a GitHub Release. The one-line installer and `whoop-cli update` both install from the latest GitHub Release.

## Usage

```bash
go run . agent-context
go run . auth login --client-id "$WHOOP_CLIENT_ID" --client-secret "$WHOOP_CLIENT_SECRET" --redirect-uri http://localhost:8787/callback --json
WHOOP_ACCESS_TOKEN=... go run . user get --json
WHOOP_ACCESS_TOKEN=... go run . user body get --json
go run . workouts list --limit 10 --json
go run . sleep list --all --json
go run . mapping get 12345678 --json
go run . auth status --json
go run . auth refresh --client-id "$WHOOP_CLIENT_ID" --client-secret "$WHOOP_CLIENT_SECRET" --json
go run . auth logout --json
go run . auth revoke --force --json
go run . feedback create "error should enumerate values" --json
go run . feedback list --json
```

## Test

```bash
go test ./...
```

## Next implementation steps

1. Add profile save/list/get/delete.
2. Move schema to the command generator instead of duplicating command metadata in Go.
3. Add cache and sync job ledger.
4. Add `--deliver` for exports.
