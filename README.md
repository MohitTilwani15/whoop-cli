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

This repo has the first working skeleton:

- `agent-context` with versioned machine-readable CLI shape.
- WHOOP API client path for `user get` and `user body get` using bearer token auth.
- Agent-native list behavior skeleton for workouts/sleep/cycles/recovery.
- Destructive `auth revoke` guarded by `--force`.
- Local `feedback create/list` JSONL loop.
- Schema file that codifies vocabulary and endpoint coverage.
- Tests for the above.

## Usage

```bash
go run . agent-context
WHOOP_ACCESS_TOKEN=... go run . user get --json
WHOOP_ACCESS_TOKEN=... go run . user body get --json
go run . workouts list --limit 10 --json
go run . auth revoke --force --json
go run . feedback create "error should enumerate values" --json
go run . feedback list --json
```

## Test

```bash
go test ./...
```

## Next implementation steps

1. Move schema to the command generator instead of duplicating command metadata in Go.
2. Implement OAuth authorization-code login and refresh.
3. Implement real paginated API calls for cycles/recovery/sleep/workouts.
4. Add profile save/list/get/delete.
5. Add cache and sync job ledger.
6. Add `--deliver` for exports.
