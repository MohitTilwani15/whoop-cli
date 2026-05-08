package main

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOAuthRemainingBranchesForCoverage(t *testing.T) {
	// open-browser branch + callback error branch through auth login
	oldOpen := openBrowser
	oldTimeout := oauthCallbackTimeout
	opened := false
	openBrowser = func(string) error { opened = true; return nil }
	oauthCallbackTimeout = 10 * time.Millisecond
	_, stderr, code := ExecuteWithEnv([]string{"auth", "login", "--client-id", "cid", "--client-secret", "secret", "--redirect-uri", freeLocalRedirect(t), "--state", "abcdefgh", "--json"}, TestEnv{})
	openBrowser = oldOpen
	oauthCallbackTimeout = oldTimeout
	if !opened || code != 1 || !strings.Contains(stderr, "oauth_callback_error") {
		t.Fatalf("expected open + callback timeout, opened=%v code=%d stderr=%s", opened, code, stderr)
	}

	// token exchange failure branch through auth login
	badTokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "bad token", http.StatusBadRequest) }))
	_, stderr, code = ExecuteWithEnv([]string{"auth", "login", "--client-id", "cid", "--client-secret", "secret", "--redirect-uri", "http://localhost:8787/callback", "--code", "bad", "--token-url", badTokenServer.URL, "--json"}, TestEnv{ConfigDir: t.TempDir()})
	badTokenServer.Close()
	if code != 4 || !strings.Contains(stderr, "oauth_token_error") {
		t.Fatalf("expected token error branch")
	}

	// save token failure branch through auth login
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"access_token":"access"}`)) }))
	file := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = ExecuteWithEnv([]string{"auth", "login", "--client-id", "cid", "--client-secret", "secret", "--redirect-uri", "http://localhost:8787/callback", "--code", "ok", "--token-url", tokenServer.URL, "--json"}, TestEnv{ConfigDir: file})
	tokenServer.Close()
	if code != 1 || !strings.Contains(stderr, "io_error") {
		t.Fatalf("expected save token io branch")
	}

	// waitForOAuthCode default path branch
	redirect := strings.TrimSuffix(freeLocalRedirect(t), "/callback")
	done := make(chan string, 1)
	go func() {
		c, err := waitForOAuthCode(redirect, "abcdefgh")
		if err != nil {
			done <- "ERR:" + err.Error()
			return
		}
		done <- c
	}()
	time.Sleep(50 * time.Millisecond)
	resp, err := http.Get(redirect + "/?code=root-code&state=abcdefgh")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if got := <-done; got != "root-code" {
		t.Fatalf("expected root-code, got %s", got)
	}

	// waitForOAuthCode listen error branch
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	busy := "http://" + ln.Addr().String() + "/callback"
	_, err = waitForOAuthCode(busy, "abcdefgh")
	_ = ln.Close()
	if err == nil {
		t.Fatal("expected listen error")
	}

	// exchangeOAuthCode read error branch
	oldClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: errReader{}, Header: make(http.Header)}, nil
	})}
	_, stderr, code = exchangeOAuthCode("http://example.com/token", "cid", "secret", "redir", "code")
	http.DefaultClient = oldClient
	if code != 1 || !strings.Contains(stderr, "io_error") {
		t.Fatalf("expected exchange read io error")
	}

	// saveToken default HOME branch
	t.Setenv("HOME", t.TempDir())
	path, err := saveToken(TestEnv{}, OAuthToken{AccessToken: "home-token"})
	if err != nil || !strings.Contains(path, ".config/whoop-pp-cli/token.json") {
		t.Fatalf("expected default token path, path=%s err=%v", path, err)
	}

	// randomState fallback branch
	oldRand := randRead
	randRead = func([]byte) (int, error) { return 0, errors.New("no entropy") }
	if got := randomState(); got != "abcdefgh" {
		t.Fatalf("expected fallback state, got %s", got)
	}
	randRead = oldRand
}
