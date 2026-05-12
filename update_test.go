package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUpdateCheckUsesReleaseMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[]}`))
	}))
	defer server.Close()

	stdout, stderr, code := ExecuteWithEnv([]string{"update", "--check", "--release-url", server.URL + "/release", "--json"}, TestEnv{})
	if code != 0 || stderr != "" {
		t.Fatalf("expected update check success, code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `"latest_version": "v9.9.9"`) || !strings.Contains(stdout, `"update_available": true`) {
		t.Fatalf("unexpected update check output: %s", stdout)
	}
}

func TestUpdateInstallsVerifiedReleaseAsset(t *testing.T) {
	assetName := "whoop-cli_v9.9.9_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	archive := testTarGz(t, updateBinaryName(runtime.GOOS), []byte("fixture-binary"))
	sum := sha256.Sum256(archive)
	checksums := hex.EncodeToString(sum[:]) + "  " + assetName + "\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release":
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[{"name":"` + assetName + `","browser_download_url":"` + serverURL(r) + `/asset"},{"name":"checksums.txt","browser_download_url":"` + serverURL(r) + `/checksums"}]}`))
		case "/asset":
			_, _ = w.Write(archive)
		case "/checksums":
			_, _ = w.Write([]byte(checksums))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	installDir := t.TempDir()
	stdout, stderr, code := ExecuteWithEnv([]string{"update", "--release-url", server.URL + "/release", "--install-dir", installDir, "--json"}, TestEnv{})
	if code != 0 || stderr != "" {
		t.Fatalf("expected update success, code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	installed := filepath.Join(installDir, updateBinaryName(runtime.GOOS))
	body, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "fixture-binary" {
		t.Fatalf("unexpected installed binary: %q", string(body))
	}
	if mode := mustStatMode(t, installed); mode&0o111 == 0 {
		t.Fatalf("installed binary should be executable, mode=%v", mode)
	}
}

func TestUpdateRejectsChecksumMismatch(t *testing.T) {
	assetName := "whoop-cli_v9.9.9_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	archive := testTarGz(t, updateBinaryName(runtime.GOOS), []byte("fixture-binary"))
	badChecksum := strings.Repeat("0", 64) + "  " + assetName + "\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release":
			_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[{"name":"` + assetName + `","browser_download_url":"` + serverURL(r) + `/asset"},{"name":"checksums.txt","browser_download_url":"` + serverURL(r) + `/checksums"}]}`))
		case "/asset":
			_, _ = w.Write(archive)
		case "/checksums":
			_, _ = w.Write([]byte(badChecksum))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	_, stderr, code := ExecuteWithEnv([]string{"update", "--release-url", server.URL + "/release", "--install-dir", t.TempDir(), "--json"}, TestEnv{})
	if code != 1 || !strings.Contains(stderr, "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, code=%d stderr=%s", code, stderr)
	}
}

func testTarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}

func mustStatMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode()
}
