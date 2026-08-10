package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"aide/internal/config"
	"aide/internal/display"
)

// writeFakeVersionTool creates an executable that prints version info to
// stderr (mimicking `java -version`) with a zero exit code, and prepends its
// directory to PATH for the duration of the test.
func writeFakeVersionTool(t *testing.T, name, version string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell script tool not supported on windows")
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, name)
	script := fmt.Sprintf("#!/bin/sh\necho '%s version \"%s\"' >&2\nexit 0\n", name, version)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake tool: %v", err)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
}

func TestCheckProviderFound(t *testing.T) {
	// Use a tool we know exists on almost all systems: "go"
	c := New(&config.Config{Provider: "go"})
	result := c.CheckProvider()
	if !result.Ok {
		t.Logf("go not found in PATH (may be expected in some envs): %+v", result)
	} else if result.Name != "go" {
		t.Errorf("expected name 'go', got %q", result.Name)
	}
}

func TestCheckProviderNotFound(t *testing.T) {
	c := New(&config.Config{Provider: "nonexistent-tool-xyz-123"})
	result := c.CheckProvider()
	if result.Ok {
		t.Error("expected provider not found")
	}
}

func TestCheckToolsFound(t *testing.T) {
	c := New(&config.Config{
		Provider: "go",
		Tools: []config.Tool{
			{Name: "go"},
		},
	})
	results := c.CheckTools()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Ok {
		t.Logf("go not found (may be expected): %+v", results[0])
	}
}

func TestCheckToolsNotFound(t *testing.T) {
	c := New(&config.Config{
		Provider: "go",
		Tools: []config.Tool{
			{Name: "nonexistent-tool-xyz-123"},
		},
	})
	results := c.CheckTools()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Ok {
		t.Error("expected tool not found")
	}
}

func TestCheckToolsVersionConstraint(t *testing.T) {
	c := New(&config.Config{
		Provider: "go",
		Tools: []config.Tool{
			{Name: "go", Version: ">=0.0.1"},
			{Name: "go", Version: ">=999.0.0"},
		},
	})
	results := c.CheckTools()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Ok {
		t.Errorf("go should satisfy >=0.0.1: %+v", results[0])
	}
	if results[1].Ok {
		t.Errorf("go should not satisfy >=999.0.0: %+v", results[1])
	}
}

func TestCheckToolsVersionOnStderr(t *testing.T) {
	writeFakeVersionTool(t, "fakejava", "17.0.9")

	c := New(&config.Config{
		Provider: "go",
		Tools: []config.Tool{
			{Name: "fakejava", Version: ">=11.0.0"},
		},
	})
	results := c.CheckTools()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if !r.Found {
		t.Errorf("expected Found=true for a binary on PATH, got %+v", r)
	}
	if r.Installed != "17.0.9" {
		t.Errorf("expected version parsed from stderr output, got %+v", r)
	}
	if !r.Ok {
		t.Errorf("expected Ok=true since 17.0.9 satisfies >=11.0.0, got %+v", r)
	}
}

func TestAllOk(t *testing.T) {
	p := display.CheckResult{Name: "go", Ok: true}
	tools := []display.CheckResult{{Name: "gh", Ok: true}}
	if !display.AllOk(p, tools) {
		t.Error("expected AllOk = true")
	}
	p.Ok = false
	if display.AllOk(p, tools) {
		t.Error("expected AllOk = false when provider fails")
	}
}
