package topics

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/OnitiFR/barry/common"
	"github.com/spf13/cobra"
)

// projectInfosCmd represents the "project infos" command
var projectInfosCmd = &cobra.Command{
	Use:   "infos [project]",
	Short: "Get informations about project",

	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := url.PathEscape(args[0])
		call := client.GlobalAPI.NewCall("GET", "/project/infos", map[string]string{
			"project": projectName,
		})
		call.JSONCallback = projectInfosDisplay
		call.Do()
	},
}

func projectInfosDisplay(reader io.Reader, headers http.Header) {
	var data common.APIProjectInfos
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
		val := common.InterfaceValueToString(v.Field(i).Interface(), format)
		fmt.Printf("%s: %s\n", key, val)
	}
}

func init() {
	projectCmd.AddCommand(projectInfosCmd)
}
