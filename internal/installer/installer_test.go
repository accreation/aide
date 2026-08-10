package installer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewInstaller(t *testing.T) {
	recipes := map[string]Recipe{
		"test": {
			Windows: []PMEntry{{"cmd": "/c echo installed"}},
		},
	}
	inst := New(recipes)
	if inst == nil {
		t.Fatal("expected non-nil installer")
	}
}

func TestInstallMissingRecipe(t *testing.T) {
	inst := New(map[string]Recipe{})
	err := inst.Install("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing recipe")
	}
}

func TestInstallGithubDispatch(t *testing.T) {
	recipes := map[string]Recipe{
		"mytool": {
			Windows: []PMEntry{{"github": "owner/repo mytool-${GOARCH}.zip mytool.exe"}},
			Linux:   []PMEntry{{"github": "owner/repo mytool-${GOARCH}.tar.gz mytool"}},
		},
	}
	inst := New(recipes)

	// Serve a valid release JSON with no matching assets so the test is
	// hermetic (no real GitHub API call).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{TagName: "v9.9.9"})
	}))
	defer srv.Close()

	origAPI := githubAPIBaseURL
	githubAPIBaseURL = srv.URL
	defer func() { githubAPIBaseURL = origAPI }()

	// Github install should reach the github codepath and fail on the
	// missing asset — not fall through to default exec.
	err := inst.Install("mytool")
	if err == nil {
		t.Fatal("expected error — github dispatch should attempt a real fetch")
	}
	// Error should come from asset matching on the mock release.
	if !strings.Contains(err.Error(), "no asset matching") {
		t.Fatalf("expected no-asset error, got: %v", err)
	}
}

// TestInstallWithOptionsGlobalHonorsVersionConstraint is a regression test for
// https://github.com/accreation/aide/issues/18: global-mode (non-isolated)
// installs of github-recipe tools must resolve and download a release that
// satisfies the aide.yaml version constraint, not just whatever GitHub
// currently reports as "latest".
func TestInstallWithOptionsGlobalHonorsVersionConstraint(t *testing.T) {
	const assetName = "mytool-linux.tar.gz"

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/releases":
			// Newest first: v2.0.0 violates "<2.0.0", v1.9.0 satisfies it.
			json.NewEncoder(w).Encode([]githubRelease{
				{TagName: "v2.0.0", Assets: []githubAsset{{Name: assetName, BrowserDownloadURL: srv.URL + "/dl/v2"}}},
				{TagName: "v1.9.0", Assets: []githubAsset{{Name: assetName, BrowserDownloadURL: srv.URL + "/dl/v1.9"}}},
			})
		case r.URL.Path == "/dl/v2":
			t.Error("downloaded the unconstrained latest release (v2.0.0) instead of the constrained one (v1.9.0)")
			http.NotFound(w, r)
		case r.URL.Path == "/dl/v1.9":
			// Not a real archive — installFromGithub's extraction is exercised
			// elsewhere; here we only care about which release got selected.
			w.Write([]byte("fake"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	origAPI := githubAPIBaseURL
	githubAPIBaseURL = srv.URL
	defer func() { githubAPIBaseURL = origAPI }()

	tmpDir, err := os.MkdirTemp("", "aide-global-install-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	binDirOverride = tmpDir
	defer func() { binDirOverride = "" }()

	recipes := map[string]Recipe{
		"mytool": {
			Linux:   []PMEntry{{"github": "owner/repo " + assetName + " mytool"}},
			MacOS:   []PMEntry{{"github": "owner/repo " + assetName + " mytool"}},
			Windows: []PMEntry{{"github": "owner/repo " + assetName + " mytool"}},
		},
	}
	inst := New(recipes)

	// Global mode: ProjectDir left empty.
	err = inst.InstallWithOptions("mytool", InstallOptions{Version: "<2.0.0"})
	// The extraction step will fail since "fake" isn't a real tar.gz, but by
	// that point the version resolution (the thing under test) has already run.
	if err == nil {
		t.Fatal("expected extraction error from the fake archive payload")
	}
	if strings.Contains(err.Error(), "no release found satisfying constraint") {
		t.Fatalf("version constraint was not honored: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(tmpDir, "mytool")); statErr == nil {
		t.Error("expected extraction to fail for the fake archive, but a binary was placed")
	}
}
