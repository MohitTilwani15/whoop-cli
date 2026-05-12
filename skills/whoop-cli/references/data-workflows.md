# Data Workflows

Use this reference for common WHOOP data tasks.

## Privacy Default

WHOOP responses can contain health-adjacent personal data. For smoke tests or diagnostics, suppress response bodies:

```bash
whoop-cli user get --json >/dev/null
```

Only print full responses when the user asks to inspect the data.

## Profile And Body

```bash
whoop-cli user get --json
whoop-cli user body get --json
```

Use profile/body commands to confirm the token has profile and body scopes.

## Workouts

Start with a bounded list:

```bash
whoop-cli workouts list --limit 1 --json
```

Get one workout:

```bash
workout_id="$(whoop-cli workouts list --limit 1 --json | jq -r '.records[0].id // empty')"
whoop-cli workouts get "$workout_id" --json
```

For date windows:

```bash
whoop-cli workouts list \
  --start 2026-05-01T00:00:00Z \
  --end 2026-05-08T00:00:00Z \
  --limit 25 \
  --json
```

## Sleep

```bash
whoop-cli sleep list --limit 1 --json
sleep_id="$(whoop-cli sleep list --limit 1 --json | jq -r '.records[0].id // empty')"
whoop-cli sleep get "$sleep_id" --json
```

## Cycles

```bash
whoop-cli cycles list --limit 1 --json
cycle_id="$(whoop-cli cycles list --limit 1 --json | jq -r '.records[0].id // empty')"
whoop-cli cycles get "$cycle_id" --json
whoop-cli cycles sleep get "$cycle_id" --json
whoop-cli cycles recovery get "$cycle_id" --json
```

The nested cycle sleep/recovery commands may require the relevant sleep or recovery scopes.

## Recovery

```bash
whoop-cli recovery list --limit 1 --json
```

Recovery is a list resource in this CLI. Use cycle recovery for a specific cycle:

```bash
whoop-cli cycles recovery get "$cycle_id" --json
```

## Activity Mapping

Use mapping only when you have a v1 activity ID:

```bash
whoop-cli mapping get "$activity_v1_id" --json
```

If sampled workout data does not expose a `v1_id`, skip mapping tests rather than fabricating an ID.

## Working With `jq`

For live smoke tests, use `jq` to extract only IDs and suppress response bodies:

```bash
workout_id="$(whoop-cli workouts list --limit 1 --json | jq -r '.records[0].id // empty')"
if [ -n "$workout_id" ]; then
  whoop-cli workouts get "$workout_id" --json >/dev/null
fi
```

Do not persist complete live responses unless the user explicitly asks.
