package topics

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/OnitiFR/barry/common"
	"github.com/briandowns/spinner"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var fileDownloadVars struct {
	loop           bool
	path           string
	filename       string
	spinner        *spinner.Spinner
	previousStatus string
}

// fileDownloadCmd represents the file download command
var fileDownloadCmd = &cobra.Command{
	Use:   "download <project> <file>",
	Short: "Download a file in the current directory",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fileDownloadVars.filename = args[1]
		fileDownloadVars.path = args[0] + "/" + args[1]
		call := client.GlobalAPI.NewCall("GET", "/file/status", map[string]string{
			"file": fileDownloadVars.path,
		})
		call.JSONCallback = fileDownloadStatusCB

		// first request can be slow, let's inform the user we understood what he want
		fmt.Printf("requesting %s…\n", fileDownloadVars.filename)

		fileDownloadVars.loop = true
		for {
			call.Do()
			if !fileDownloadVars.loop {
				break // exit the whole command
			}
			time.Sleep(3 * time.Second)
		}
	},
}

func newSpinner(subject string) {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return
	}

	if fileDownloadVars.spinner != nil {
		fileDownloadVars.spinner.Stop()
		fmt.Println()
	}

	s := spinner.New(spinner.CharSets[37], 200*time.Millisecond)
	s.FinalMSG = fmt.Sprintf("%s: completed", subject)
	s.Start()
	fileDownloadVars.spinner = s
}

func fileDownloadStatusCB(reader io.Reader, headers http.Header) {
	var data common.APIFileStatus
	dec := json.NewDecoder(reader)
	err := dec.Decode(&data)
	if err != nil {
		log.Fatal(err.Error())
	}

	if data.Status == common.APIFileStatusAvailable {
		fileDownloadDo()
		fileDownloadVars.loop = false
		return
	}

	if fileDownloadVars.previousStatus != data.Status {
		newSpinner(data.Status)
		fileDownloadVars.previousStatus = data.Status
	}
	end := time.Now().Add(data.ETA).Format("2006-01-02 15:04")
	msg := fmt.Sprintf("%s: %s (%s)", data.Status, data.ETA, end)
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fileDownloadVars.spinner.Suffix = " " + msg
	} else {
		fmt.Println(msg)
	}
}

func fileDownloadDo() {
	if fileDownloadVars.spinner != nil {
		fileDownloadVars.spinner.Stop()
		fmt.Println()
	}

	call := client.GlobalAPI.NewCall("GET", "/file/download", map[string]string{
		"file": fileDownloadVars.path,
	})
	call.DestFilePath = filepath.Base(fileDownloadVars.filename)
	call.Do()
}

func init() {
	fileCmd.AddCommand(fileDownloadCmd)
}
