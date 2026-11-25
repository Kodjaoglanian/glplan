package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.2.1", "v0.2.1", 0},
		{"0.2.1", "v0.2.2", -1},
		{"v0.3.0", "v0.2.9", 1},
		{"v1.0.0", "v0.9.9", 1},
		{"v0.2.1", "v0.2.1-rc.1", 0},
		{"dev", "v0.2.1", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Fatalf("compareVersions(%q,%q)=%d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestUpdateAssetName(t *testing.T) {
	if got := updateAssetName("v0.2.2", "linux", "amd64"); got != "glplan-v0.2.2-linux-amd64.tar.gz" {
		t.Fatalf("linux asset: %s", got)
	}
	if got := updateAssetName("v0.2.2", "windows", "arm64"); got != "glplan-v0.2.2-windows-arm64.zip" {
		t.Fatalf("windows asset: %s", got)
	}
}

func buildTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractBinaryTarGz(t *testing.T) {
	want := []byte("fake-binary")
	archive := buildTarGz(t, "glplan", want)
	got, err := extractBinary(archive, "glplan-v1-linux-amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("extracted %q, want %q", got, want)
	}
}

func TestVerifySHA256(t *testing.T) {
	data := []byte("payload")
	sum := sha256.Sum256(data)
	line := fmt.Sprintf("%s  glplan-x.tar.gz\n", hex.EncodeToString(sum[:]))
	if err := verifySHA256(data, []byte(line)); err != nil {
		t.Fatalf("valid checksum rejected: %v", err)
	}
	if err := verifySHA256([]byte("other"), []byte(line)); err == nil {
		t.Fatal("invalid checksum accepted")
	}
}

func TestSelfUpdateFlowWithMockServer(t *testing.T) {
	asset := fmt.Sprintf("glplan-v9.9.9-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		asset = strings.TrimSuffix(asset, ".tar.gz") + ".zip"
	}
	if runtime.GOOS == "windows" {
		t.Skip("fluxo completo testado em formato tar.gz no linux")
	}
	archive := buildTarGz(t, "glplan", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	sumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), asset)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v9.9.9","assets":[
			{"name":%q,"browser_download_url":%q},
			{"name":%q,"browser_download_url":%q}
		]}`, asset, srv.URL+"/asset", asset+".sha256", srv.URL+"/sum")
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/sum", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(sumLine)) })

	oldURL, oldReplace := latestReleaseURL, replaceBinaryFn
	defer func() { latestReleaseURL, replaceBinaryFn = oldURL, oldReplace }()
	latestReleaseURL = srv.URL + "/latest"
	var installed []byte
	replaceBinaryFn = func(binary []byte) (string, error) {
		installed = binary
		return "/fake/glplan", nil
	}

	msg, err := selfUpdate("v0.2.1")
	if err != nil {
		t.Fatalf("selfUpdate: %v", err)
	}
	if !strings.Contains(msg, "v0.2.1 → v9.9.9") {
		t.Fatalf("unexpected message: %s", msg)
	}
	if string(installed) != "new-binary" {
		t.Fatalf("installed binary mismatch: %q", installed)
	}
}

func TestSelfUpdateAlreadyCurrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v0.2.1","assets":[]}`)
	}))
	defer srv.Close()
	oldURL := latestReleaseURL
	defer func() { latestReleaseURL = oldURL }()
	latestReleaseURL = srv.URL

	msg, err := selfUpdate("v0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "atualizado") {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestSelfUpdateRejectsCorruptChecksum(t *testing.T) {
	asset := fmt.Sprintf("glplan-v9.9.9-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		t.Skip("formato tar.gz")
	}
	archive := buildTarGz(t, "glplan", []byte("new-binary"))

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v9.9.9","assets":[
			{"name":%q,"browser_download_url":%q},
			{"name":%q,"browser_download_url":%q}
		]}`, asset, srv.URL+"/asset", asset+".sha256", srv.URL+"/sum")
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/sum", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("deadbeef  x\n")) })

	oldURL, oldReplace := latestReleaseURL, replaceBinaryFn
	defer func() { latestReleaseURL, replaceBinaryFn = oldURL, oldReplace }()
	latestReleaseURL = srv.URL + "/latest"
	called := false
	replaceBinaryFn = func(binary []byte) (string, error) { called = true; return "", nil }

	if _, err := selfUpdate("v0.2.1"); err == nil {
		t.Fatal("expected checksum error")
	}
	if called {
		t.Fatal("binary must not be installed when checksum fails")
	}
}
