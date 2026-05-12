# Errors

Use this reference for structured errors, exit codes, and troubleshooting.

## Error Shape

Errors are JSON on stderr:

```json
{
  "error": {
    "code": "invalid_flag_value",
    "message": "--limit must be between 1 and 25",
    "flag": "--limit",
    "got": 100,
    "valid_range": {
      "min": 1,
      "max": 25
    },
    "example": "whoop-cli workouts list --limit 25 --json"
  }
}
```

Do not scrape human text when a structured field exists.

## Exit Codes

Current conventions:

- `0`: success
- `1`: IO, callback, or update failure
- `2`: usage or invalid invocation
- `3`: auth/configuration failure
- `4`: OAuth or WHOOP client/upstream error
- `5`: invalid JSON or server-like failure
- `6`: rate limit
- `7`: network error

## Common Problems

### Missing Auth

Error code: `auth_missing`

Fix:

```bash
whoop-cli auth status --json
whoop-cli auth login --client-id "$WHOOP_CLIENT_ID" --redirect-uri http://localhost:8787/callback --print-url --json
```

### Invalid Limit

Error code: `invalid_flag_value`

Fix:

```bash
whoop-cli workouts list --limit 25 --json
```

### Network Failure

Error code: `network_error`

Check:

- DNS/network availability.
- Whether sandboxed execution blocks network.
- `--timeout` and `--retries` values.

Example:

```bash
whoop-cli user get --timeout 30s --retries 2 --json
```

### Rate Limit

Exit code: `6`

If `retry_after` is present, respect it. Avoid repeated immediate retries.

### Update Failure

Common causes:

- No GitHub Release exists yet.
- Release assets do not match the current OS/arch.
- `checksums.txt` is missing or does not include the archive.
- Checksum mismatch.
- Install directory is not writable.

Use:

```bash
whoop-cli update --check --json
whoop-cli update --install-dir "$HOME/.local/bin" --json
```
