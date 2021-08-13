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

var filePushVars struct {
	loop           bool
	path           string
	filename       string
	destination    string
	spinner        *spinner.Spinner
	previousStatus string
}

// filePushCmd represents the file pusj command
var filePushCmd = &cobra.Command{
	Use:   "push <project> <file> <destination>",
	Short: "Push a file directly to a remote destination",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		filePushVars.filename = args[1]
		filePushVars.path = args[0] + "/" + args[1]
		filePushVars.destination = args[2]
		// TODO: check destination BEFORE anything else :)
		call := client.GlobalAPI.NewCall("GET", "/file/status", map[string]string{
			"file": filePushVars.path,
		})
		call.JSONCallback = filePushStatusCB

		// first request can be slow, let's inform the user we understood what he want
		fmt.Printf("requesting %s…\n", filePushVars.filename)

		filePushVars.loop = true
		for {
			call.Do()
			if !filePushVars.loop {
				break // exit the whole command
			}
			time.Sleep(3 * time.Second)
		}
	},
}

func newPushSpinner(subject string) {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return
	}

	if filePushVars.spinner != nil {
		filePushVars.spinner.Stop()
		fmt.Println()
	}

	s := spinner.New(spinner.CharSets[37], 200*time.Millisecond)
	s.FinalMSG = fmt.Sprintf("%s: completed", subject)
	s.Start()
	filePushVars.spinner = s
}

func filePushStatusCB(reader io.Reader, headers http.Header) {
	var data common.APIFileStatus
	dec := json.NewDecoder(reader)
	err := dec.Decode(&data)
	if err != nil {
		log.Fatal(err.Error())
	}

	if data.Status == common.APIFileStatusAvailable {
		filePushDo()
		filePushVars.loop = false
		return
	}

	if filePushVars.previousStatus != data.Status {
		newPushSpinner(data.Status)
		filePushVars.previousStatus = data.Status
	}
	end := time.Now().Add(data.ETA).Format("2006-01-02 15:04")
	msg := fmt.Sprintf("%s: %s (%s)", data.Status, data.ETA, end)
	if isatty.IsTerminal(os.Stdout.Fd()) {
		filePushVars.spinner.Suffix = " " + msg
	} else {
		fmt.Println(msg)
	}
}

func filePushDo() {
	if filePushVars.spinner != nil {
		filePushVars.spinner.Stop()
		fmt.Println()
	}

	// TODO: use a get to allow polling of the upload to destination? (so we get an ETA)
	call := client.GlobalAPI.NewCall("POST", "/file/push", map[string]string{
		"file":        filePushVars.path,
		"destination": filePushVars.destination,
	})
	call.DestFilePath = filepath.Base(filePushVars.filename)
	call.Do()
}

func init() {
	fileCmd.AddCommand(filePushCmd)
}
