package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/OnitiFR/barry/cmd/barryd/server"
)

// ListProjectsController list projects
func ListProjectsController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")

	retData := "Hello world!"

	enc := json.NewEncoder(req.Response)
	err := enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}
}
