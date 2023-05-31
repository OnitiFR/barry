package topics

import (
	"log"
	"os"
	"time"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/spf13/cobra"
)

// fileUploadloadCmd represents the file upload command
var fileUploadloadCmd = &cobra.Command{
	Use:   "upload <project> <local-file>",
	Short: "Upload a file to the server",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		expire, _ := cmd.Flags().GetString("expire")
		expireDuration, err := client.ParseExpiration(expire)
		if err != nil {
			log.Fatalf("unable to parse expiration: %s", err)
		}

		infos, err := os.Stat(args[1])
		if err != nil {
			log.Fatal(err)
		}

		call := client.GlobalAPI.NewCall("POST", "/file/upload", map[string]string{
			"project":  args[0],
			"expire":   client.DurationAsSecondsString(expireDuration),
			"mod_time": infos.ModTime().Format(time.RFC3339),
		})

		err = call.AddFile("file", args[1])
		if err != nil {
			log.Fatal(err)
		}
		call.Do()
	},
}

func init() {
	fileCmd.AddCommand(fileUploadloadCmd)
	fileUploadloadCmd.Flags().StringP("expire", "e", "", "expiration delay (ex: 2h, 10d, 1y)")
}
