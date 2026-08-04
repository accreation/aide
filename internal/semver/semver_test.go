package semver

import "testing"

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		output   string
		expected string
	}{
		{"gh version 2.65.0 (2025-01-15)\nhttps://github.com/cli/cli/releases/tag/v2.65.0", "2.65.0"},
		{"git version 2.47.0", "2.47.0"},
		{"Azure CLI 2.50.0", "2.50.0"},
		{"v1.2.3", "1.2.3"},
		{"no version here", ""},
	}
	for _, tt := range tests {
		v, err := ExtractVersion(tt.output)
		if tt.expected == "" {
			if err == nil {
				t.Errorf("expected error for %q, got version %q", tt.output, v)
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for %q: %v", tt.output, err)
			continue
		}
		if v != tt.expected {
			t.Errorf("ExtractVersion(%q) = %q, want %q", tt.output, v, tt.expected)
		}
	}
}

func TestCheckConstraint(t *testing.T) {
	tests := []struct {
		version    string
		constraint string
		ok         bool
	}{
		{"2.65.0", ">=2.0.0", true},
		{"2.65.0", ">=3.0.0", false},
		{"1.0.0", "^1.0.0", true},
		{"2.0.0", "^1.0.0", false},
		{"2.65.0", "", true},
		{"1.0.0", "~1.0.0", true},
		{"1.1.0", "~1.0.0", false},
		{"3.2.1", ">=2.0.0, <4.0.0", true},
		{"4.0.0", ">=2.0.0, <4.0.0", false},
	}
	for _, tt := range tests {
		ok, err := CheckConstraint(tt.version, tt.constraint)
		if err != nil {
			t.Errorf("CheckConstraint(%q, %q): unexpected error: %v", tt.version, tt.constraint, err)
			continue
		}
		if ok != tt.ok {
			t.Errorf("CheckConstraint(%q, %q) = %v, want %v", tt.version, tt.constraint, ok, tt.ok)
		}
	}
}
