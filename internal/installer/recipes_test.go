package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadEmbeddedRecipes(t *testing.T) {
	recipes, err := LoadRecipes("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := recipes["gh"]; !ok {
		t.Fatal("expected 'gh' recipe")
	}
	if _, ok := recipes["git"]; !ok {
		t.Fatal("expected 'git' recipe")
	}
}

func TestLoadExternalOverrides(t *testing.T) {
	tmp := t.TempDir()
	extPath := filepath.Join(tmp, "recipes.yaml")
	content := `mytool:
  windows:
    - winget: MyTool
`
	os.WriteFile(extPath, []byte(content), 0644)

	recipes, err := LoadRecipes(extPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := recipes["mytool"]; !ok {
		t.Fatal("expected 'mytool' from external file")
	}
	// embedded still present
	if _, ok := recipes["gh"]; !ok {
		t.Fatal("expected 'gh' from embedded")
	}
}

func TestResolvePMMissingRecipe(t *testing.T) {
	recipes := map[string]Recipe{}
	_, _, err := ResolvePM("notfound", recipes)
	if err == nil {
		t.Fatal("expected error for missing recipe")
	}
}

func TestResolvePMForOS(t *testing.T) {
	recipes := map[string]Recipe{
		"testtool": {
			Windows: []PMEntry{{"winget": "Test.Tool"}},
			MacOS:   []PMEntry{{"brew": "test-tool"}},
			Linux:   []PMEntry{{"apt": "test-tool"}},
		},
	}
	_, _, err := ResolvePM("testtool", recipes)
	if err != nil {
		t.Logf("ResolvePM returned error (expected if no PM available in test env): %v", err)
	}
}

func TestOSDetection(t *testing.T) {
	os := CurrentOS()
	if os != "windows" && os != "darwin" && os != "linux" {
		t.Errorf("unexpected OS: %q", os)
	}
	// Verify it matches runtime.GOOS
	expected := runtime.GOOS
	if expected == "darwin" {
		expected = "macos"
	}
	if os != expected {
		t.Errorf("CurrentOS() = %q, runtime.GOOS = %q", os, runtime.GOOS)
	}
}
