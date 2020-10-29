package topics

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/OnitiFR/barry/common"
	"github.com/c2h5oh/datasize"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var projectListFlagBasic bool
var projectListFlagSize bool

// projectListCmd represents the "project list" command
var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	// Long: ``,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		projectListFlagBasic, _ = cmd.Flags().GetBool("basic")
		projectListFlagSize, _ = cmd.Flags().GetBool("size")
		if projectListFlagBasic == true {
			client.GetExitMessage().Disable()
		}

		call := client.GlobalAPI.NewCall("GET", "/project", map[string]string{})
		call.JSONCallback = projectListCB
		call.Do()
	},
}

func projectListCB(reader io.Reader, headers http.Header) {
	var data common.APIProjectListEntries
	dec := json.NewDecoder(reader)
	err := dec.Decode(&data)
	if err != nil {
		log.Fatal(err.Error())
	}

	// default (controller) sort is by name
	if projectListFlagSize {
		sort.Slice(data, func(i, j int) bool {
			return data[i].SizeCountCurrent < data[j].SizeCountCurrent
		})
	}

	if projectListFlagBasic {
		for _, line := range data {
			fmt.Println(line.Path)
		}
	} else {
		if len(data) == 0 {
			fmt.Printf("Currently, no projects exists.\n")
			return
		}

		strData := [][]string{}
		for _, line := range data {
			strData = append(strData, []string{
				line.Path,
				strconv.Itoa(line.FileCountCurrent),
				datasize.ByteSize(line.SizeCountCurrent).HR(),
			})
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Path", "Files", "Size"})
		table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
		table.SetCenterSeparator("|")
		table.AppendBulk(strData)
		table.Render()
	}
}

func init() {
	projectCmd.AddCommand(projectListCmd)
	projectListCmd.Flags().BoolP("basic", "b", false, "show basic list, without any formating")
	projectListCmd.Flags().BoolP("size", "s", false, "sort by size (ascending)")
}
