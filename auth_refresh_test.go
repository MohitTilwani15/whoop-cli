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

func TestAuthRefreshUsesSavedRefreshTokenAndRotatesTokenFile(t *testing.T) {
	dir := t.TempDir()
	oldTok := OAuthToken{AccessToken: "old-access", RefreshToken: "old-refresh", ExpiresIn: 1, TokenType: "bearer", Scope: "read:profile", ObtainedAt: "2026-01-01T00:00:00Z"}
	if _, err := saveToken(TestEnv{ConfigDir: dir}, oldTok); err != nil {
		t.Fatal(err)
	}

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "old-refresh" || r.Form.Get("client_id") != "cid" || r.Form.Get("client_secret") != "secret" || r.Form.Get("scope") != "offline" {
			t.Fatalf("bad refresh form: %#v", r.Form)
		}
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"token_type":"bearer","scope":"read:profile"}`))
	}))
	defer tokenServer.Close()

	stdout, stderr, code := ExecuteWithEnv([]string{"auth", "refresh", "--client-id", "cid", "--client-secret", "secret", "--token-url", tokenServer.URL, "--json"}, TestEnv{ConfigDir: dir})
	if code != 0 || stderr != "" {
		t.Fatalf("expected refresh success code=%d stderr=%s", code, stderr)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatal(err)
	}
	if out["token_refreshed"] != true {
		t.Fatalf("expected token_refreshed true: %s", stdout)
	}
	b, err := os.ReadFile(filepath.Join(dir, "token.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "new-access") || !strings.Contains(string(b), "new-refresh") || strings.Contains(string(b), "old-refresh") {
		t.Fatalf("token file was not rotated correctly: %s", string(b))
	}
}

func TestAuthRefreshErrorsTeachMissingInputs(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := ExecuteWithEnv([]string{"auth", "refresh", "--client-id", "cid", "--client-secret", "secret", "--json"}, TestEnv{ConfigDir: dir})
	if code != 3 || !strings.Contains(stderr, "refresh token is missing") {
		t.Fatalf("expected missing refresh token error, code=%d stderr=%s", code, stderr)
	}

	if _, err := saveToken(TestEnv{ConfigDir: dir}, OAuthToken{AccessToken: "old-access"}); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = ExecuteWithEnv([]string{"auth", "refresh", "--client-id", "cid", "--client-secret", "secret", "--json"}, TestEnv{ConfigDir: dir})
	if code != 3 || !strings.Contains(stderr, "refresh token is missing") {
		t.Fatalf("expected token file missing refresh token error")
	}

	_, stderr, code = ExecuteWithEnv([]string{"auth", "refresh", "--json"}, TestEnv{ConfigDir: dir})
	if code != 3 || !strings.Contains(stderr, "requires --client-id and --client-secret") {
		t.Fatalf("expected missing credentials error")
	}
}

func TestExchangeRefreshTokenErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "bad refresh", http.StatusBadRequest) }))
	_, stderr, code := exchangeRefreshToken(server.URL, "cid", "secret", "refresh")
	server.Close()
	if code != 4 || !strings.Contains(stderr, "oauth_token_error") {
		t.Fatalf("expected refresh HTTP error")
	}

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`not-json`)) }))
	_, _, code = exchangeRefreshToken(server.URL, "cid", "secret", "refresh")
	server.Close()
	if code != 5 {
		t.Fatalf("expected invalid json exit 5")
	}

	_, _, code = exchangeRefreshToken("http://127.0.0.1:1", "cid", "secret", "refresh")
	if code != 7 {
		t.Fatalf("expected network exit 7")
	}
}
