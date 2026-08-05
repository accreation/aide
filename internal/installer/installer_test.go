package installer

import (
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
	// Github install will fail because there's no real release, but it should
	// reach the github codepath (not fall through to default exec).
	err := inst.Install("mytool")
	if err == nil {
		t.Fatal("expected error — github dispatch should attempt a real fetch")
	}
	// Error should be from github fetch, not from exec.LookPath
	if !strings.Contains(err.Error(), "fetching release") && !strings.Contains(err.Error(), "downloading") {
		t.Fatalf("expected github-related error, got: %v", err)
	}
}
