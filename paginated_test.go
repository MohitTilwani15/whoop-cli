package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newRecordingServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing bearer auth: %q", r.Header.Get("Authorization"))
		}
		if h, ok := handlers[r.URL.Path]; ok {
			h(w, r)
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	}))
}

func TestWorkoutsListCallsPaginatedEndpointWithFilters(t *testing.T) {
	server := newRecordingServer(t, map[string]http.HandlerFunc{
		"/v2/activity/workout": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("limit") != "25" || q.Get("start") != "2026-05-01T00:00:00Z" || q.Get("end") != "2026-05-08T00:00:00Z" || q.Get("nextToken") != "abc" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"records":[{"id":"w1","sport_name":"Running"}],"next_token":"def"}`))
		},
	})
	defer server.Close()

	stdout, stderr, code := ExecuteWithEnv([]string{"workouts", "list", "--limit", "25", "--start", "2026-05-01T00:00:00Z", "--end", "2026-05-08T00:00:00Z", "--cursor", "abc", "--json"}, TestEnv{APIBase: server.URL, AccessToken: "test-token"})
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout must be JSON: %v", err)
	}
	if got["next_cursor"] != "def" || got["truncated"] != true {
		t.Fatalf("expected normalized pagination metadata, got %#v", got)
	}
}

func TestSleepListAllFollowsNextToken(t *testing.T) {
	calls := 0
	server := newRecordingServer(t, map[string]http.HandlerFunc{
		"/v2/activity/sleep": func(w http.ResponseWriter, r *http.Request) {
			calls++
			if calls == 1 {
				_, _ = w.Write([]byte(`{"records":[{"id":"s1"}],"next_token":"n2"}`))
				return
			}
			if r.URL.Query().Get("nextToken") != "n2" {
				t.Fatalf("second call missing nextToken=n2: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"records":[{"id":"s2"}]}`))
		},
	})
	defer server.Close()

	stdout, stderr, code := ExecuteWithEnv([]string{"sleep", "list", "--all", "--json"}, TestEnv{APIBase: server.URL, AccessToken: "test-token"})
	if code != 0 {
		t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
	records := got["records"].([]any)
	if len(records) != 2 || got["truncated"] != false {
		t.Fatalf("expected combined records without truncation, got %#v", got)
	}
}

func TestGetCommandsCallCorrectEndpoints(t *testing.T) {
	cases := []struct {
		name string
		args []string
		path string
	}{
		{"workout get", []string{"workouts", "get", "workout-id", "--json"}, "/v2/activity/workout/workout-id"},
		{"sleep get", []string{"sleep", "get", "sleep-id", "--json"}, "/v2/activity/sleep/sleep-id"},
		{"cycle get", []string{"cycles", "get", "123", "--json"}, "/v2/cycle/123"},
		{"cycle sleep get", []string{"cycles", "sleep", "get", "123", "--json"}, "/v2/cycle/123/sleep"},
		{"cycle recovery get", []string{"cycles", "recovery", "get", "123", "--json"}, "/v2/cycle/123/recovery"},
		{"mapping get", []string{"mapping", "get", "456", "--json"}, "/v1/activity-mapping/456"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newRecordingServer(t, map[string]http.HandlerFunc{
				tc.path: func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"ok":true}`)) },
			})
			defer server.Close()
			stdout, stderr, code := ExecuteWithEnv(tc.args, TestEnv{APIBase: server.URL, AccessToken: "test-token"})
			if code != 0 || stderr != "" {
				t.Fatalf("expected success, code=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
		})
	}
}

func TestRecoveryAndCyclesListUseCorrectEndpoints(t *testing.T) {
	cases := []struct{ command, path string }{{"recovery", "/v2/recovery"}, {"cycles", "/v2/cycle"}}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			server := newRecordingServer(t, map[string]http.HandlerFunc{
				tc.path: func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"records":[]}`)) },
			})
			defer server.Close()
			_, stderr, code := ExecuteWithEnv([]string{tc.command, "list", "--json"}, TestEnv{APIBase: server.URL, AccessToken: "test-token"})
			if code != 0 {
				t.Fatalf("expected success, got code=%d stderr=%s", code, stderr)
			}
		})
	}
}
