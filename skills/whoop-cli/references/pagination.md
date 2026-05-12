# Pagination

Use this reference for list commands, cursors, and rate-safe iteration.

## Defaults

List commands are bounded by default. The CLI uses:

- `--limit`, max 25
- `--cursor`
- `--all`
- `--start`
- `--end`

Default to small limits in tests:

```bash
whoop-cli workouts list --limit 1 --json
```

Use max page size for normal bounded retrieval:

```bash
whoop-cli workouts list --limit 25 --json
```

## Cursor Flow

A truncated list response includes `next_cursor`:

```json
{
  "records": [],
  "next_cursor": "cursor-value",
  "truncated": true,
  "limit": 1
}
```

Fetch the next page:

```bash
whoop-cli workouts list --limit 1 --cursor "$next_cursor" --json
```

## Date Windows

Prefer date windows for user requests like "last week" or "this month":

```bash
whoop-cli sleep list \
  --start 2026-05-01T00:00:00Z \
  --end 2026-05-08T00:00:00Z \
  --limit 25 \
  --json
```

Use exact ISO-8601 timestamps. If the user uses relative dates, resolve them to concrete dates before running commands.

## `--all`

Use `--all` only when the user explicitly asks for all available data or when the time range is tightly bounded:

```bash
whoop-cli workouts list --start 2026-05-01T00:00:00Z --end 2026-05-02T00:00:00Z --all --json
```

Avoid unbounded `--all` in automated tests.

## Rate Safety

If the CLI returns a `whoop_api_error` with `retry_after`, stop and report the wait hint. Do not loop aggressively.

For test suites:

- Use `--limit 1`.
- Run destructive auth tests last.
- Avoid repeated `--all`.
