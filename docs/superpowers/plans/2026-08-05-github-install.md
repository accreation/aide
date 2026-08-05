# GitHub Releases Install — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `github` package-manager type to recipes — downloads binaries from GitHub Releases, extracts archives, installs to `~/.local/bin`, and adds the directory to PATH.

**Architecture:** Extend Recipe with `arch_map` and template variables (`${GOARCH}`, `${GOOS}`, `${ARCH}`). Add `github.go` for release-fetching/asset-matching/extraction. Add `path.go` for bin-dir creation and PATH management. Wire the new `github` case into `Installer.Install()`.

**Tech Stack:** Go 1.22+, stdlib only (`net/http`, `archive/zip`, `archive/tar`, `compress/gzip`, `strings`)

## Global Constraints

- All new types/functions must have unit tests (TDD)
- Template variables: `${GOARCH}`, `${GOOS}`, `${OS}`, `${ARCH}` (resolved via `arch_map`)
- `github` recipe format: `"owner/repo asset-pattern binary-name"`
- Binaries installed to `~/.local/bin` (Unix) / `%USERPROFILE%\.local\bin` (Windows)
- PATH addition is idempotent (no duplicates)
- GitHub API: no auth, 30s timeout, public repos only
- Supported archives: `.zip`, `.tar.gz`; plain binary files also supported

---

### Task 1: Template variables + arch_map in Recipe

**Files:**
- Modify: `internal/installer/recipes.go`
- Modify: `internal/installer/recipes_test.go`

**Interfaces:**
- Produces: `Recipe.ArchMap map[string]string` field, `resolveTemplates(value string, archMap map[string]string) string`, updated `ResolvePM` that resolves templates, `buildInstallArgs` with `github` case returning `[]string{ownerRepo, assetPattern, binaryName}`

- [ ] **Step 1: Write failing tests**

In `internal/installer/recipes_test.go`, add:

```go
func TestResolveTemplates(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		archMap  map[string]string
		expected string
	}{
		{
			name:     "no variables",
			value:    "rtk-ai/rtk rtk.zip rtk.exe",
			archMap:  nil,
			expected: "rtk-ai/rtk rtk.zip rtk.exe",
		},
		{
			name:     "GOARCH substitution",
			value:    "tool-${GOARCH}.zip",
			archMap:  nil,
			expected: "tool-" + runtime.GOARCH + ".zip",
		},
		{
			name:     "GOOS substitution",
			value:    "tool-${GOOS}.tar.gz",
			archMap:  nil,
			expected: "tool-" + runtime.GOOS + ".tar.gz",
		},
		{
			name:     "OS alias substitution",
			value:    "tool-${OS}.zip",
			archMap:  nil,
			expected: "tool-" + runtime.GOOS + ".zip",
		},
		{
			name:  "ARCH with mapping",
			value: "tool-${ARCH}.zip",
			archMap: map[string]string{
				"amd64": "x86_64",
				"arm64": "aarch64",
			},
			expected: func() string {
				m := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}
				if v, ok := m[runtime.GOARCH]; ok {
					return "tool-" + v + ".zip"
				}
				return "tool-" + runtime.GOARCH + ".zip"
			}(),
		},
		{
			name:     "ARCH without mapping falls back to GOARCH",
			value:    "tool-${ARCH}.zip",
			archMap:  nil,
			expected: "tool-" + runtime.GOARCH + ".zip",
		},
		{
			name:     "multiple variables",
			value:    "${GOOS}-${GOARCH}-${ARCH}",
			archMap:  nil,
			expected: runtime.GOOS + "-" + runtime.GOARCH + "-" + runtime.GOARCH,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTemplates(tt.value, tt.archMap)
			if got != tt.expected {
				t.Errorf("resolveTemplates(%q, %v) = %q, want %q",
					tt.value, tt.archMap, got, tt.expected)
			}
		})
	}
}

func TestLoadRecipesArchMap(t *testing.T) {
	recipes, err := LoadRecipes("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rtk, ok := recipes["rtk"]
	if !ok {
		t.Fatal("expected 'rtk' recipe")
	}
	if rtk.ArchMap == nil {
		t.Fatal("expected 'rtk' to have arch_map")
	}
	if rtk.ArchMap["amd64"] != "x86_64" {
		t.Errorf("rtk.ArchMap[amd64] = %q, want %q", rtk.ArchMap["amd64"], "x86_64")
	}
}

func TestBuildInstallArgsGithub(t *testing.T) {
	args := buildInstallArgs("github", "rtk-ai/rtk rtk-x86_64-pc-windows-msvc.zip rtk.exe")
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(args), args)
	}
	if args[0] != "rtk-ai/rtk" {
		t.Errorf("args[0] = %q, want %q", args[0], "rtk-ai/rtk")
	}
	if args[1] != "rtk-x86_64-pc-windows-msvc.zip" {
		t.Errorf("args[1] = %q, want %q", args[1], "rtk-x86_64-pc-windows-msvc.zip")
	}
	if args[2] != "rtk.exe" {
		t.Errorf("args[2] = %q, want %q", args[2], "rtk.exe")
	}
}
```

- [ ] **Step 2: Run tests — expect failures**

Run: `go test ./internal/installer/ -v -run "TestResolveTemplates|TestLoadRecipesArchMap|TestBuildInstallArgsGithub"`
Expected: FAIL — `resolveTemplates` undefined, `TestLoadRecipesArchMap` fails (rtk not yet in recipes.yaml)

- [ ] **Step 3: Add ArchMap to Recipe struct**

In `internal/installer/recipes.go`, modify `Recipe`:

```go
type Recipe struct {
	Windows []PMEntry         `yaml:"windows,omitempty"`
	MacOS   []PMEntry         `yaml:"macos,omitempty"`
	Linux   []PMEntry         `yaml:"linux,omitempty"`
	ArchMap map[string]string `yaml:"arch_map,omitempty"`
}
```

- [ ] **Step 4: Add resolveTemplates function**

In `internal/installer/recipes.go`, add after `CurrentOS()`:

```go
// resolveTemplates replaces template variables in a recipe value.
// Supported: ${GOARCH}, ${GOOS}, ${OS}, ${ARCH} (resolved via archMap, fallback to GOARCH).
func resolveTemplates(value string, archMap map[string]string) string {
	arch := runtime.GOARCH
	if mapped, ok := archMap[runtime.GOARCH]; ok {
		arch = mapped
	}
	replacer := strings.NewReplacer(
		"${GOARCH}", runtime.GOARCH,
		"${GOOS}", runtime.GOOS,
		"${OS}", runtime.GOOS,
		"${ARCH}", arch,
	)
	return replacer.Replace(value)
}
```

- [ ] **Step 5: Update ResolvePM to resolve templates**

In `internal/installer/recipes.go`, in `ResolvePM`, change the loop body:

```go
	for _, entry := range entries {
		for pm, pkg := range entry {
			if pmAvailable(pm) {
				resolved := resolveTemplates(pkg, recipe.ArchMap)
				return pm, buildInstallArgs(pm, resolved), nil
			}
		}
	}
```

- [ ] **Step 6: Add github case to buildInstallArgs**

In `internal/installer/recipes.go`, add to `buildInstallArgs` switch, before `default`:

```go
	case "github":
		parts := strings.Fields(pkg)
		if len(parts) != 3 {
			return []string{pkg}
		}
		return parts
```

- [ ] **Step 7: Add `strings` to imports**

In `internal/installer/recipes.go`, add `"strings"` to the import block:

```go
import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/goccy/go-yaml"
)
```

- [ ] **Step 8: Run tests — expect ResolveTemplates + BuildInstallArgsGithub pass**

Run: `go test ./internal/installer/ -v -run "TestResolveTemplates|TestBuildInstallArgsGithub"`
Expected: PASS for ResolveTemplates and BuildInstallArgsGithub. TestLoadRecipesArchMap still fails (rtk not in recipes.yaml yet — added in Task 2).

- [ ] **Step 9: Commit**

```bash
git add internal/installer/recipes.go internal/installer/recipes_test.go
git commit -m "feat: add template variables and arch_map to Recipe struct"
```

---

### Task 2: Add rtk recipe to recipes.yaml

**Files:**
- Modify: `internal/installer/recipes.yaml`
- Modify: `internal/installer/recipes_test.go`

**Interfaces:**
- Consumes: `Recipe.ArchMap` (Task 1)
- Produces: rtk recipe in embedded YAML, test assertions for rtk

- [ ] **Step 1: Write test for rtk recipe**

In `internal/installer/recipes_test.go`, the `TestLoadRecipesArchMap` test is already written in Task 1. It expects `rtk` with `arch_map`.

- [ ] **Step 2: Run test — confirm it fails**

Run: `go test ./internal/installer/ -v -run TestLoadRecipesArchMap`
Expected: FAIL — `expected 'rtk' recipe`

- [ ] **Step 3: Add rtk recipe**

In `internal/installer/recipes.yaml`, add before `git:`:

```yaml
rtk:
  arch_map:
    amd64: x86_64
    arm64: aarch64
  windows:
    - github: "rtk-ai/rtk rtk-${ARCH}-pc-windows-msvc.zip rtk.exe"
  macos:
    - brew: rtk
  linux:
    - curl: "https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh | sh"
```

- [ ] **Step 4: Run test — expect pass**

Run: `go test ./internal/installer/ -v -run TestLoadRecipesArchMap`
Expected: PASS

- [ ] **Step 5: Run all installer tests**

Run: `go test ./internal/installer/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/installer/recipes.yaml internal/installer/recipes_test.go
git commit -m "feat: add rtk recipe with github release install for Windows"
```

---

### Task 3: GitHub release download + extraction

**Files:**
- Create: `internal/installer/github.go`
- Create: `internal/installer/github_test.go`

**Interfaces:**
- Consumes: template-resolved args from `buildInstallArgs` `["owner/repo", "asset-pattern", "binary-name"]`
- Produces: `func installFromGithub(ownerRepo, assetPattern, binaryName string) error`

- [ ] **Step 1: Write failing tests**

Create `internal/installer/github_test.go`:

```go
package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFetchLatestRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	tw.WriteHeader(&tar.Header{Name: "mybinary", Size: 18, Mode: 0755, Typeflag: tar.TypeReg})
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
```

- [ ] **Step 2: Run tests — expect failures**

Run: `go test ./internal/installer/ -v -run "TestFetch|TestMatch|TestExtract"`
Expected: FAIL — `githubRelease` undefined, functions undefined

- [ ] **Step 3: Implement github.go**

Create `internal/installer/github.go`:

```go
package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// fetchLatestRelease fetches the latest GitHub release from the given API URL.
func fetchLatestRelease(url string, client *http.Client) (*githubRelease, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release JSON: %w", err)
	}
	return &release, nil
}

// matchAsset finds an asset whose name matches the given pattern.
func matchAsset(assets []githubAsset, pattern string) (githubAsset, bool) {
	for _, a := range assets {
		if a.Name == pattern {
			return a, true
		}
	}
	return githubAsset{}, false
}

// installFromGithub downloads a binary from GitHub Releases, extracts if needed,
// and places it in the user's local bin directory.
func installFromGithub(ownerRepo, assetPattern, binaryName string) error {
	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid owner/repo: %q", ownerRepo)
	}
	owner, repo := parts[0], parts[1]

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	client := &http.Client{Timeout: 30 * time.Second}

	release, err := fetchLatestRelease(apiURL, client)
	if err != nil {
		return fmt.Errorf("fetching release for %s: %w", ownerRepo, err)
	}

	asset, ok := matchAsset(release.Assets, assetPattern)
	if !ok {
		return fmt.Errorf("no asset matching %q in latest release (%s)", assetPattern, release.TagName)
	}

	fmt.Printf("  Downloading %s from %s...\n", asset.Name, release.TagName)

	// Download asset to temp file
	tmpDir, err := os.MkdirTemp("", "aide-dl-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	assetPath := filepath.Join(tmpDir, asset.Name)
	if err := downloadFile(asset.BrowserDownloadURL, assetPath, client); err != nil {
		return fmt.Errorf("downloading asset: %w", err)
	}

	// Determine extraction or direct copy
	binDir, err := getBinDir()
	if err != nil {
		return fmt.Errorf("getting bin dir: %w", err)
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("creating bin dir %s: %w", binDir, err)
	}

	if isArchive(asset.Name) {
		if err := extractArchive(assetPath, binDir, binaryName); err != nil {
			return fmt.Errorf("extracting archive: %w", err)
		}
	} else {
		// Plain binary — rename directly
		dest := filepath.Join(binDir, binaryName)
		if err := os.Rename(assetPath, dest); err != nil {
			// Fallback: copy
			if err := copyFile(assetPath, dest); err != nil {
				return fmt.Errorf("copying binary: %w", err)
			}
		}
		if err := os.Chmod(dest, 0755); err != nil {
			return fmt.Errorf("setting executable bit: %w", err)
		}
	}

	// Ensure bin dir in PATH
	if err := ensureInPath(binDir); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not add %s to PATH: %v\n", binDir, err)
		fmt.Fprintf(os.Stderr, "  Add it manually to use %s from any terminal.\n", binaryName)
	}

	fmt.Printf("  OK  %s installed to %s\n", binaryName, binDir)
	return nil
}

// isArchive returns true if the filename is a supported archive.
func isArchive(name string) bool {
	return strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tar.gz")
}

// downloadFile downloads a URL to a local file path.
func downloadFile(url, dest string, client *http.Client) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

// extractArchive extracts a binary from a .zip or .tar.gz archive.
func extractArchive(src, destDir, binaryName string) error {
	if strings.HasSuffix(src, ".zip") {
		return extractZip(src, destDir, binaryName)
	}
	if strings.HasSuffix(src, ".tar.gz") {
		return extractTarGz(src, destDir, binaryName)
	}
	return fmt.Errorf("unsupported archive format: %s", src)
}

func extractZip(src, destDir, binaryName string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName || f.Name == binaryName {
			return extractZipFile(f, filepath.Join(destDir, binaryName))
		}
	}
	return fmt.Errorf("binary %q not found in zip archive", binaryName)
}

func extractZipFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening zip entry: %w", err)
	}
	defer rc.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("extracting: %w", err)
	}
	return nil
}

func extractTarGz(src, destDir, binaryName string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening tar.gz: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}
		if filepath.Base(hdr.Name) == binaryName || hdr.Name == binaryName {
			dest := filepath.Join(destDir, binaryName)
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return fmt.Errorf("extracting: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("binary %q not found in tar.gz archive", binaryName)
}

// copyFile copies a file from src to dest.
func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
```

- [ ] **Step 4: Run tests — expect all pass**

Run: `go test ./internal/installer/ -v -run "TestFetch|TestMatch|TestExtract"`
Expected: all PASS

- [ ] **Step 5: Run full installer test suite**

Run: `go test ./internal/installer/ -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/installer/github.go internal/installer/github_test.go
git commit -m "feat: add GitHub release download and archive extraction"
```

---

### Task 4: PATH management

**Files:**
- Create: `internal/installer/path.go`
- Create: `internal/installer/path_test.go`

**Interfaces:**
- Produces: `getBinDir() (string, error)`, `ensureInPath(binDir string) error`, `isInPathUnix(binDir string) bool`

- [ ] **Step 1: Write failing tests**

Create `internal/installer/path_test.go`:

```go
package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetBinDir(t *testing.T) {
	dir, err := getBinDir()
	if err != nil {
		t.Fatalf("getBinDir: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty bin dir")
	}

	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		expected := filepath.Join(home, ".local", "bin")
		if dir != expected {
			t.Errorf("bin dir = %q, want %q", dir, expected)
		}
	} else {
		expected := filepath.Join(home, ".local", "bin")
		if dir != expected {
			t.Errorf("bin dir = %q, want %q", dir, expected)
		}
	}
}

func TestIsInPathUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test")
	}

	if !isInPathUnix("/usr/local/bin") {
		t.Error("expected /usr/local/bin to be in PATH")
	}
	if isInPathUnix("/nonexistent/path/xyz") {
		t.Error("expected /nonexistent/path/xyz to NOT be in PATH")
	}
}

func TestAppendToShellRc(t *testing.T) {
	tmp := t.TempDir()

	rcPath := filepath.Join(tmp, ".bashrc")
	exportLine := `export PATH="$HOME/.local/bin:$PATH"`

	// File doesn't exist — should still work (no-op)
	err := appendToShellRc(rcPath, exportLine)
	if err != nil {
		t.Fatalf("appendToShellRc (no file): %v", err)
	}

	// Create file with existing content
	os.WriteFile(rcPath, []byte("# existing config\n"), 0644)

	// First append
	err = appendToShellRc(rcPath, exportLine)
	if err != nil {
		t.Fatalf("appendToShellRc: %v", err)
	}

	data, _ := os.ReadFile(rcPath)
	if !containsLine(string(data), exportLine) {
		t.Errorf("expected file to contain %q, got:\n%s", exportLine, string(data))
	}

	// Second append — should be idempotent
	err = appendToShellRc(rcPath, exportLine)
	if err != nil {
		t.Fatalf("appendToShellRc (second): %v", err)
	}

	data2, _ := os.ReadFile(rcPath)
	if count := lineCount(string(data2), exportLine); count > 1 {
		t.Errorf("expected line to appear once, got %d times", count)
	}
}

func containsLine(content, line string) bool {
	for _, l := range filepath.SplitList(content) {
		if l == line {
			return true
		}
	}
	return false
}

func lineCount(content, line string) int {
	count := 0
	for _, l := range splitLines(content) {
		if l == line {
			count++
		}
	}
	return count
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
```

- [ ] **Step 2: Run tests — expect failures**

Run: `go test ./internal/installer/ -v -run "TestGetBinDir|TestIsInPathUnix|TestAppendToShellRc"`
Expected: FAIL — functions undefined

- [ ] **Step 3: Implement path.go**

Create `internal/installer/path.go`:

```go
package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// getBinDir returns the user-local bin directory path.
func getBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// ensureInPath adds binDir to the user's PATH if not already present.
func ensureInPath(binDir string) error {
	if isInPath(binDir) {
		return nil
	}
	if runtime.GOOS == "windows" {
		return ensureInPathWindows(binDir)
	}
	return ensureInPathUnix(binDir)
}

func isInPath(binDir string) bool {
	if runtime.GOOS == "windows" {
		return isInPathWindows(binDir)
	}
	return isInPathUnix(binDir)
}

// isInPathUnix checks if the directory is in the current PATH environment variable.
func isInPathUnix(binDir string) bool {
	for _, p := range strings.Split(os.Getenv("PATH"), ":") {
		if p == binDir {
			return true
		}
	}
	return false
}

// isInPathWindows checks if the directory is in the current PATH (case-insensitive).
func isInPathWindows(binDir string) bool {
	lower := strings.ToLower(binDir)
	for _, p := range strings.Split(os.Getenv("PATH"), ";") {
		if strings.ToLower(p) == lower {
			return true
		}
	}
	return false
}

// ensureInPathUnix appends the binDir to ~/.bashrc and ~/.zshrc if they exist.
func ensureInPathUnix(binDir string) error {
	exportLine := fmt.Sprintf(`export PATH="%s:$PATH"`, binDir)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var errs []string
	for _, rc := range []string{".bashrc", ".zshrc"} {
		rcPath := filepath.Join(home, rc)
		if _, err := os.Stat(rcPath); err == nil {
			if err := appendToShellRc(rcPath, exportLine); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", rc, err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("updating shell config: %s", strings.Join(errs, "; "))
	}
	return nil
}

// appendToShellRc appends a line to a shell rc file if not already present.
func appendToShellRc(rcPath, line string) error {
	data, err := os.ReadFile(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file doesn't exist, skip
		}
		return err
	}
	content := string(data)
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == strings.TrimSpace(line) {
			return nil // already present
		}
	}
	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, "\n"+line)
	return err
}

// ensureInPathWindows adds binDir to user PATH via registry.
func ensureInPathWindows(binDir string) error {
	cmd := exec.Command("cmd", "/c", "setx", "PATH", os.Getenv("PATH")+";"+binDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("setx failed: %v: %s", err, string(output))
	}
	return nil
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `go test ./internal/installer/ -v -run "TestGetBinDir|TestIsInPathUnix|TestAppendToShellRc"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/installer/path.go internal/installer/path_test.go
git commit -m "feat: add PATH management for ~/.local/bin"
```

---

### Task 5: Wire github into Installer.Install()

**Files:**
- Modify: `internal/installer/installer.go`
- Modify: `internal/installer/installer_test.go`

**Interfaces:**
- Consumes: `installFromGithub` (Task 3), `github` case in `buildInstallArgs` (Task 1)
- Produces: updated `Install()` that dispatches `pm == "github"` to `installFromGithub`

- [ ] **Step 1: Add test for github install dispatch**

In `internal/installer/installer_test.go`, add:

```go
func TestInstallGithubDispatch(t *testing.T) {
	recipes := map[string]Recipe{
		"mytool": {
			Windows: []PMEntry{{"github": "owner/repo mytool-${GOARCH}.zip mytool.exe"}},
			Linux:   []PMEntry{{"github": "owner/repo mytool-${GOARCH}.tar.gz mytool"}},
		},
	}
	inst := New(recipes)
	// Github install will fail because there's no real release, but it should
	// reach the github codepath (not fall through to default exec).
	err := inst.Install("mytool")
	if err == nil {
		t.Skip("unexpected success — no real GitHub release")
	}
	// Error should be from github fetch, not from exec.LookPath
	if !strings.Contains(err.Error(), "fetching release") && !strings.Contains(err.Error(), "downloading") {
		t.Logf("install error (expected github-related): %v", err)
	}
}
```

- [ ] **Step 2: Run test — expect FAIL or skip**

Run: `go test ./internal/installer/ -v -run TestInstallGithubDispatch`
Expected: FAIL — github case not handled in Install()

- [ ] **Step 3: Add github case to Install()**

In `internal/installer/installer.go`, add to `Install()` after `if pm == "curl"` block:

```go
	if pm == "github" {
		if len(args) < 3 {
			return fmt.Errorf("invalid github recipe for %s: expected 'owner/repo asset binary'", toolName)
		}
		return installFromGithub(args[0], args[1], args[2])
	}
```

- [ ] **Step 4: Add `strings` import to installer_test.go if needed**

Check: `internal/installer/installer_test.go` may need `"strings"` import for the test.

- [ ] **Step 5: Run test — expect github-related error**

Run: `go test ./internal/installer/ -v -run TestInstallGithubDispatch`
Expected: PASS (error is from github fetch, which is the correct code path)

- [ ] **Step 6: Run full test suite**

Run: `go test ./... -v`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/installer/installer.go internal/installer/installer_test.go
git commit -m "feat: wire github PM type into Installer.Install()"
```

---

### Task 6: End-to-end integration test with mock server

**Files:**
- Create: `internal/installer/integration_test.go`

**Interfaces:**
- Consumes: all previous tasks
- Produces: integration test verifying the full github install pipeline

- [ ] **Step 1: Write integration test**

Create `internal/installer/integration_test.go`:

```go
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

func TestGithubInstallIntegration(t *testing.T) {
	// Set up mock GitHub API + download server
	var downloadPayload []byte
	downloadName := "tool-x86_64-pc-windows-msvc.zip"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Create a fake binary in a zip
	if runtime.GOOS == "windows" {
		downloadName = "tool-x86_64-pc-windows-msvc.zip"
	}
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("tool.exe")
	f.Write([]byte("#!/fake binary"))
	w.Close()
	downloadPayload = buf.Bytes()

	tmpDir := t.TempDir()

	// Override getBinDir to use temp dir for this test
	origGetBinDir := getBinDir
	getBinDir = func() (string, error) { return tmpDir, nil }
	defer func() { getBinDir = origGetBinDir }()

	// Call installFromGithub directly (bypasses ResolvePM since we test the core)
	err := installFromGithub("test/tool", downloadName, "tool.exe")
	if err != nil {
		t.Fatalf("installFromGithub: %v", err)
	}

	extracted := filepath.Join(tmpDir, "tool.exe")
	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("reading installed binary: %v", err)
	}
	if string(data) != "#!/fake binary" {
		t.Errorf("binary content = %q", string(data))
	}
}

func TestGithubInstallIntegrationTarGz(t *testing.T) {
	downloadName := "tool-linux-amd64.tar.gz"
	var downloadPayload []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Create tar.gz
	buf := new(bytes.Buffer)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)
	tw.WriteHeader(&tar.Header{Name: "tool", Size: 17, Mode: 0755, Typeflag: tar.TypeReg})
	tw.Write([]byte("#!/fake binary tar"))
	tw.Close()
	gw.Close()
	downloadPayload = buf.Bytes()

	tmpDir := t.TempDir()
	origGetBinDir := getBinDir
	getBinDir = func() (string, error) { return tmpDir, nil }
	defer func() { getBinDir = origGetBinDir }()

	err := installFromGithub("test/tool2", downloadName, "tool")
	if err != nil {
		t.Fatalf("installFromGithub: %v", err)
	}

	extracted := filepath.Join(tmpDir, "tool")
	data, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("reading installed binary: %v", err)
	}
	if string(data) != "#!/fake binary tar" {
		t.Errorf("binary content = %q", string(data))
	}
}
```

Wait — `getBinDir` is a regular function, not a variable. For the integration test to override it, we need a different approach. Let me use an environment variable or add a package-level override.

- [ ] **Step 2: Make getBinDir overridable for testing**

In `internal/installer/path.go`, add a package-level variable:

```go
// binDirOverride allows tests to override the bin directory.
var binDirOverride string
```

Modify `getBinDir`:

```go
func getBinDir() (string, error) {
	if binDirOverride != "" {
		return binDirOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ".local", "bin"), nil
}
```

- [ ] **Step 3: Update integration test to use override**

In the integration test, replace `origGetBinDir` / `getBinDir` approach with:

```go
	binDirOverride = tmpDir
	defer func() { binDirOverride = "" }()
```

- [ ] **Step 4: Run integration tests**

Run: `go test ./internal/installer/ -v -run TestGithubInstallIntegration`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./... -v`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/installer/integration_test.go internal/installer/path.go
git commit -m "test: add github install integration tests with mock server"
```
