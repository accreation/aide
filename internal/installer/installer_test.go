package installer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
