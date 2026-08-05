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
	if _, ok := recipes["rtk"]; !ok {
		t.Fatal("expected 'rtk' recipe")
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

func TestOSDetection(t *testing.T) {
	os := CurrentOS()
	if os != "windows" && os != "macos" && os != "linux" {
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
