package topics

import (
	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/spf13/cobra"
)

// projectSetCmd represents the "project set" command
var projectSetCmd = &cobra.Command{
	Use:   "set <setting> <value> <project>",
	Short: "Set settings of a project",
	Long: `Set a specific setting on a project.

Supported settings:
 - backup_every
 - local_expiration
 - remote_expiration
`,

	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		setting := args[0]
		value := args[1]
		projectName := args[2]
		call := client.GlobalAPI.NewCall("POST", "/project/setting", map[string]string{
			"setting": setting,
			"value":   value,
			"project": projectName,
		})
		call.Do()
	},
}

func init() {
	projectCmd.AddCommand(projectSetCmd)
}
