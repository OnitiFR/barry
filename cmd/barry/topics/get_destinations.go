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

var getDestinationsFlagBasic bool

// getDestinationsCmd represents the "get destinations" command
var getDestinationsCmd = &cobra.Command{
	Use:   "destinations",
	Short: "List push destinations",
	// Long: ``,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		getDestinationsFlagBasic, _ = cmd.Flags().GetBool("basic")
		if getDestinationsFlagBasic {
			client.GetExitMessage().Disable()
		}

		call := client.GlobalAPI.NewCall("GET", "/destination", map[string]string{})
		call.JSONCallback = getDestinationCB
		call.Do()
	},
}

func getDestinationCB(reader io.Reader, headers http.Header) {
	var data common.APIDestinationEntries
	dec := json.NewDecoder(reader)
	err := dec.Decode(&data)
	if err != nil {
		log.Fatal(err.Error())
	}

	if getDestinationsFlagBasic {
		for _, line := range data {
			fmt.Println(line.Name)
		}
	} else {
		if len(data) == 0 {
			fmt.Printf("No push destination configured.\n")
			return
		}

		strData := [][]string{}
		for _, line := range data {
			strData = append(strData, []string{
				line.Name,
				line.Type,
			})
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Type"})
		table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
		table.SetCenterSeparator("|")
		table.AppendBulk(strData)
		table.Render()
	}
}

func init() {
	getCmd.AddCommand(getDestinationsCmd)
	getDestinationsCmd.Flags().BoolP("basic", "b", false, "show basic list, without any formating")
}
