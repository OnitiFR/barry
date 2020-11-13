package topics

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"time"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/OnitiFR/barry/common"
	"github.com/spf13/cobra"
)

var fileDownloadLoop = true
var fileDownloadPath string
var fileDownloadFilename string

// fileDownloadCmd represents the file download command
var fileDownloadCmd = &cobra.Command{
	Use:   "download <project> <file>",
	Short: "Download a file in the current directory",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fileDownloadFilename = args[1]
		fileDownloadPath = url.PathEscape(path.Clean(args[0] + "/" + url.PathEscape(args[1])))
		call := client.GlobalAPI.NewCall("GET", "/file/status/"+fileDownloadPath, map[string]string{})
		call.JSONCallback = fileDownloadStatusCB
		for {
			call.Do()
			if fileDownloadLoop == false {
				break // exit the whole command
			}
			time.Sleep(3 * time.Second)
		}
	},
}

func fileDownloadStatusCB(reader io.Reader, headers http.Header) {
	var data common.APIFileStatus
	dec := json.NewDecoder(reader)
	err := dec.Decode(&data)
	if err != nil {
		log.Fatal(err.Error())
	}
	switch data.Status {
	case common.APIFileStatusAvailable:
		fmt.Printf("file is available, downloading…\n")
		fileDownloadDo()
		fileDownloadLoop = false
	default:
		fmt.Printf("file status: %s, ETA %s\n", data.Status, data.ETA)
	}
}

func fileDownloadDo() {
	call := client.GlobalAPI.NewCall("GET", "/file/download/"+fileDownloadPath, map[string]string{})
	call.DestFilePath = filepath.Base(fileDownloadFilename)
	call.Do()
}

func init() {
	fileCmd.AddCommand(fileDownloadCmd)
}
