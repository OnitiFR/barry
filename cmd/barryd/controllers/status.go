package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/OnitiFR/barry/cmd/barryd/server"
)

// GetStatusController return status informations about host
func GetStatusController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")

	retData, err := req.App.Status()
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}

	enc := json.NewEncoder(req.Response)
	err = enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}
}
