package topics

import (
	"github.com/spf13/cobra"
)

// emergencyCmd represents the file command
var emergencyCmd = &cobra.Command{
	Use:   "emergency",
	Short: "Emergency operations on files",
}

func init() {
	rootCmd.AddCommand(emergencyCmd)
}
