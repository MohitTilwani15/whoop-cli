package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserGetCallsWhoopProfileEndpointWithBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/user/profile/basic" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing bearer auth: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":10129,"email":"jsmith123@whoop.com","first_name":"John","last_name":"Smith"}`))
	}))
	defer server.Close()

	stdout, stderr, code := ExecuteWithEnv([]string{"user", "get", "--json"}, TestEnv{APIBase: server.URL, AccessToken: "test-token"})
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout must be JSON: %v", err)
	}
	if got["email"] != "jsmith123@whoop.com" {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestMissingAuthReturnsConfigurationError(t *testing.T) {
	stdout, stderr, code := ExecuteWithEnv([]string{"user", "get", "--json"}, TestEnv{APIBase: "http://example.invalid", ConfigDir: t.TempDir()})
	if stdout != "" {
		t.Fatalf("expected no stdout, got %s", stdout)
	}
	if code != 3 {
		t.Fatalf("expected auth/config exit 3, got %d", code)
	}
	if stderr == "" || !json.Valid([]byte(stderr)) {
		t.Fatalf("expected JSON error on stderr, got %q", stderr)
	}
}
