package display

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintProviderResultOk(t *testing.T) {
	var buf bytes.Buffer
	PrintProviderResult(&buf, CheckResult{Name: "claude", Ok: true})
	out := buf.String()
	if !strings.Contains(out, "claude") {
		t.Errorf("expected output to contain 'claude', got: %s", out)
	}
	// Should contain a checkmark character
	if !strings.Contains(out, "OK") || !strings.Contains(out, "provider") {
		t.Errorf("expected OK indication, got: %s", out)
	}
}

func TestPrintProviderResultFail(t *testing.T) {
	var buf bytes.Buffer
	PrintProviderResult(&buf, CheckResult{Name: "codex", Ok: false, Installed: ""})
	out := buf.String()
	if !strings.Contains(out, "FAIL") || !strings.Contains(out, "not found") {
		t.Errorf("expected FAIL / not found, got: %s", out)
	}
}

func TestPrintProviderResultFoundVersionUndetermined(t *testing.T) {
	var buf bytes.Buffer
	PrintProviderResult(&buf, CheckResult{Name: "java", Ok: false, Found: true})
	out := buf.String()
	if !strings.Contains(out, "FAIL") || !strings.Contains(out, "java") {
		t.Errorf("expected FAIL / java, got: %s", out)
	}
	if strings.Contains(out, "not found") {
		t.Errorf("binary was found on PATH, message should not say 'not found', got: %s", out)
	}
}

func TestPrintToolResultsFoundVersionUndetermined(t *testing.T) {
	var buf bytes.Buffer
	results := []CheckResult{
		{Name: "java", Required: ">=11.0.0", Ok: false, Found: true},
	}
	PrintToolResults(&buf, results)
	out := buf.String()
	if !strings.Contains(out, "java") || strings.Contains(out, "not found") {
		t.Errorf("expected java found-but-undetermined message, not 'not found', got: %s", out)
	}
}

func TestPrintAccountResultNoneConfiguredPrintsNothing(t *testing.T) {
	var buf bytes.Buffer
	PrintAccountResult(&buf, CheckResult{Ok: true})
	if buf.Len() != 0 {
		t.Errorf("expected nothing printed for an accountless project, got: %s", buf.String())
	}
}

func TestPrintAccountResultOk(t *testing.T) {
	var buf bytes.Buffer
	PrintAccountResult(&buf, CheckResult{Name: "acme", Ok: true, Installed: "anar@acme.example"})
	out := buf.String()
	if !strings.Contains(out, "OK") || !strings.Contains(out, "acme") || !strings.Contains(out, "anar@acme.example") {
		t.Errorf("expected OK/acme/email, got: %s", out)
	}
}

func TestPrintAccountResultFail(t *testing.T) {
	var buf bytes.Buffer
	PrintAccountResult(&buf, CheckResult{Name: "acme", Ok: false, Installed: "not logged in"})
	out := buf.String()
	if !strings.Contains(out, "FAIL") || !strings.Contains(out, "acme") || !strings.Contains(out, "not logged in") {
		t.Errorf("expected FAIL/acme/not logged in, got: %s", out)
	}
}

func TestPrintToolResults(t *testing.T) {
	var buf bytes.Buffer
	results := []CheckResult{
		{Name: "gh", Required: ">=2.0.0", Installed: "2.65.0", Ok: true},
		{Name: "glab", Ok: false},
		{Name: "az", Required: ">=2.60.0", Installed: "2.50.0", Ok: false},
	}
	PrintToolResults(&buf, results)
	out := buf.String()
	if !strings.Contains(out, "gh") || !strings.Contains(out, "2.65.0") {
		t.Errorf("expected gh info, got: %s", out)
	}
	if !strings.Contains(out, "glab") {
		t.Errorf("expected glab, got: %s", out)
	}
}
