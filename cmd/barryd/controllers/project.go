package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/OnitiFR/barry/cmd/barryd/server"
	"github.com/OnitiFR/barry/common"
)

// ListProjectsController list projects
func ListProjectsController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")

	var retData common.APIProjectListEntries

	names := req.App.ProjectDB.GetNames()
	for _, name := range names {

		project, err := req.App.ProjectDB.GetByName(name)
		if err != nil {
			msg := fmt.Sprintf("project %s: %s", name, err)
			req.App.Log.Error(server.MsgGlob, msg)
			http.Error(req.Response, msg, 500)
			return
		}

		var totalSize int64
		var totalCost float64
		for _, file := range project.Files {
			totalSize += file.Size
			totalCost += file.Cost
		}

		retData = append(retData, common.APIProjectListEntry{
			Path:             project.Path,
			FileCountCurrent: len(project.Files),
			SizeCountCurrent: totalSize,
			CostCurrent:      totalCost,
		})
	}

	enc := json.NewEncoder(req.Response)
	err := enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}
}
