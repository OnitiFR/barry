package topics

// TODO: update github.com/briandowns/spinner since have an issue with FinalMSG
// https://github.com/briandowns/spinner/issues/123

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

// filePushCmd represents the file push command
var filePushCmd = &cobra.Command{
	Use:   "push <project> <file> <destination>",
	Short: "Push a file directly to a remote destination",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		filePushVars.filename = args[1]
		filePushVars.path = args[0] + "/" + args[1]
		filePushVars.destination = args[2]
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
		pushDo()
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

func pushDo() {
	call := client.GlobalAPI.NewCall("GET", "/file/push/status", map[string]string{
		"file":        filePushVars.path,
		"destination": filePushVars.destination,
	})
	call.JSONCallback = pushStatusCB
	filePushVars.loop = true
	for {
		call.Do()
		if !filePushVars.loop {
			break // exit the whole command
		}
		time.Sleep(3 * time.Second)
	}
}

func pushStatusCB(reader io.Reader, headers http.Header) {
	var data common.APIPushStatus
	dec := json.NewDecoder(reader)
	err := dec.Decode(&data)
	if err != nil {
		log.Fatal(err.Error())
	}

	if data.Status != common.APIPushStatusPushing {
		filePushVars.loop = false
		if filePushVars.spinner != nil {
			filePushVars.spinner.Stop()
			fmt.Println()
		}
		switch data.Status {
		case common.APIPushStatusError:
			log.Fatal(data.Error)
		case common.APIPushStatusSuccess:
			fmt.Printf("File successfully pushed to '%s'\n", filePushVars.destination)
		default:
			log.Fatalf("unknown status '%s'", data.Status)
		}
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

func init() {
	fileCmd.AddCommand(filePushCmd)
}
