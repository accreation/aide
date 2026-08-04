package installer

import (
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
