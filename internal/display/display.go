package display

import (
	"fmt"
	"io"
)

type CheckResult struct {
	Name      string
	Required  string
	Installed string
	Ok        bool
	// Found indicates the binary was located on PATH, even if Ok is false
	// (e.g. a version constraint failed, or the version couldn't be determined).
	Found bool
}

// PrintProviderResult prints the provider check result.
func PrintProviderResult(w io.Writer, r CheckResult) {
	switch {
	case r.Ok:
		fmt.Fprintf(w, "  OK  provider: %s\n", r.Name)
	case r.Found:
		fmt.Fprintf(w, "  FAIL  provider: %s — found but could not determine version\n", r.Name)
	default:
		fmt.Fprintf(w, "  FAIL  provider: %s — not found\n", r.Name)
	}
}

// PrintAccountResult prints the account check result. A zero-value Name
// means no account is configured for this project, in which case nothing
// is printed — accountless projects must look bit-identical to before this
// check existed.
func PrintAccountResult(w io.Writer, r CheckResult) {
	if r.Name == "" {
		return
	}
	if r.Ok {
		fmt.Fprintf(w, "  OK  account: %s — %s\n", r.Name, r.Installed)
	} else {
		fmt.Fprintf(w, "  FAIL  account: %s — %s\n", r.Name, r.Installed)
	}
}

// PrintToolResults prints all tool check results.
func PrintToolResults(w io.Writer, tools []CheckResult) {
	for _, r := range tools {
		switch {
		case r.Ok && r.Required != "":
			fmt.Fprintf(w, "  OK  %s v%s (%s)\n", r.Name, r.Installed, r.Required)
		case r.Ok:
			if r.Installed != "" {
				fmt.Fprintf(w, "  OK  %s v%s\n", r.Name, r.Installed)
			} else {
				fmt.Fprintf(w, "  OK  %s\n", r.Name)
			}
		case r.Installed != "":
			fmt.Fprintf(w, "  FAIL  %s — version %s, required %s\n", r.Name, r.Installed, r.Required)
		case r.Found:
			fmt.Fprintf(w, "  FAIL  %s — found but could not determine version (required %s)\n", r.Name, r.Required)
		default:
			fmt.Fprintf(w, "  FAIL  %s — not found\n", r.Name)
		}
	}
}

// AllOk returns true if provider, account, and every tool result are Ok.
// account should be CheckResult{Ok: true} when no account is configured,
// so accountless projects are never gated by it.
func AllOk(provider, account CheckResult, tools []CheckResult) bool {
	if !provider.Ok || !account.Ok {
		return false
	}
	for _, t := range tools {
		if !t.Ok {
			return false
		}
	}
	return true
}
