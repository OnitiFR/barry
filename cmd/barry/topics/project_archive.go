package topics

import (
	"net/url"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/spf13/cobra"
)

// projectArchiveCmd represents the "project archive" command
var projectArchiveCmd = &cobra.Command{
	Use:   "archive [project]",
	Short: "Archive a project",
	Long: `An archived project will no more fire 'no backup' alerts. Any
new backup will unarchive the project.
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := url.PathEscape(args[0])
		call := client.GlobalAPI.NewCall("POST", "/project", map[string]string{
			"project": projectName,
			"action":  "archive",
		})
		call.Do()
	},
}

func init() {
	projectCmd.AddCommand(projectArchiveCmd)
}
