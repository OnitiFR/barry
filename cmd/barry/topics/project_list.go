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
	"time"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/OnitiFR/barry/common"
	"github.com/c2h5oh/datasize"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var projectListFlagBasic bool
var projectListFlagSize bool

// projectListCmd represents the "project list" command
var projectListCmd = &cobra.Command{
	Use:   "list [project]",
	Short: "List all projects or all project's files",
	// Long: ``,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectListFlagBasic, _ = cmd.Flags().GetBool("basic")
		projectListFlagSize, _ = cmd.Flags().GetBool("size")
		if projectListFlagBasic {
			client.GetExitMessage().Disable()
		}

		if len(args) > 0 {
			projectName := args[0]
			call := client.GlobalAPI.NewCall("GET", "/project/files", map[string]string{
				"project": projectName,
			})
			call.JSONCallback = projectFileListCB
			call.Do()
		} else {
			call := client.GlobalAPI.NewCall("GET", "/project", map[string]string{})
			call.JSONCallback = projectListCB
			call.Do()
		}
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

		grey := color.New(color.FgHiBlack).SprintFunc()
		strData := [][]string{}
		for _, line := range data {
			name := line.Path
			if line.Archived {
				name = grey(name)
			}
			strData = append(strData, []string{
				name,
				strconv.Itoa(line.FileCountCurrent),
				datasize.ByteSize(line.SizeCountCurrent).HR(),
				fmt.Sprintf("%.2f", line.CostCurrent),
			})
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Path", "Files", "Size", "Cost"})
		table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
		table.SetCenterSeparator("|")
		table.AppendBulk(strData)
		table.Render()
	}
}

func projectFileListCB(reader io.Reader, headers http.Header) {
	var data common.APIFileListEntries
	dec := json.NewDecoder(reader)
	err := dec.Decode(&data)
	if err != nil {
		log.Fatal(err.Error())
	}

	if projectListFlagBasic {
		for _, line := range data {
			fmt.Println(line.Filename)
		}
	} else {
		strData := [][]string{}
		yellow := color.New(color.FgHiYellow).SprintFunc()
		red := color.New(color.FgHiRed).SprintFunc()

		for _, line := range data {
			var name string
			if line.Encrypted {
				name = "🔒"
			} else {
				name = "  "
			}
			name += line.Filename
			maxExpire := line.ExpireRemote
			container := "(local)"
			if line.ExpireLocal.After(line.ExpireRemote) {
				maxExpire = line.ExpireLocal
			}
			expire := maxExpire.Format("2006-01-02 15:04")
			if time.Until(maxExpire) < time.Hour*24 {
				expire = red(expire) // will expire in less than one week
			} else if time.Until(maxExpire) < time.Hour*24*7 {
				expire = yellow(expire)
			}
			if line.ExpiredLocal {
				container = yellow(line.Container)
			}
			if line.Retrieved {
				container = "(retrieved)"
			}
			strData = append(strData, []string{
				name,
				line.ModTime.Format("2006-01-02 15:04"),
				datasize.ByteSize(line.Size).HR(),
				expire,
				container,
			})
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "mtime", "Size", "Expire", "Container"})
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
