---
name: whoop-cli
description: Use whoop-cli to install, authenticate, test, update, debug, distribute, or fetch WHOOP profile, body, sleep, recovery, cycles, workouts, and activity-mapping data with agent-native JSON output. Use this skill whenever the user mentions WHOOP data, whoop-cli, OAuth callback URLs, access/refresh tokens, release installs, CLI updates, smoke testing, or GitHub Release distribution, even if they do not explicitly ask for a skill.
version: 0.2.0
---

# whoop-cli

This skill helps agents operate the local `whoop-cli` safely and predictably.

## First Principles

- Prefer the installed `whoop-cli` binary when the user is asking as an end user.
- Prefer `go run .` from the repo root when testing local source changes.
- Run `whoop-cli agent-context` or `go run . agent-context` before programmatic use; treat it as the command surface source of truth.
- Use `--json` for data commands. Data goes to stdout; diagnostics/errors go to stderr.
- Use canonical verbs: `get`, `list`, `create`, `update`, `delete`. Do not invent aliases like `ls`, `info`, or `--format=json`.
- Bound list commands with `--limit`, `--start`, and `--end` unless the user explicitly asks for all data.
- Handle WHOOP output as health-adjacent personal data. Avoid printing full live response bodies unless the user explicitly asks.
- Never commit token files, callback URLs containing `code=...`, env files containing secrets, or full health-data response dumps.

## Reference Index

Load only the reference file needed for the task:

| Task | Read |
| --- | --- |
| Installing, first run, local source vs release binary | `references/quickstart.md` |
| OAuth login, callback URL exchange, refresh, logout, revoke | `references/auth.md` |
| Command inventory and flags | `references/commands.md` |
| Profile/body/workout/sleep/cycle/recovery workflows | `references/data-workflows.md` |
| Pagination, cursors, time windows, rate-safe listing | `references/pagination.md` |
| Structured errors, exit codes, troubleshooting | `references/errors.md` |
| Secrets, health-data privacy, git safety | `references/privacy.md` |
| Local and live smoke testing | `references/testing.md` |
| One-line installer, update command, releases | `references/distribution.md` |

## Default Workflow

1. Establish context:
   ```bash
   whoop-cli agent-context
   ```
   In the repo, use:
   ```bash
   go run . agent-context
   ```

2. Check auth without exposing token contents:
   ```bash
   whoop-cli auth status --json
   ```

3. Use the smallest data request that satisfies the task:
   ```bash
   whoop-cli workouts list --limit 1 --json
   ```

4. For live smoke tests, redirect response bodies to `/dev/null` unless the user asks to inspect them:
   ```bash
   whoop-cli user get --json >/dev/null
   ```

5. If the CLI behavior is painful or an error is unclear, record local feedback:
   ```bash
   whoop-cli feedback create "describe friction" --json
   ```

## Destructive Commands

- `auth logout` deletes only the local saved token.
- `auth revoke --force` calls WHOOP token revocation and deletes the local saved token.
- Use `auth revoke --force --dry-run --json` before a real revoke unless the user explicitly asks to revoke.
- Run destructive auth tests last because they invalidate the session for later live endpoint checks.
