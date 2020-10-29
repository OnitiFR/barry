package topics

import (
	"github.com/spf13/cobra"
)

// projectCmd represents the project command
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Projects management",
}

func init() {
	rootCmd.AddCommand(projectCmd)
}
