package topics

import (
	"net/url"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/spf13/cobra"
)

// projectUnarchiveCmd represents the "project unarchive" command
var projectUnarchiveCmd = &cobra.Command{
	Use:   "unarchive [project]",
	Short: "Unarchive a project",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := url.PathEscape(args[0])
		call := client.GlobalAPI.NewCall("POST", "/project/"+projectName, map[string]string{
			"action": "unarchive",
		})
		call.Do()
	},
}

func init() {
	projectCmd.AddCommand(projectUnarchiveCmd)
}