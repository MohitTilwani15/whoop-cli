# whoop-pp-cli

Agent-native CLI for the WHOOP Developer API.

## Agent-native rules

- Use `--json` for data output. Do not use `--format=json`.
- Data goes to stdout; diagnostics and errors go to stderr.
- List commands are bounded by default and use `--limit`, `--cursor`, `--all`, `--start`, and `--end`.
- Destructive commands require `--force` and should support `--dry-run` as they mature.
- Run `whoop-pp-cli agent-context` before using the CLI programmatically.
- Errors are structured JSON and include valid ranges/values where possible.

## Current milestone

This repo has the first working implementation:

- `agent-context` with versioned machine-readable CLI shape.
- `auth login` with WHOOP authorization URL generation, localhost callback capture, token exchange, and local token storage.
- WHOOP API client path for `user get` and `user body get` using bearer token auth.
- Real paginated list support for workouts/sleep/cycles/recovery.
- Get support for workouts, sleep, cycles, cycle sleep, cycle recovery, and v1 activity mapping.
- Destructive `auth revoke` guarded by `--force`.
- Local `feedback create/list` JSONL loop.
- Schema file that codifies vocabulary and endpoint coverage.
- 100% statement coverage in tests.

## Usage

```bash
go run . agent-context
go run . auth login --client-id "$WHOOP_CLIENT_ID" --client-secret "$WHOOP_CLIENT_SECRET" --redirect-uri http://localhost:8787/callback --json
WHOOP_ACCESS_TOKEN=... go run . user get --json
WHOOP_ACCESS_TOKEN=... go run . user body get --json
go run . workouts list --limit 10 --json
go run . sleep list --all --json
go run . mapping get 12345678 --json
go run . auth revoke --force --json
go run . feedback create "error should enumerate values" --json
go run . feedback list --json
```

## Test

```bash
go test ./...
```

## Next implementation steps

1. Implement OAuth refresh with WHOOP refresh-token rotation.
2. Add profile save/list/get/delete.
3. Move schema to the command generator instead of duplicating command metadata in Go.
4. Add cache and sync job ledger.
5. Add `--deliver` for exports.
