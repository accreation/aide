package checker

import (
	"testing"

	"aide/internal/config"
	"aide/internal/display"
)

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
