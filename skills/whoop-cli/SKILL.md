---
name: whoop-cli
description: Use whoop-cli to fetch WHOOP profile, body, sleep, recovery, cycles, and workouts with agent-native JSON output.
version: 0.1.0
---

# whoop-cli

## When to use

Use this skill when an agent needs WHOOP data from the local CLI.

## First step

Always inspect the machine-readable surface first:

```bash
whoop-cli agent-context
```

## Conventions

- Always request `--json` on data commands.
- Use `get`, not `info`.
- Use `list`, not `ls`.
- Use `--force` for destructive operations.
- Bound list commands with `--limit`, `--start`, and `--end` unless the user explicitly asks for all data.

## Common workflows

### Basic profile

```bash
WHOOP_ACCESS_TOKEN=... whoop-cli user get --json
```

### Body measurements

```bash
WHOOP_ACCESS_TOKEN=... whoop-cli user body get --json
```

### Workouts

```bash
WHOOP_ACCESS_TOKEN=... whoop-cli workouts list --start 2026-05-01T00:00:00Z --end 2026-05-08T00:00:00Z --json
```

## Feedback

If the CLI is painful or an error message is unhelpful, record feedback:

```bash
whoop-cli feedback create "describe friction" --json
```
