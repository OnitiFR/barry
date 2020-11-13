package topics

import (
	"github.com/spf13/cobra"
)

// fileCmd represents the file command
var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "Files management",
}

func init() {
	rootCmd.AddCommand(fileCmd)
}
