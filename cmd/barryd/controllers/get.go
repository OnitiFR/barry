package controllers

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/OnitiFR/barry/cmd/barryd/server"
	"github.com/OnitiFR/barry/common"
)

// GetDestinationsController list push destinations
func GetDestinationsController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")

	var retData common.APIDestinationEntries
	for _, dest := range req.App.Config.Pushers {
		retData = append(retData, common.APIDestinationEntry{
			Name: dest.Name,
			Type: dest.Type,
		})
	}

	sort.Slice(retData, func(i, j int) bool {
		return retData[i].Name < retData[j].Name
	})

	enc := json.NewEncoder(req.Response)
	err := enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}
}
