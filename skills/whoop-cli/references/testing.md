# Testing

Use this reference for local verification and live WHOOP smoke tests.

## Local Code Tests

From the repo root:

```bash
go test ./...
go vet ./...
git diff --check
```

For release/update changes, also check:

```bash
sh -n install.sh
```

## Local CLI Smoke Tests

Use an isolated config directory for local-only tests:

```bash
tmp="$(mktemp -d)"
go run . auth status --config "$tmp" --json
go run . auth revoke --force --dry-run --config "$tmp" --json
go run . feedback create "smoke test" --config "$tmp" --json
go run . feedback list --config "$tmp" --json
```

Validate structured errors:

```bash
go run . workouts list --limit 100 --config "$tmp" --json
go run . user get --config "$tmp" --json
```

The first should reject the limit. The second should report missing auth.

## OAuth Live Test

1. Generate authorization URL:
   ```bash
   go run . auth login \
     --client-id "$WHOOP_CLIENT_ID" \
     --redirect-uri http://localhost:8787/callback \
     --print-url \
     --json
   ```

2. User logs in and returns callback URL.

3. Verify `state` matches.

4. Exchange code:
   ```bash
   go run . auth login \
     --client-id "$WHOOP_CLIENT_ID" \
     --client-secret "$WHOOP_CLIENT_SECRET" \
     --redirect-uri http://localhost:8787/callback \
     --code "<code>" \
     --json
   ```

5. Confirm auth:
   ```bash
   go run . auth status --json
   ```

## Live Endpoint Smoke Test

Suppress bodies:

```bash
go run . user get --json >/dev/null
go run . user body get --json >/dev/null
go run . workouts list --limit 1 --json >/dev/null
go run . sleep list --limit 1 --json >/dev/null
go run . cycles list --limit 1 --json >/dev/null
go run . recovery list --limit 1 --json >/dev/null
```

ID-based checks:

```bash
workout_id="$(go run . workouts list --limit 1 --json | jq -r '.records[0].id // empty')"
if [ -n "$workout_id" ]; then go run . workouts get "$workout_id" --json >/dev/null; fi

sleep_id="$(go run . sleep list --limit 1 --json | jq -r '.records[0].id // empty')"
if [ -n "$sleep_id" ]; then go run . sleep get "$sleep_id" --json >/dev/null; fi

cycle_id="$(go run . cycles list --limit 1 --json | jq -r '.records[0].id // empty')"
if [ -n "$cycle_id" ]; then
  go run . cycles get "$cycle_id" --json >/dev/null
  go run . cycles sleep get "$cycle_id" --json >/dev/null
  go run . cycles recovery get "$cycle_id" --json >/dev/null
fi
```

Mapping check only when a real v1 activity ID exists:

```bash
v1_id="$(go run . workouts list --limit 1 --json | jq -r '.records[0].v1_id // empty')"
if [ -n "$v1_id" ]; then go run . mapping get "$v1_id" --json >/dev/null; fi
```

## Pagination Smoke Test

```bash
cursor="$(go run . workouts list --limit 1 --json | jq -r '.next_cursor // empty')"
if [ -n "$cursor" ]; then go run . workouts list --limit 1 --cursor "$cursor" --json >/dev/null; fi
```

Repeat for sleep, cycles, and recovery if needed.

## Destructive Auth Tests

Run last:

```bash
go run . auth logout --config "$tmp" --json
go run . auth revoke --force --dry-run --json
```

Only run real revoke with explicit user approval:

```bash
go run . auth revoke --force --json
```
