package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestMainFunctionUsesOsExitHook(t *testing.T) {
	oldExit := osExit
	oldArgs := os.Args
	defer func() { osExit = oldExit; os.Args = oldArgs }()
	called := false
	osExit = func(code int) {
		called = true
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	}
	os.Args = []string{"whoop-cli", "version"}
	main()
	if !called {
		t.Fatal("expected osExit hook to be called")
	}
}

func TestRunMainReturnsErrorCode(t *testing.T) {
	if code := runMain([]string{"does-not-exist"}); code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

func TestHelpAndUnknownCommands(t *testing.T) {
	stdout, stderr, code := ExecuteWithEnv(nil, TestEnv{})
	if code != 0 || stderr != "" || stdout == "" {
		t.Fatalf("expected help success, got code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = ExecuteWithEnv([]string{"--help"}, TestEnv{})
	if code != 0 || stderr != "" || stdout == "" {
		t.Fatalf("expected help flag success")
	}
	stdout, stderr, code = ExecuteWithEnv([]string{"nope"}, TestEnv{})
	if code != 2 || stdout != "" || !json.Valid([]byte(stderr)) {
		t.Fatalf("expected JSON unknown command error, code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAuthBranches(t *testing.T) {
	cases := [][]string{{"auth"}, {"auth", "status", "--json"}, {"auth", "login", "--json"}, {"auth", "refresh", "--json"}, {"auth", "logout", "--json"}, {"auth", "revoke", "--force", "--json"}, {"auth", "wat", "--json"}}
	for _, args := range cases {
		ExecuteWithEnv(args, TestEnv{})
	}
}

func TestInvalidInvocations(t *testing.T) {
	cases := [][]string{
		{"workouts"}, {"workouts", "get"}, {"workouts", "wat"},
		{"sleep"}, {"sleep", "get"}, {"sleep", "wat"},
		{"cycles"}, {"cycles", "get"}, {"cycles", "sleep", "get"}, {"cycles", "wat"},
		{"recovery"}, {"mapping"}, {"mapping", "get"}, {"user", "wat"},
		{"feedback"}, {"feedback", "create", "--json"}, {"feedback", "wat"},
	}
	for _, args := range cases {
		_, stderr, code := ExecuteWithEnv(args, TestEnv{ConfigDir: t.TempDir()})
		if code == 0 || !json.Valid([]byte(stderr)) {
			t.Fatalf("expected JSON error for %v, code=%d stderr=%q", args, code, stderr)
		}
	}
}

func TestFeedbackListNoFile(t *testing.T) {
	stdout, stderr, code := ExecuteWithEnv([]string{"feedback", "list", "--json"}, TestEnv{ConfigDir: t.TempDir()})
	if code != 0 || stderr != "" || !json.Valid([]byte(stdout)) {
		t.Fatalf("expected empty feedback list JSON, code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
}

func TestParseLimitErrors(t *testing.T) {
	cases := [][]string{{"--limit"}, {"--limit", "abc"}, {"--limit", "0"}}
	for _, args := range cases {
		_, stderr, code := parseLimit(args, "example")
		if code != 2 || !json.Valid([]byte(stderr)) {
			t.Fatalf("expected JSON limit error for %v", args)
		}
	}
}

func TestListEndpointUnknown(t *testing.T) {
	if _, ok := listEndpoint("unknown"); ok {
		t.Fatal("expected unknown resource")
	}
}
