package main

import (
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

func freeLocalRedirect(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return "http://" + addr + "/callback"
}

func TestWaitForOAuthCodeSuccessAndErrors(t *testing.T) {
	redirect := freeLocalRedirect(t)
	done := make(chan string, 1)
	go func() {
		code, err := waitForOAuthCode(redirect, "abcdefgh")
		if err != nil {
			done <- "ERR:" + err.Error()
			return
		}
		done <- code
	}()
	time.Sleep(50 * time.Millisecond)
	resp, err := http.Get(redirect + "?code=ok-code&state=abcdefgh")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if got := <-done; got != "ok-code" {
		t.Fatalf("expected code, got %s", got)
	}

	redirect = freeLocalRedirect(t)
	done = make(chan string, 1)
	go func() {
		_, err := waitForOAuthCode(redirect, "abcdefgh")
		if err != nil {
			done <- err.Error()
		}
	}()
	time.Sleep(50 * time.Millisecond)
	resp, err = http.Get(redirect + "?code=ok-code&state=badstate")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if got := <-done; !strings.Contains(got, "state mismatch") {
		t.Fatalf("expected state mismatch, got %s", got)
	}

	redirect = freeLocalRedirect(t)
	done = make(chan string, 1)
	go func() {
		_, err := waitForOAuthCode(redirect, "abcdefgh")
		if err != nil {
			done <- err.Error()
		}
	}()
	time.Sleep(50 * time.Millisecond)
	resp, err = http.Get(redirect + "?state=abcdefgh")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if got := <-done; !strings.Contains(got, "missing code") {
		t.Fatalf("expected missing code, got %s", got)
	}

	oldTimeout := oauthCallbackTimeout
	oauthCallbackTimeout = 10 * time.Millisecond
	_, err = waitForOAuthCode(freeLocalRedirect(t), "abcdefgh")
	oauthCallbackTimeout = oldTimeout
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout, got %v", err)
	}

	if _, err = waitForOAuthCode("http://127.0.0.1:1/%zz", "abcdefgh"); err == nil {
		t.Fatal("expected bad redirect URL error")
	}
}

func TestAuthLoginEndToEndViaLocalCallback(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"access2","refresh_token":"refresh2","expires_in":60}`))
	}))
	defer tokenServer.Close()
	redirect := freeLocalRedirect(t)
	dir := t.TempDir()
	done := make(chan struct{}, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(redirect + "?code=cb-code&state=abcdefgh")
		if err == nil {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}
		done <- struct{}{}
	}()
	stdout, stderr, code := ExecuteWithEnv([]string{"auth", "login", "--client-id", "cid", "--client-secret", "secret", "--redirect-uri", redirect, "--state", "abcdefgh", "--token-url", tokenServer.URL, "--no-browser", "--json"}, TestEnv{ConfigDir: dir})
	<-done
	if code != 0 || stderr != "" || !strings.Contains(stdout, "token_saved") {
		t.Fatalf("expected login success code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestOAuthExchangeAndSaveErrorBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "bad", http.StatusBadRequest) }))
	_, stderr, code := exchangeOAuthCode(server.URL, "cid", "secret", "redir", "code")
	server.Close()
	if code != 4 || stderr == "" {
		t.Fatalf("expected token status error")
	}

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`not-json`)) }))
	_, _, code = exchangeOAuthCode(server.URL, "cid", "secret", "redir", "code")
	server.Close()
	if code != 5 {
		t.Fatalf("expected invalid token json")
	}

	_, _, code = exchangeOAuthCode("http://127.0.0.1:1", "cid", "secret", "redir", "code")
	if code != 7 {
		t.Fatalf("expected network error")
	}

	file := filepath.Join(t.TempDir(), "not-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := saveToken(TestEnv{ConfigDir: file}, OAuthToken{AccessToken: "x"}); err == nil {
		t.Fatal("expected save token mkdir error")
	}
}

func TestRandomStateIsEightCharacters(t *testing.T) {
	if got := randomState(); len(got) != 8 {
		t.Fatalf("expected 8 char state, got %q", got)
	}
}
