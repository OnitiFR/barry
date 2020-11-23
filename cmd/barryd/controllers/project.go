package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/OnitiFR/barry/cmd/barryd/server"
	"github.com/OnitiFR/barry/common"
)

func getEntryFromRequest(req *server.Request) (*server.Project, error) {
	projectName, err := url.PathUnescape(req.SubPath)
	if err != nil {
		return nil, err
	}

	project, err := req.App.ProjectDB.GetByName(projectName)
	if err != nil {
		return nil, err
	}
	return project, nil
}

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
			Archived:         project.Archived,
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
	project, err := getEntryFromRequest(req)
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
			Container:     file.Container,
		})
	}

	enc := json.NewEncoder(req.Response)
	err = enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}
}

// ActionProjectController manage all "actions" on a project
func ActionProjectController(req *server.Request) {
	project, err := getEntryFromRequest(req)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 404)
		return
	}

	action := req.HTTP.FormValue("action")
	switch action {
	case "archive":
		projectControllerActionArchive(project, req)
	case "unarchive":
		projectControllerActionUnarchive(project, req)
	default:
		msg := fmt.Sprintf("unknown action '%s'", action)
		req.App.Log.Error(project.Path, msg)
		http.Error(req.Response, msg, 400)
	}
}

func projectControllerActionArchive(project *server.Project, req *server.Request) {
	if project.Archived {
		msg := fmt.Sprintf("project '%s' is already archived", project.Path)
		req.App.Log.Error(project.Path, msg)
		http.Error(req.Response, msg, 400)
		return
	}

	project.Archived = true
	err := req.App.ProjectDB.Save()
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}

	req.Printf("project '%s' is now archived\n", project.Path)
}

func projectControllerActionUnarchive(project *server.Project, req *server.Request) {
	if !project.Archived {
		msg := fmt.Sprintf("project '%s' is not archived", project.Path)
		req.App.Log.Error(project.Path, msg)
		http.Error(req.Response, msg, 400)
		return
	}

	project.Archived = false
	err := req.App.ProjectDB.Save()
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}

	req.Printf("project '%s' is now unarchived\n", project.Path)
}
