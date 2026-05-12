# Privacy And Safety

Use this reference whenever live tokens, callback URLs, env files, or WHOOP response bodies are involved.

## Sensitive Values

Treat these as sensitive:

- `WHOOP_CLIENT_SECRET`
- access tokens
- refresh tokens
- callback URLs containing `code=...`
- `~/.config/whoop-cli/token.json`
- `/private/tmp/*token*.env` files
- full live WHOOP profile/body/sleep/recovery/cycle/workout responses

Do not paste these into summaries, commits, README examples, test fixtures, or issue text.

## Git Safety

Before committing:

```bash
git status --short
git diff --cached --stat
```

Confirm no token files or env files are staged. Expected code/doc/test files are fine; files under home config or `/private/tmp` should never be staged.

If a token file appears in the repo, stop and ask before doing anything destructive. Do not silently delete user files outside the repo.

## Live Test Output

Suppress full response bodies by default:

```bash
whoop-cli user get --json >/dev/null
whoop-cli workouts list --limit 1 --json >/dev/null
```

When extracting IDs:

```bash
workout_id="$(whoop-cli workouts list --limit 1 --json | jq -r '.records[0].id // empty')"
```

The ID itself is less sensitive than a full health-data payload, but still avoid logging more than needed.

## Auth Destructive Actions

`auth logout` deletes local token state only.

`auth revoke --force` invalidates the token at WHOOP and deletes local token state.

Use dry-run first:

```bash
whoop-cli auth revoke --force --dry-run --json
```

Only run real revoke if the user explicitly asks or confirms.

## Secret Files

If the user provides a path like `/private/tmp/whoop-token.env`, inspect only variable names or file existence unless you need to source it:

```bash
cut -d= -f1 /private/tmp/whoop-token.env
```

When sourcing:

```bash
set -a
. /private/tmp/whoop-token.env
set +a
```

Do not print the environment afterward.
