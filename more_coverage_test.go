package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemainingBranchesForCoverage(t *testing.T) {
	_, stderr, code := handleListLike("unknown", []string{"list"}, TestEnv{AccessToken: "tok"})
	if code != 2 || stderr == "" {
		t.Fatalf("expected unknown list resource error")
	}

	server := newRecordingServer(t, map[string]http.HandlerFunc{
		"/v2/user/measurement/body": func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"height_meter":1.8}`)) },
	})
	defer server.Close()
	_, stderr, code = ExecuteWithEnv([]string{"user", "body", "get", "--json"}, TestEnv{APIBase: server.URL, AccessToken: "test-token"})
	if code != 0 || stderr != "" {
		t.Fatalf("expected body success, code=%d stderr=%s", code, stderr)
	}

	t.Setenv("HOME", t.TempDir())
	stdout, stderr, code := ExecuteWithEnv([]string{"feedback", "list", "--json"}, TestEnv{})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "records") {
		t.Fatalf("expected home fallback feedback list")
	}

	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = ExecuteWithEnv([]string{"feedback", "create", "x", "--json"}, TestEnv{ConfigDir: file})
	if code != 1 || stderr == "" {
		t.Fatalf("expected mkdir io error")
	}

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "feedback.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = ExecuteWithEnv([]string{"feedback", "create", "x", "--json"}, TestEnv{ConfigDir: dir})
	if code != 1 || stderr == "" {
		t.Fatalf("expected append io error")
	}
	_, stderr, code = ExecuteWithEnv([]string{"feedback", "list", "--json"}, TestEnv{ConfigDir: dir})
	if code != 1 || stderr == "" {
		t.Fatalf("expected read io error")
	}

	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "feedback.jsonl"), []byte("{}\n\n{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = ExecuteWithEnv([]string{"feedback", "list", "--json"}, TestEnv{ConfigDir: dir2})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "records") {
		t.Fatalf("expected feedback list with blank line")
	}
}

func TestAPIListPropagatesRequestErrorAndDefaultBaseBranches(t *testing.T) {
	_, stderr, code := apiList(TestEnv{}, "/x", nil, "example")
	if code != 3 || stderr == "" {
		t.Fatalf("expected auth missing from apiList")
	}

	oldClient := http.DefaultClient
	defer func() { http.DefaultClient = oldClient }()
	seenURL := ""
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seenURL = r.URL.String()
		return &http.Response{StatusCode: 200, Body: ioNopCloser{strings.NewReader(`{"ok":true}`)}, Header: make(http.Header)}, nil
	})}
	t.Setenv("WHOOP_API_BASE", "")
	_, _, code = apiGET(TestEnv{AccessToken: "tok"}, "/x")
	if code != 0 || !strings.HasPrefix(seenURL, "https://api.prod.whoop.com/developer/x") {
		t.Fatalf("expected default base, code=%d url=%s", code, seenURL)
	}

	t.Setenv("WHOOP_API_BASE", "http://env-base")
	_, _, code = apiGET(TestEnv{AccessToken: "tok"}, "/y")
	if code != 0 || !strings.HasPrefix(seenURL, "http://env-base/y") {
		t.Fatalf("expected env base, code=%d url=%s", code, seenURL)
	}
}

type ioNopCloser struct{ *strings.Reader }

func (ioNopCloser) Close() error { return nil }
