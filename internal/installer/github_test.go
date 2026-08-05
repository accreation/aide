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
	"runtime"
	"testing"
)

func TestFetchLatestRelease(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/latest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := githubRelease{
			TagName: "v1.0.0",
			Assets: []githubAsset{
				{Name: "tool-linux-amd64.tar.gz", BrowserDownloadURL: srv.URL + "/dl/tool-linux-amd64.tar.gz"},
				{Name: "tool-windows-amd64.zip", BrowserDownloadURL: srv.URL + "/dl/tool-windows-amd64.zip"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	release, err := fetchLatestRelease(srv.URL+"/repos/owner/repo/releases/latest", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.TagName != "v1.0.0" {
		t.Errorf("tag = %q, want v1.0.0", release.TagName)
	}
	if len(release.Assets) != 2 {
		t.Fatalf("got %d assets, want 2", len(release.Assets))
	}
}

func TestMatchAsset(t *testing.T) {
	assets := []githubAsset{
		{Name: "tool-linux-amd64.tar.gz", BrowserDownloadURL: "http://example.com/dl/linux"},
		{Name: "tool-x86_64-pc-windows-msvc.zip", BrowserDownloadURL: "http://example.com/dl/windows"},
		{Name: "tool-aarch64-apple-darwin.tar.gz", BrowserDownloadURL: "http://example.com/dl/mac"},
	}

	asset, ok := matchAsset(assets, "tool-x86_64-pc-windows-msvc.zip")
	if !ok {
		t.Fatal("expected to match asset")
	}
	if asset.Name != "tool-x86_64-pc-windows-msvc.zip" {
		t.Errorf("matched %q, want tool-x86_64-pc-windows-msvc.zip", asset.Name)
	}

	_, ok = matchAsset(assets, "nonexistent.zip")
	if ok {
		t.Fatal("expected no match for nonexistent")
	}
}

func TestExtractZip(t *testing.T) {
	tmp := t.TempDir()

	// Create a zip with a binary inside
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("mybinary.exe")
	f.Write([]byte("fake binary content"))
	w.Close()

	src := filepath.Join(tmp, "test.zip")
	os.WriteFile(src, buf.Bytes(), 0644)

	err := extractArchive(src, tmp, "mybinary.exe")
	if err != nil {
		t.Fatalf("extractArchive: %v", err)
	}

	extracted := filepath.Join(tmp, "mybinary.exe")
	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(data) != "fake binary content" {
		t.Errorf("content = %q, want 'fake binary content'", string(data))
	}
}

func TestExtractTarGz(t *testing.T) {
	tmp := t.TempDir()

	// Create a tar.gz with a binary inside
	buf := new(bytes.Buffer)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	tw.WriteHeader(&tar.Header{Name: "mybinary", Size: int64(len("fake binary content")), Mode: 0755, Typeflag: tar.TypeReg})
	tw.Write([]byte("fake binary content"))
	tw.Close()
	gw.Close()

	src := filepath.Join(tmp, "test.tar.gz")
	os.WriteFile(src, buf.Bytes(), 0644)

	err := extractArchive(src, tmp, "mybinary")
	if err != nil {
		t.Fatalf("extractArchive: %v", err)
	}

	extracted := filepath.Join(tmp, "mybinary")
	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(data) != "fake binary content" {
		t.Errorf("content = %q, want 'fake binary content'", string(data))
	}

	// Check executable bit on Unix
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(extracted)
		if info.Mode()&0111 == 0 {
			t.Error("extracted binary is not executable")
		}
	}
}

func TestExtractBinaryNotFound(t *testing.T) {
	tmp := t.TempDir()

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("other.exe")
	f.Write([]byte("content"))
	w.Close()

	src := filepath.Join(tmp, "test.zip")
	os.WriteFile(src, buf.Bytes(), 0644)

	err := extractArchive(src, tmp, "mybinary.exe")
	if err == nil {
		t.Fatal("expected error for missing binary in archive")
	}
}
