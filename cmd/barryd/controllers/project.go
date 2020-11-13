package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

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

// ListProjectController list all files in a project
func ListProjectController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")

	var retData common.APIFileListEntries
	projectName, err := url.PathUnescape(req.SubPath)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 404)
		return
	}

	project, err := req.App.ProjectDB.GetByName(projectName)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 404)
		return
	}

	fileNames, err := req.App.ProjectDB.GetFilenames(project.Path)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}

	for _, name := range fileNames {
		file := req.App.ProjectDB.FindFile(project.Path, name)
		if file == nil {
			msg := fmt.Sprintf("error with file '%s'", name)
			req.App.Log.Error(server.MsgGlob, msg)
			http.Error(req.Response, msg, 500)
			return
		}

		retData = append(retData, common.APIFileListEntry{
			Filename:      file.Filename,
			ModTime:       file.ModTime,
			Size:          file.Size,
			ExpireLocal:   file.ExpireLocal,
			ExpireRemote:  file.ExpireRemote,
			RemoteKeep:    file.RemoteKeep,
			ExpiredLocal:  file.ExpiredLocal,
			ExpiredRemote: file.ExpiredRemote,
		})
	}

	enc := json.NewEncoder(req.Response)
	err = enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}
}
