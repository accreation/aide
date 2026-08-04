package semver

import (
	"fmt"
	"regexp"

	"github.com/Masterminds/semver/v3"
)

var semverRe = regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)

// ExtractVersion extracts the first semver-like version from a tool's --version output.
// Returns empty string and error if no version found.
func ExtractVersion(output string) (string, error) {
	match := semverRe.FindStringSubmatch(output)
	if match == nil {
		return "", fmt.Errorf("no semver version found in output")
	}
	return match[1], nil
}

// CheckConstraint checks if version satisfies the semver constraint.
// Empty constraint always returns true.
func CheckConstraint(version, constraint string) (bool, error) {
	if constraint == "" {
		return true, nil
	}
	v, err := semver.NewVersion(version)
	if err != nil {
		return false, fmt.Errorf("invalid version %q: %w", version, err)
	}
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false, fmt.Errorf("invalid constraint %q: %w", constraint, err)
	}
	return c.Check(v), nil
}
