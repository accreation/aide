package cmd

import (
	"fmt"
	"os"

	"aide/internal/project"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Jump to a registered project and launch aide",
	Long:  "Looks up a project registered with 'aide init --name', changes to its directory, and runs the full aide check + launch.",
	Args:  cobra.ExactArgs(1),
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	name := args[0]

	projectPath, err := project.Get(name)
	if err != nil {
		return err
	}

	if err := os.Chdir(projectPath); err != nil {
		return fmt.Errorf("changing to project directory %q: %w", projectPath, err)
	}

	// Run the standard aide check + launch flow (from root.go)
	return runCheck(cmd, args)
}
