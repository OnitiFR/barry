package topics

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/OnitiFR/barry/common"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// keyListCmd represents the "key list" command
var keyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List keys",
	// Long: ``,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		call := client.GlobalAPI.NewCall("GET", "/key", map[string]string{})
		call.JSONCallback = keyListCB
		call.Do()
	},
}

func keyListCB(reader io.Reader, headers http.Header) {
	var data common.APIKeyListEntries
	dec := json.NewDecoder(reader)
	err := dec.Decode(&data)
	if err != nil {
		log.Fatal(err.Error())
	}

	if len(data) == 0 {
		fmt.Printf("No result. But you've called the API. WTF.\n")
		return
	}

	strData := [][]string{}
	for _, line := range data {
		strData = append(strData, []string{
			line.Comment,
		})
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Comment"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(strData)
	table.Render()
}

func init() {
	keyCmd.AddCommand(keyListCmd)
}
