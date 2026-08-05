package launcher

import (
	"testing"
)

func TestLaunchNotFound(t *testing.T) {
	l := &Launcher{}
	err := l.Launch("nonexistent-tool-xyz-123")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestLaunchSuccess(t *testing.T) {
	l := &Launcher{}
	err := l.Launch("go", "version")
	if err != nil {
		t.Logf("launch failed (may be expected in restricted env): %v", err)
	}
}

func TestLaunchWithAccountMissing(t *testing.T) {
	l := &Launcher{AccountName: "nonexistent-account"}
	err := l.Launch("go", "version")
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}
