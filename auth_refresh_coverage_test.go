package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthRefreshRemainingBranchesForCoverage(t *testing.T) {
	dir := t.TempDir()
	if _, err := saveToken(TestEnv{ConfigDir: dir}, OAuthToken{AccessToken: "old", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	badTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "bad refresh", http.StatusBadRequest) }))
	_, stderr, code := ExecuteWithEnv([]string{"auth", "refresh", "--client-id", "cid", "--client-secret", "secret", "--token-url", badTokenServer.URL, "--json"}, TestEnv{ConfigDir: dir})
	badTokenServer.Close()
	if code != 4 || !strings.Contains(stderr, "oauth_token_error") {
		t.Fatalf("expected refresh exchange error branch, code=%d stderr=%s", code, stderr)
	}

	file := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	goodTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"new","refresh_token":"new-refresh"}`))
	}))
	_, stderr, code = ExecuteWithEnv([]string{"auth", "refresh", "--client-id", "cid", "--client-secret", "secret", "--token-url", goodTokenServer.URL, "--json"}, TestEnv{ConfigDir: file})
	goodTokenServer.Close()
	if code != 3 || !strings.Contains(stderr, "refresh token is missing") {
		t.Fatalf("expected missing refresh token before save when config path invalid, code=%d stderr=%s", code, stderr)
	}

	dir2 := t.TempDir()
	if _, err := saveToken(TestEnv{ConfigDir: dir2}, OAuthToken{AccessToken: "old", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir2, "token.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir2, "token.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = ExecuteWithEnv([]string{"auth", "refresh", "--client-id", "cid", "--client-secret", "secret", "--token-url", goodTokenServer.URL, "--json"}, TestEnv{ConfigDir: dir2})
	if code != 3 || !strings.Contains(stderr, "refresh token is missing") {
		t.Fatalf("expected unreadable token as missing refresh token")
	}

	if got := defaultOpenCommand("about:blank"); got == nil {
		t.Fatal("expected default open command")
	}
	oldExec := execOpenCommand
	execOpenCommand = func(string) *exec.Cmd { return exec.Command("/usr/bin/true") }
	if err := openBrowser("ignored"); err != nil {
		t.Fatalf("expected fake open browser success: %v", err)
	}
	execOpenCommand = oldExec

	dir3 := t.TempDir()
	if _, err := saveToken(TestEnv{ConfigDir: dir3}, OAuthToken{AccessToken: "old", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	goodTokenServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"new","refresh_token":"new-refresh"}`))
	}))
	oldSave := saveTokenFunc
	saveTokenFunc = func(TestEnv, OAuthToken) (string, error) { return "", errors.New("save boom") }
	_, stderr, code = ExecuteWithEnv([]string{"auth", "refresh", "--client-id", "cid", "--client-secret", "secret", "--token-url", goodTokenServer2.URL, "--json"}, TestEnv{ConfigDir: dir3})
	saveTokenFunc = oldSave
	goodTokenServer2.Close()
	if code != 1 || !strings.Contains(stderr, "save boom") {
		t.Fatalf("expected refresh save error branch, code=%d stderr=%s", code, stderr)
	}
}
