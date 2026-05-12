# Commands

Use this reference for the stable command vocabulary. Always confirm the live surface with `whoop-cli agent-context`.

## Discovery

```bash
whoop-cli agent-context
whoop-cli version
```

From source:

```bash
go run . agent-context
go run . version
```

## Auth Commands

```bash
whoop-cli auth status --json
whoop-cli auth login --client-id "$WHOOP_CLIENT_ID" --redirect-uri http://localhost:8787/callback --print-url --json
whoop-cli auth login --client-id "$WHOOP_CLIENT_ID" --client-secret "$WHOOP_CLIENT_SECRET" --code "<code>" --json
whoop-cli auth refresh --client-id "$WHOOP_CLIENT_ID" --client-secret "$WHOOP_CLIENT_SECRET" --json
whoop-cli auth logout --json
whoop-cli auth revoke --force --dry-run --json
whoop-cli auth revoke --force --json
```

## User Commands

```bash
whoop-cli user get --json
whoop-cli user body get --json
```

## List Commands

List commands support:

- `--limit` with range `1..25`
- `--cursor`
- `--all`
- `--start`
- `--end`

Commands:

```bash
whoop-cli workouts list --limit 10 --json
whoop-cli sleep list --limit 10 --json
whoop-cli cycles list --limit 10 --json
whoop-cli recovery list --limit 10 --json
```

## Get Commands

Use IDs from list responses:

```bash
whoop-cli workouts get "<workout-id>" --json
whoop-cli sleep get "<sleep-id>" --json
whoop-cli cycles get "<cycle-id>" --json
whoop-cli cycles sleep get "<cycle-id>" --json
whoop-cli cycles recovery get "<cycle-id>" --json
whoop-cli mapping get "<activity-v1-id>" --json
```

IDs are path-escaped by the CLI. Still quote ID shell arguments.

## Feedback

```bash
whoop-cli feedback create "describe friction" --json
whoop-cli feedback list --json
```

Use `--config <temp-dir>` when testing feedback so real local feedback files are not touched.

## Update

```bash
whoop-cli update --check --json
whoop-cli update --json
whoop-cli update --version v0.1.1 --json
```

Use `--install-dir <dir>` in tests to avoid overwriting the running binary.
