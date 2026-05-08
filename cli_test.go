package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func executeForTest(args ...string) (stdout string, stderr string, code int) {
	return Execute(args, TestEnv{})
}

func TestAgentContextIsVersionedAndNamesConventions(t *testing.T) {
	stdout, stderr, code := executeForTest("agent-context")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout must be JSON: %v\n%s", err, stdout)
	}
	if got["schema_version"] != "1" {
		t.Fatalf("expected schema_version=1, got %#v", got["schema_version"])
	}
	conventions := got["conventions"].(map[string]any)
	if conventions["json_flag"] != "--json" {
		t.Fatalf("expected --json convention, got %#v", conventions["json_flag"])
	}
	if conventions["destructive_bypass_flag"] != "--force" {
		t.Fatalf("expected --force convention, got %#v", conventions["destructive_bypass_flag"])
	}
	commands := got["commands"].(map[string]any)
	for _, command := range []string{"user.get", "workouts.list", "sleep.get", "auth.revoke", "feedback.create"} {
		if _, ok := commands[command]; !ok {
			t.Fatalf("agent-context missing command %s", command)
		}
	}
}

func TestListLimitRejectsAboveWhoopMaximumWithTeachingError(t *testing.T) {
	stdout, stderr, code := executeForTest("workouts", "list", "--limit", "100", "--json")
	if stdout != "" {
		t.Fatalf("expected no stdout on error, got %q", stdout)
	}
	if code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	if !strings.Contains(stderr, "--limit must be between 1 and 25") {
		t.Fatalf("stderr should teach valid range, got %q", stderr)
	}
	if !strings.Contains(stderr, "whoop-pp-cli workouts list --limit 25 --json") {
		t.Fatalf("stderr should include working example, got %q", stderr)
	}
}

func TestDestructiveRevokeRequiresForce(t *testing.T) {
	stdout, stderr, code := executeForTest("auth", "revoke", "--json")
	if stdout != "" {
		t.Fatalf("expected no stdout on error, got %q", stdout)
	}
	if code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	if !strings.Contains(stderr, "auth revoke is destructive and requires --force") {
		t.Fatalf("stderr should explain --force requirement, got %q", stderr)
	}
}

func TestFeedbackCreatePersistsLocally(t *testing.T) {
	env := TestEnv{ConfigDir: t.TempDir()}
	stdout, stderr, code := ExecuteWithEnv([]string{"feedback", "create", "limit error should enumerate valid range", "--json"}, env)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout must be JSON: %v\n%s", err, stdout)
	}
	if got["recorded"] != true {
		t.Fatalf("expected recorded=true, got %#v", got)
	}

	stdout, stderr, code = ExecuteWithEnv([]string{"feedback", "list", "--json"}, env)
	if code != 0 {
		t.Fatalf("expected list exit 0, got %d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "limit error should enumerate valid range") {
		t.Fatalf("feedback list missing entry: %s", stdout)
	}
}
