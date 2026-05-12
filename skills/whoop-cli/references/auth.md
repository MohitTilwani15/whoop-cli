# Auth

Use this reference for OAuth login, callback URL handling, token refresh, logout, and revocation.

## Token Sources

The CLI looks for an access token in this order:

1. Test/runtime environment passed by the caller.
2. `WHOOP_ACCESS_TOKEN`.
3. Saved token file under `~/.config/whoop-cli/token.json`, or `--config <dir>/token.json`.

Do not print token values. Do not commit token files.

## OAuth Browser Flow

The authorization URL only needs:

- client ID
- redirect URI
- scopes
- state

The client secret is needed later when exchanging the callback code.

Generate a URL:

```bash
whoop-cli auth login \
  --client-id "$WHOOP_CLIENT_ID" \
  --redirect-uri http://localhost:8787/callback \
  --print-url \
  --json
```

The user opens the URL, logs in, and copies the redirected URL. It usually looks like:

```text
http://localhost:8787/callback?code=...&scope=...&state=abcdefgh
```

Before exchanging, verify the callback `state` matches the generated state. If it does not match, stop and generate a fresh authorization URL.

Exchange the callback code:

```bash
whoop-cli auth login \
  --client-id "$WHOOP_CLIENT_ID" \
  --client-secret "$WHOOP_CLIENT_SECRET" \
  --redirect-uri http://localhost:8787/callback \
  --code "<callback-code>" \
  --json
```

If the secret is in an env file, source it without printing it:

```bash
set -a
. /private/tmp/whoop-token.env
set +a
whoop-cli auth login --client-id "$WHOOP_CLIENT_ID" --code "<code>" --json
```

## Local Callback Capture

If running interactively on the user's machine, the CLI can listen on localhost and open the browser:

```bash
whoop-cli auth login \
  --client-id "$WHOOP_CLIENT_ID" \
  --client-secret "$WHOOP_CLIENT_SECRET" \
  --redirect-uri http://localhost:8787/callback \
  --json
```

Use `--no-browser` when another process opens the URL.

## Auth Status

```bash
whoop-cli auth status --json
```

Expected unauthenticated shape:

```json
{
  "authenticated": false,
  "hint": "Run whoop-cli auth login"
}
```

## Refresh

Refresh requires a saved refresh token and client credentials:

```bash
whoop-cli auth refresh \
  --client-id "$WHOOP_CLIENT_ID" \
  --client-secret "$WHOOP_CLIENT_SECRET" \
  --json
```

Refresh rotates the saved token file. If refresh fails because the refresh token is missing, run login again with the default `offline` scope.

## Logout vs Revoke

`logout` deletes only local saved token state:

```bash
whoop-cli auth logout --json
```

`revoke` calls WHOOP and invalidates the token:

```bash
whoop-cli auth revoke --force --json
```

Safe dry run:

```bash
whoop-cli auth revoke --force --dry-run --json
```

Run real revoke last in live test sessions because it prevents additional data calls until a new login.

## Common Auth Failures

- `missing_oauth_credentials`: provide `--client-id`, `--client-secret`, or matching env vars.
- `oauth_callback_error`: state mismatch, missing code, occupied local port, or timeout.
- `oauth_token_error`: callback code expired, wrong secret, wrong redirect URI, or WHOOP rejected the token request.
- `refresh_token_missing`: run login again and ensure `offline` scope is included.
