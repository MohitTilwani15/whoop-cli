package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentContextIncludesImplementedCommands(t *testing.T) {
	stdout, stderr, code := ExecuteWithEnv([]string{"agent-context"}, TestEnv{})
	if code != 0 || stderr != "" {
		t.Fatalf("expected agent-context success, code=%d stderr=%s", code, stderr)
	}
	var ctx AgentContext
	if err := json.Unmarshal([]byte(stdout), &ctx); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{
		"auth.login", "auth.status", "auth.refresh", "auth.logout", "auth.revoke",
		"user.get", "user.body.get", "cycles.list", "cycles.get", "cycles.sleep.get", "cycles.recovery.get",
		"recovery.list", "sleep.list", "sleep.get", "workouts.list", "workouts.get", "mapping.get",
		"feedback.create", "feedback.list",
		"update",
	} {
		if _, ok := ctx.Commands[command]; !ok {
			t.Fatalf("agent-context missing implemented command %s", command)
		}
	}
}

func TestAuthStatusAndLogoutReflectSavedToken(t *testing.T) {
	dir := t.TempDir()
	if _, err := saveToken(TestEnv{ConfigDir: dir}, OAuthToken{AccessToken: "saved", ExpiresIn: 60, Scope: "read:profile", ObtainedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := ExecuteWithEnv([]string{"auth", "status", "--json"}, TestEnv{ConfigDir: dir})
	if code != 0 || stderr != "" {
		t.Fatalf("expected status success, code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `"authenticated": true`) || !strings.Contains(stdout, `"expires_at": "2026-01-01T00:01:00Z"`) {
		t.Fatalf("status should report saved token and expiry: %s", stdout)
	}

	stdout, stderr, code = ExecuteWithEnv([]string{"auth", "logout", "--json"}, TestEnv{ConfigDir: dir})
	if code != 0 || stderr != "" {
		t.Fatalf("expected logout success, code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `"token_deleted": true`) {
		t.Fatalf("logout should delete token: %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "token.json")); !os.IsNotExist(err) {
		t.Fatalf("expected token file deleted, err=%v", err)
	}
}

func TestAuthRevokeCallsWhoopDeleteAndDeletesToken(t *testing.T) {
	dir := t.TempDir()
	if _, err := saveToken(TestEnv{ConfigDir: dir}, OAuthToken{AccessToken: "saved-access"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v2/user/access" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer saved-access" {
			t.Fatalf("missing saved token auth: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	stdout, stderr, code := ExecuteWithEnv([]string{"auth", "revoke", "--force", "--json"}, TestEnv{ConfigDir: dir, APIBase: server.URL})
	if code != 0 || stderr != "" {
		t.Fatalf("expected revoke success, code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `"revoked": true`) || !strings.Contains(stdout, `"token_deleted": true`) {
		t.Fatalf("unexpected revoke output: %s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "token.json")); !os.IsNotExist(err) {
		t.Fatalf("expected token file deleted, err=%v", err)
	}
}

func TestAuthRevokeDryRunDoesNotRequireToken(t *testing.T) {
	stdout, stderr, code := ExecuteWithEnv([]string{"auth", "revoke", "--force", "--dry-run", "--json"}, TestEnv{ConfigDir: t.TempDir()})
	if code != 0 || stderr != "" {
		t.Fatalf("expected dry-run success, code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `"dry_run": true`) || strings.Contains(stdout, `"revoked": true`) {
		t.Fatalf("unexpected dry-run output: %s", stdout)
	}
}

func TestPathSegmentsAreEscaped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/v2/activity/workout/a%2Fb" {
			t.Fatalf("expected escaped path, got %s", r.RequestURI)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	_, stderr, code := ExecuteWithEnv([]string{"workouts", "get", "a/b", "--json"}, TestEnv{APIBase: server.URL, AccessToken: "tok"})
	if code != 0 || stderr != "" {
		t.Fatalf("expected escaped get success, code=%d stderr=%s", code, stderr)
	}
}

func TestRuntimeFlagsConfigureAPIBaseConfigTimeoutAndRetries(t *testing.T) {
	dir := t.TempDir()
	if _, err := saveToken(TestEnv{ConfigDir: dir}, OAuthToken{AccessToken: "saved-access"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "transient", http.StatusInternalServerError)
			return
		}
		if r.Header.Get("Authorization") != "Bearer saved-access" {
			t.Fatalf("expected token loaded from --config, got %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	stdout, stderr, code := ExecuteWithEnv([]string{"user", "get", "--api-base", server.URL, "--config", dir, "--timeout", "1s", "--retries", "1", "--json"}, TestEnv{})
	if code != 0 || stderr != "" {
		t.Fatalf("expected retry success, code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if calls != 2 {
		t.Fatalf("expected one retry, got %d calls", calls)
	}
}

func TestRateLimitErrorIncludesRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "3")
		http.Error(w, "too many", http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, stderr, code := ExecuteWithEnv([]string{"user", "get", "--json"}, TestEnv{APIBase: server.URL, AccessToken: "tok"})
	if code != 6 || !strings.Contains(stderr, `"retry_after": "3"`) {
		t.Fatalf("expected rate limit retry_after, code=%d stderr=%s", code, stderr)
	}
}

func TestFeedbackListRejectsCorruptJSONL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "feedback.jsonl"), []byte("{bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := ExecuteWithEnv([]string{"feedback", "list", "--json"}, TestEnv{ConfigDir: dir})
	if code != 5 || !strings.Contains(stderr, "feedback file contains invalid JSONL") {
		t.Fatalf("expected invalid feedback JSONL error, code=%d stderr=%s", code, stderr)
	}
}
