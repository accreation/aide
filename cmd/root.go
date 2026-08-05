package cmd

import (
	"fmt"
	"os"

	"aion/internal/checker"
	"aion/internal/config"
	"aion/internal/display"

	"github.com/spf13/cobra"
)

// Version is set via ldflags at build time: -X aion/cmd.Version=1.0.0
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "aion",
	Version: Version,
	Short:   "AI environment manager — check and install tools for your AI provider",
	Long: `Aion reads aion.yaml and verifies that your AI provider and all required
tools are installed with correct versions.

Without subcommands, aion runs a check and exits with code 0 if everything is ok.`,
	RunE: runCheck,
}

func Execute() error {
	return rootCmd.Execute()
}

func runCheck(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	cfg, err := config.FindAndParse(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		fmt.Fprintln(os.Stderr, "Tip: run 'aion init' to create aion.yaml")
		os.Exit(1)
	}

	chk := checker.New(cfg)
	providerResult := chk.CheckProvider()
	toolResults := chk.CheckTools()

	fmt.Println("Aion — environment check")
	fmt.Println("───────────────────────")
	display.PrintProviderResult(os.Stdout, providerResult)
	display.PrintToolResults(os.Stdout, toolResults)

	if !display.AllOk(providerResult, toolResults) {
		fmt.Println("\nSome items are missing or outdated. Run 'aion install' to fix.")
		os.Exit(1)
	}

	fmt.Println("\nAll checks passed!")
	return nil
}
