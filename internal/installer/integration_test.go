package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestGithubInstallIntegration exercises the full github install pipeline with a
// mock server: API call -> asset match -> download -> zip extraction -> placement.
func TestGithubInstallIntegration(t *testing.T) {
	downloadName := "tool-x86_64-pc-windows-msvc.zip"
	payload := []byte("#!/fake binary")

	// Build a zip containing the fake binary.
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	f, err := zw.Create("tool.exe")
	if err != nil {
		t.Fatalf("creating zip entry: %v", err)
	}
	if _, err := f.Write(payload); err != nil {
		t.Fatalf("writing zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip: %v", err)
	}
	downloadPayload := buf.Bytes()

	// Mock GitHub API + download server.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/test/tool/releases/latest":
			json.NewEncoder(w).Encode(githubRelease{
				TagName: "v2.0.0",
				Assets: []githubAsset{
					{
						Name:               downloadName,
						BrowserDownloadURL: srv.URL + "/dl/" + downloadName,
					},
				},
			})
		case "/dl/" + downloadName:
			w.Write(downloadPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tmpDir := t.TempDir()

	// Redirect bin installation to the temp dir for this test.
	binDirOverride = tmpDir
	defer func() { binDirOverride = "" }()

	// Point the GitHub API at the mock server.
	origAPI := githubAPIBaseURL
	githubAPIBaseURL = srv.URL
	defer func() { githubAPIBaseURL = origAPI }()

	// Call installFromGithub directly (bypasses ResolvePM since we test the core).
	if err := installFromGithub("test/tool", downloadName, "tool.exe", "", nil); err != nil {
		t.Fatalf("installFromGithub: %v", err)
	}

	extracted := filepath.Join(tmpDir, "tool.exe")
	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("reading installed binary: %v", err)
	}
	if string(data) != string(payload) {
		t.Errorf("binary content = %q, want %q", string(data), string(payload))
	}
}

// TestGithubInstallIntegrationTarGz exercises the same pipeline with a tar.gz asset.
func TestGithubInstallIntegrationTarGz(t *testing.T) {
	downloadName := "tool-linux-amd64.tar.gz"
	payload := []byte("#!/fake binary tar")

	// Build a tar.gz containing the fake binary.
	buf := new(bytes.Buffer)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "tool", Size: int64(len(payload)), Mode: 0755, Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("writing tar header: %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("writing tar entry: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("closing tar: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("closing gzip: %v", err)
	}
	downloadPayload := buf.Bytes()

	// Mock GitHub API + download server.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/test/tool2/releases/latest":
			json.NewEncoder(w).Encode(githubRelease{
				TagName: "v1.0.0",
				Assets: []githubAsset{
					{
						Name:               downloadName,
						BrowserDownloadURL: srv.URL + "/dl/" + downloadName,
					},
				},
			})
		case "/dl/" + downloadName:
			w.Write(downloadPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tmpDir := t.TempDir()

	// Redirect bin installation to the temp dir for this test.
	binDirOverride = tmpDir
	defer func() { binDirOverride = "" }()

	// Point the GitHub API at the mock server.
	origAPI := githubAPIBaseURL
	githubAPIBaseURL = srv.URL
	defer func() { githubAPIBaseURL = origAPI }()

	if err := installFromGithub("test/tool2", downloadName, "tool", "", nil); err != nil {
		t.Fatalf("installFromGithub: %v", err)
	}

	extracted := filepath.Join(tmpDir, "tool")
	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("reading installed binary: %v", err)
	}
	if string(data) != string(payload) {
		t.Errorf("binary content = %q, want %q", string(data), string(payload))
	}
}
