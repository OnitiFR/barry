package topics

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/OnitiFR/barry/common"
	"github.com/spf13/cobra"
)

// statusCmd represents the "status" command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get informations about server",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		call := client.GlobalAPI.NewCall("GET", "/status", map[string]string{})
		call.JSONCallback = statusDisplay
		call.Do()
	},
}

func statusDisplay(reader io.Reader, headers http.Header) {
	var data common.APIStatus
	dec := json.NewDecoder(reader)
	err := dec.Decode(&data)
	if err != nil {
		log.Fatal(err.Error())
	}
	v := reflect.ValueOf(data)
	typeOfT := v.Type()
	for i := 0; i < v.NumField(); i++ {
		key := typeOfT.Field(i).Name
		format, _ := typeOfT.Field(i).Tag.Lookup("format")
		if format == "ignore" {
			continue
		}
		val := common.InterfaceValueToString(v.Field(i).Interface(), format)
		fmt.Printf("%s: %s\n", key, val)
	}
	fmt.Println("Workers:")
	for id, status := range data.Workers {
		fmt.Printf("  %d: %s\n", id, status)
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
