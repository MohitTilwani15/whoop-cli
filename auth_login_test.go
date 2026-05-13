package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthLoginPrintURLBuildsWhoopAuthorizeURL(t *testing.T) {
	stdout, stderr, code := ExecuteWithEnv([]string{"auth", "login", "--client-id", "cid", "--redirect-uri", "http://localhost:8787/callback", "--scopes", "read:profile read:sleep", "--state", "abcdefgh", "--print-url", "--json"}, TestEnv{})
	if code != 0 || stderr != "" {
		t.Fatalf("expected success code=%d stderr=%s", code, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	authURL := got["authorization_url"].(string)
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "api.prod.whoop.com" || u.Path != "/oauth/oauth2/auth" {
		t.Fatalf("bad auth URL: %s", authURL)
	}
	if strings.Contains(stdout, `\u0026`) || !strings.Contains(stdout, "&redirect_uri=") {
		t.Fatalf("authorization_url should be copy-pasteable without escaped ampersands: %s", stdout)
	}
	q := u.Query()
	if q.Get("client_id") != "cid" || q.Get("redirect_uri") != "http://localhost:8787/callback" || q.Get("scope") != "read:profile read:sleep" || q.Get("state") != "abcdefgh" || q.Get("response_type") != "code" {
		t.Fatalf("bad query: %s", u.RawQuery)
	}
}

func TestAuthLoginPrintURLDoesNotRequireClientSecret(t *testing.T) {
	stdout, stderr, code := ExecuteWithEnv([]string{"auth", "login", "--client-id", "cid", "--state", "abcdefgh", "--print-url", "--json"}, TestEnv{})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "authorization_url") {
		t.Fatalf("expected print-url success without secret, code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestAuthLoginCodeExchangeRequiresClientSecret(t *testing.T) {
	_, stderr, code := ExecuteWithEnv([]string{"auth", "login", "--client-id", "cid", "--code", "cb-code", "--json"}, TestEnv{ConfigDir: t.TempDir()})
	if code != 3 || !strings.Contains(stderr, "code exchange requires --client-secret") {
		t.Fatalf("expected missing exchange secret error, code=%d stderr=%s", code, stderr)
	}
}

func TestAuthLoginRejectsInvalidStateLength(t *testing.T) {
	_, stderr, code := ExecuteWithEnv([]string{"auth", "login", "--client-id", "cid", "--client-secret", "secret", "--state", "short", "--print-url", "--json"}, TestEnv{})
	if code != 2 || !strings.Contains(stderr, "--state must be exactly 8 characters") {
		t.Fatalf("expected teaching state error, code=%d stderr=%s", code, stderr)
	}
}

func TestAuthLoginExchangeCodeStoresToken(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "code123" || r.Form.Get("client_id") != "cid" || r.Form.Get("client_secret") != "secret" || r.Form.Get("redirect_uri") != "http://localhost:8787/callback" {
			t.Fatalf("bad form: %#v", r.Form)
		}
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":3600,"token_type":"bearer","scope":"read:profile"}`))
	}))
	defer tokenServer.Close()
	dir := t.TempDir()
	stdout, stderr, code := ExecuteWithEnv([]string{"auth", "login", "--client-id", "cid", "--client-secret", "secret", "--redirect-uri", "http://localhost:8787/callback", "--code", "code123", "--token-url", tokenServer.URL, "--json"}, TestEnv{ConfigDir: dir})
	if code != 0 || stderr != "" {
		t.Fatalf("expected success code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "token_saved") {
		t.Fatalf("expected token_saved output: %s", stdout)
	}
	b, err := os.ReadFile(filepath.Join(dir, "token.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "access") || strings.Contains(string(b), "secret") {
		t.Fatalf("token file wrong: %s", string(b))
	}
}
