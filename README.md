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
curl -fsSL https://raw.githubusercontent.com/MohitTilwani15/whoop-cli/main/install.sh | sh
```

By default, the installer downloads the matching GitHub Release binary, verifies `checksums.txt`, and installs `whoop-cli` to `~/.local/bin`.

Options:

```bash
WHOOP_CLI_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/MohitTilwani15/whoop-cli/main/install.sh | sh
WHOOP_CLI_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/MohitTilwani15/whoop-cli/main/install.sh | sh
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

## Create a WHOOP Developer App

Before `whoop-cli` can access WHOOP data, create an app in the [WHOOP Developer Dashboard](https://developer-dashboard.whoop.com/). WHOOP's docs describe this as the place to create apps, manage client credentials, configure scopes, and register redirect URIs.

1. Sign in at [developer.whoop.com](https://developer.whoop.com/) and open **Dashboard**.
2. Create a team if prompted.
3. Create an app.
4. Select the scopes this CLI should request. For full current CLI coverage, use:
   ```text
   read:profile read:body_measurement read:cycles read:recovery read:sleep read:workout offline
   ```
   Request fewer scopes if you only need a subset of commands.
5. Add this redirect URI to the app:
   ```text
   http://localhost:8787/callback
   ```
   The redirect URI in the OAuth request must exactly match one registered in the dashboard.
6. Copy the app's **Client ID** and **Client Secret**. Keep the secret private; do not commit it, log it, or put it in frontend/mobile code.

Set credentials in your shell:

```bash
export WHOOP_CLIENT_ID="your-client-id"
export WHOOP_CLIENT_SECRET="your-client-secret"
```

Start OAuth login:

```bash
whoop-cli auth login \
  --client-id "$WHOOP_CLIENT_ID" \
  --client-secret "$WHOOP_CLIENT_SECRET" \
  --redirect-uri http://localhost:8787/callback \
  --json
```

If the browser callback cannot be captured automatically, generate a URL manually:

```bash
whoop-cli auth login \
  --client-id "$WHOOP_CLIENT_ID" \
  --redirect-uri http://localhost:8787/callback \
  --print-url \
  --json
```

Open the URL, sign in, then copy the redirected `localhost` URL. Exchange the `code` from that URL:

```bash
whoop-cli auth login \
  --client-id "$WHOOP_CLIENT_ID" \
  --client-secret "$WHOOP_CLIENT_SECRET" \
  --redirect-uri http://localhost:8787/callback \
  --code "<code-from-callback-url>" \
  --json
```

Verify and make a small request:

```bash
whoop-cli auth status --json
whoop-cli user get --json
whoop-cli workouts list --limit 1 --json
```

## Release

Create and push a version tag to publish release binaries:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds macOS, Linux, and Windows archives, injects the tag into `whoop-cli version`, generates `checksums.txt`, and publishes a GitHub Release. The one-line installer and `whoop-cli update` both install from the latest GitHub Release.

## Agent Skill

The canonical skill lives in `skills/whoop-cli`. Repo-local discovery paths are symlinked to that source:

```text
.claude/skills/whoop-cli -> ../../skills/whoop-cli
.agents/skills/whoop-cli -> ../../skills/whoop-cli
```

Edit only `skills/whoop-cli` so the skill does not drift across agent runtimes.

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
