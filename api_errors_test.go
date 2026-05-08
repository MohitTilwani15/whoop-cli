package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read boom") }
func (errReader) Close() error             { return nil }

func TestAPIStatusAndInvalidJSONErrors(t *testing.T) {
	cases := []struct{ status, want int }{{http.StatusUnauthorized, 3}, {http.StatusTooManyRequests, 6}, {http.StatusInternalServerError, 5}, {http.StatusBadRequest, 4}}
	for _, tc := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", tc.status) }))
		_, _, code := apiGET(TestEnv{APIBase: server.URL, AccessToken: "tok"}, "/x")
		server.Close()
		if code != tc.want {
			t.Fatalf("status %d expected exit %d got %d", tc.status, tc.want, code)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`not-json`)) }))
	defer server.Close()
	_, _, code := apiGET(TestEnv{APIBase: server.URL, AccessToken: "tok"}, "/x")
	if code != 5 {
		t.Fatalf("expected invalid json exit 5, got %d", code)
	}
	_, _, code = apiList(TestEnv{APIBase: server.URL, AccessToken: "tok"}, "/x", nil, "example")
	if code != 5 {
		t.Fatalf("expected invalid page json exit 5, got %d", code)
	}
}

func TestAPIRequestInvalidAndNetworkErrors(t *testing.T) {
	_, _, _, code := apiRequest(TestEnv{APIBase: "http://[::1", AccessToken: "tok"}, "/x", nil)
	if code != 2 {
		t.Fatalf("expected invalid request exit 2, got %d", code)
	}
	_, _, _, code = apiRequest(TestEnv{APIBase: "http://127.0.0.1:1", AccessToken: "tok"}, "/x", nil)
	if code != 7 {
		t.Fatalf("expected network exit 7, got %d", code)
	}
}

func TestAPIRequestReadError(t *testing.T) {
	old := http.DefaultClient
	defer func() { http.DefaultClient = old }()
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: errReader{}, Header: make(http.Header)}, nil
	})}
	_, _, _, code := apiRequest(TestEnv{APIBase: "http://example.com", AccessToken: "tok"}, "/x", nil)
	if code != 1 {
		t.Fatalf("expected io error exit 1, got %d", code)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

var _ io.ReadCloser = errReader{}
