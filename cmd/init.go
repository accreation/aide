package cmd

import (
	"fmt"
	"os"

	"aide/internal/config"
	"aide/internal/project"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var initProvider string
var initName string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create aide.yaml in the current directory",
	Long:  "Generates a default aide.yaml with the specified provider. Use --name to register the project for 'aide start'.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initProvider, "provider", "p", "claude", "AI provider name (claude, copilot, codex, opencode)")
	initCmd.Flags().StringVarP(&initName, "name", "n", "", "Register project with a name for 'aide start'")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	path := "aide.yaml"
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}

	cfg := config.GenerateDefault(initProvider)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Printf("Created %s with provider: %s\n", path, initProvider)
	fmt.Println("Add your tools to the 'tools' list, then run 'aide check'.")

	if initName != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		if err := project.Register(initName, cwd); err != nil {
			return fmt.Errorf("registering project: %w", err)
		}
		fmt.Printf("Registered project %q — use 'aide start %s' to jump here from anywhere.\n", initName, initName)
	}

	return nil
}
