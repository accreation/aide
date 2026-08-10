package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aide/internal/config"
	"aide/internal/installer"
	"aide/internal/semver"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var addVersion string

var addCmd = &cobra.Command{
	Use:   "add <tool-name>",
	Short: "Add a tool to aide.yaml",
	Long: `Adds a tool to the aide.yaml config file.

If --version is provided, it is used directly.
If not, aion detects the currently installed version and pins it with >= constraint.
If the tool is not installed, it is added without a version constraint.

Examples:
  aide add glab
  aide add gh --version ">=2.0.0"`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().StringVarP(&addVersion, "version", "v", "", "Version constraint (e.g. \">=2.0.0\" or \"^1.0.0\")")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	toolName := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Find aide.yaml
	cfgPath, err := config.FindPath(cwd)
	if err != nil {
		return fmt.Errorf("aide.yaml not found — run 'aide init' first: %w", err)
	}

	// Read current config
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	cfg, err := config.FindAndParse(cwd)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	projectDir := filepath.Dir(cfgPath)

	// Determine version
	version := addVersion
	if version == "" {
		version = detectVersion(cfg, projectDir, toolName)
	}

	// Check if tool already exists — update or append
	found := false
	for i, t := range cfg.Tools {
		if t.Name == toolName {
			if t.Version == version {
				fmt.Printf("Tool %q is already in aide.yaml with the same version (%s)\n", toolName, version)
				return nil
			}
			cfg.Tools[i].Version = version
			found = true
			fmt.Printf("Updated %q version to %s\n", toolName, version)
			break
		}
	}
	if !found {
		cfg.Tools = append(cfg.Tools, config.Tool{
			Name:    toolName,
			Version: version,
		})
		if version != "" {
			fmt.Printf("Added %q with version %s\n", toolName, version)
		} else {
			fmt.Printf("Added %q (no version constraint — tool not found in PATH)\n", toolName)
		}
	}

	// Write back
	newData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(cfgPath, newData, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	fmt.Printf("Updated %s\n", cfgPath)
	_ = data // suppress unused warning
	return nil
}

// detectVersion tries to detect the installed version of a tool
// by running it with --version or version subcommand.
// Returns version constraint (e.g. ">=1.2.3") or empty string if not found.
func detectVersion(cfg *config.Config, projectDir, name string) string {
	if cfg.IsIsolated() {
		// Prepend .aide/shims to PATH so isolated-mode installs are found,
		// matching checker.checkBinary's resolution order.
		origPath := os.Getenv("PATH")
		os.Setenv("PATH", installer.ShimDir(projectDir)+string(os.PathListSeparator)+origPath)
		defer os.Setenv("PATH", origPath)
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	_ = path

	// Try --version first, then version subcommand
	for _, flag := range []string{"--version", "version"} {
		cmd := exec.Command(name, flag)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		v, err := semver.ExtractVersion(strings.TrimSpace(string(out)))
		if err != nil {
			continue
		}
		return ">=" + v
	}
	return ""
}
