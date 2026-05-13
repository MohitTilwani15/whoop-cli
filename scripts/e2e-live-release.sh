#!/bin/sh
set -eu

REPO="${WHOOP_CLI_E2E_REPO:-MohitTilwani15/whoop-cli}"
INSTALL_URL="${WHOOP_CLI_E2E_INSTALL_URL:-https://raw.githubusercontent.com/$REPO/main/install.sh}"
ENV_FILE="${WHOOP_CLI_E2E_ENV:-$HOME/.whoop-cli-e2e.env}"
TOKEN_FILE="${WHOOP_CLI_E2E_TOKEN:-$HOME/.config/whoop-cli/token.json}"

tmp="${TMPDIR:-/tmp}/whoop-cli-live-e2e.$$"
bin_dir="$tmp/bin"
update_dir="$tmp/update-bin"
feedback_config="$tmp/feedback-config"
mkdir -p "$bin_dir" "$update_dir" "$feedback_config"

cleanup() {
	rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

pass_count=0
fail_count=0
skip_count=0
cmd_index=0
LAST_OUT=""

log() {
	printf '%s\n' "$*"
}

pass() {
	pass_count=$((pass_count + 1))
	log "PASS $1"
}

fail() {
	fail_count=$((fail_count + 1))
	log "FAIL $1"
}

skip() {
	skip_count=$((skip_count + 1))
	log "SKIP $1"
}

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		fail "missing required command: $1"
		return 1
	fi
	pass "found required command: $1"
}

run_ok() {
	name="$1"
	shift
	cmd_index=$((cmd_index + 1))
	out="$tmp/out_$cmd_index"
	err="$tmp/err_$cmd_index"
	if "$@" >"$out" 2>"$err"; then
		pass "$name"
	else
		code=$?
		fail "$name exited with $code"
	fi
}

run_json() {
	name="$1"
	shift
	cmd_index=$((cmd_index + 1))
	out="$tmp/out_$cmd_index.json"
	err="$tmp/err_$cmd_index"
	LAST_OUT="$out"
	if "$@" >"$out" 2>"$err"; then
		if jq -e . "$out" >/dev/null 2>&1; then
			pass "$name"
		else
			fail "$name returned invalid JSON"
		fi
	else
		code=$?
		fail "$name exited with $code"
	fi
}

run_expected_json_failure() {
	name="$1"
	shift
	cmd_index=$((cmd_index + 1))
	out="$tmp/out_$cmd_index"
	err="$tmp/err_$cmd_index.json"
	if "$@" >"$out" 2>"$err"; then
		fail "$name unexpectedly succeeded"
	else
		if jq -e . "$err" >/dev/null 2>&1; then
			pass "$name failed with structured JSON"
		else
			fail "$name failed without structured JSON"
		fi
	fi
}

jq_value() {
	file="$1"
	query="$2"
	jq -r "$query // empty" "$file" 2>/dev/null || true
}

run_id_get_or_skip() {
	name="$1"
	id="$2"
	shift 2
	if [ -n "$id" ]; then
		run_json "$name" "$@" "$id" --json
	else
		skip "$name missing source id"
	fi
}

run_cursor_page_or_skip() {
	name="$1"
	cursor="$2"
	shift 2
	if [ -n "$cursor" ]; then
		run_json "$name" "$@" list --limit 1 --cursor "$cursor" --json
	else
		skip "$name missing next_cursor"
	fi
}

require_command curl
require_command jq
require_command tar
require_command shasum

if [ "$fail_count" -ne 0 ]; then
	log "SUMMARY pass=$pass_count fail=$fail_count skip=$skip_count"
	exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
	fail "missing env file: $ENV_FILE"
	log "SUMMARY pass=$pass_count fail=$fail_count skip=$skip_count"
	exit 1
fi

if [ ! -f "$TOKEN_FILE" ]; then
	fail "missing WHOOP token file: $TOKEN_FILE"
	log "SUMMARY pass=$pass_count fail=$fail_count skip=$skip_count"
	exit 1
fi

# shellcheck disable=SC1090
. "$ENV_FILE"

if [ -z "${WHOOP_CLIENT_ID:-}" ] || [ -z "${WHOOP_CLIENT_SECRET:-}" ]; then
	fail "$ENV_FILE must define WHOOP_CLIENT_ID and WHOOP_CLIENT_SECRET"
	log "SUMMARY pass=$pass_count fail=$fail_count skip=$skip_count"
	exit 1
fi

installer="$tmp/install.sh"
curl -fsSL "$INSTALL_URL" -o "$installer"
WHOOP_CLI_INSTALL_DIR="$bin_dir" sh "$installer" >/dev/null
cli="$bin_dir/whoop-cli"

if [ ! -x "$cli" ]; then
	fail "installer did not create executable whoop-cli"
	log "SUMMARY pass=$pass_count fail=$fail_count skip=$skip_count"
	exit 1
fi
pass "installed latest release"

run_ok "version text" "$cli" version
run_json "version json" "$cli" version --json
run_json "agent-context" "$cli" agent-context
run_json "update check" "$cli" update --check --json
run_json "update temp install" "$cli" update --install-dir "$update_dir" --json

run_json "auth refresh" "$cli" auth refresh --client-id "$WHOOP_CLIENT_ID" --client-secret "$WHOOP_CLIENT_SECRET" --json
run_json "auth status" "$cli" auth status --json
if jq -e '.authenticated == true' "$LAST_OUT" >/dev/null 2>&1; then
	pass "auth status authenticated"
else
	fail "auth status not authenticated"
fi
run_json "auth revoke dry-run" "$cli" auth revoke --force --dry-run --json

run_json "feedback create temp config" "$cli" feedback create "daily live e2e smoke" --config "$feedback_config" --json
run_json "feedback list temp config" "$cli" feedback list --config "$feedback_config" --json

run_json "user get" "$cli" user get --json
run_json "user body get" "$cli" user body get --json

run_json "workouts list" "$cli" workouts list --limit 1 --json
workouts_out="$LAST_OUT"
workout_id="$(jq_value "$workouts_out" '.records[0].id')"
workout_v1_id="$(jq_value "$workouts_out" '.records[0].v1_id')"
workout_cursor="$(jq_value "$workouts_out" '.next_cursor')"
run_id_get_or_skip "workouts get" "$workout_id" "$cli" workouts get
run_id_get_or_skip "mapping get" "$workout_v1_id" "$cli" mapping get
run_cursor_page_or_skip "workouts cursor page" "$workout_cursor" "$cli" workouts

run_json "sleep list" "$cli" sleep list --limit 1 --json
sleep_out="$LAST_OUT"
sleep_id="$(jq_value "$sleep_out" '.records[0].id')"
sleep_cycle_id="$(jq_value "$sleep_out" '.records[0].cycle_id')"
sleep_cursor="$(jq_value "$sleep_out" '.next_cursor')"
run_id_get_or_skip "sleep get" "$sleep_id" "$cli" sleep get
run_id_get_or_skip "cycles sleep get" "$sleep_cycle_id" "$cli" cycles sleep get
run_cursor_page_or_skip "sleep cursor page" "$sleep_cursor" "$cli" sleep

run_json "cycles list" "$cli" cycles list --limit 1 --json
cycles_out="$LAST_OUT"
cycle_id="$(jq_value "$cycles_out" '.records[0].id')"
cycle_cursor="$(jq_value "$cycles_out" '.next_cursor')"
run_id_get_or_skip "cycles get" "$cycle_id" "$cli" cycles get
run_cursor_page_or_skip "cycles cursor page" "$cycle_cursor" "$cli" cycles

run_json "recovery list" "$cli" recovery list --limit 1 --json
recovery_out="$LAST_OUT"
recovery_cycle_id="$(jq_value "$recovery_out" '.records[0].cycle_id')"
recovery_cursor="$(jq_value "$recovery_out" '.next_cursor')"
run_id_get_or_skip "cycles recovery get" "$recovery_cycle_id" "$cli" cycles recovery get
run_cursor_page_or_skip "recovery cursor page" "$recovery_cursor" "$cli" recovery

run_expected_json_failure "invalid workout limit" "$cli" workouts list --limit 100 --json

log "SUMMARY pass=$pass_count fail=$fail_count skip=$skip_count"

if [ "$fail_count" -ne 0 ]; then
	exit 1
fi
