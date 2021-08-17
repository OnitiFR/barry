package topics

import (
	"github.com/spf13/cobra"
)

// keyCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get informations from server",
}

func init() {
	rootCmd.AddCommand(getCmd)
}
