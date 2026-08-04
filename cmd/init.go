package cmd

import (
	"fmt"
	"os"

	"aion/internal/config"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var initProvider string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create aion.yaml in the current directory",
	Long:  "Generates a default aion.yaml with the specified provider.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initProvider, "provider", "p", "claude", "AI provider name (claude, copilot, codex, opencode)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	path := "aion.yaml"
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
	fmt.Println("Add your tools to the 'tools' list, then run 'aion check'.")
	return nil
}
