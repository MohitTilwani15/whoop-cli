package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAPIRequestLoadsSavedTokenWhenEnvMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token.json"), []byte(`{"access_token":"saved-access"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/user/profile/basic" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer saved-access" {
			t.Fatalf("expected saved token auth header, got %q", got)
		}
		_, _ = w.Write([]byte(`{"user_id":123}`))
	}))
	defer server.Close()
	_, stderr, code := ExecuteWithEnv([]string{"user", "get", "--json"}, TestEnv{ConfigDir: dir, APIBase: server.URL})
	if code != 0 || stderr != "" {
		t.Fatalf("expected saved token success, code=%d stderr=%s", code, stderr)
	}
}

func TestLoadSavedTokenBranches(t *testing.T) {
	if got := loadSavedAccessToken(TestEnv{ConfigDir: t.TempDir()}); got != "" {
		t.Fatalf("expected empty missing token, got %q", got)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token.json"), []byte(`not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadSavedAccessToken(TestEnv{ConfigDir: dir}); got != "" {
		t.Fatalf("expected empty invalid token, got %q", got)
	}
	t.Setenv("HOME", t.TempDir())
	if got := loadSavedAccessToken(TestEnv{}); got != "" {
		t.Fatalf("expected empty home fallback token, got %q", got)
	}
}
