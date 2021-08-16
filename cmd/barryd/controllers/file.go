package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/OnitiFR/barry/cmd/barryd/server"
	"github.com/OnitiFR/barry/common"
)

// FileStatusController returns file status
func FileStatusController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")

	fullPath := req.HTTP.FormValue("file")

	projectName := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)

	file := req.App.ProjectDB.FindFile(projectName, fileName)
	if file == nil {
		msg := fmt.Sprintf("error with file '%s' in project '%s'", fileName, projectName)
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}

	retData, err := req.App.MakeFileAvailable(file)
	if err != nil {
		req.App.Log.Error(projectName, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}

	enc := json.NewEncoder(req.Response)
	err = enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(projectName, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}
}

// getLocalPath will return the local path of an available file
func getLocalPath(fullPath string, app *server.App) (string, *server.File, error) {
	projectName := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)

	file := app.ProjectDB.FindFile(projectName, fileName)
	if file == nil {
		return "", nil, fmt.Errorf("error with file '%s' in project '%s'", fileName, projectName)
	}

	availability, err := app.MakeFileAvailable(file)
	if err != nil {
		return "", file, err
	}

	if availability.Status != common.APIFileStatusAvailable {
		return "", file, fmt.Errorf("file '%s' in project '%s' is not available", fileName, projectName)
	}

	path := ""
	if !file.ExpiredLocal {
		path, _ = app.LocalStoragePath(server.FileStorageName, file.Path)
	} else if file.RetrievedPath != "" {
		path = file.RetrievedPath
	}

	if path == "" {
		return "", file, fmt.Errorf("can't file local path for file '%s' in project '%s'", fileName, projectName)
	}

	return path, file, nil
}

// FileDownloadController return the file stream
func FileDownloadController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/octet-stream")

	fullPath := req.HTTP.FormValue("file")

	projectName := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)

	path, _, err := getLocalPath(fullPath, req.App)
	if err != nil {
		req.App.Log.Error(projectName, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}

	req.App.Log.Infof(projectName, "file '%s' (%s) is downloaded by key '%s'", fileName, projectName, req.APIKey.Comment)

	http.ServeFile(req.Response, req.HTTP, path)
}

// FilePushStatusController will start to push the (available) file to a remote
// destination (if not already started) and return status
func FilePushStatusController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")
	var retData common.APIPushStatus

	fullPath := req.HTTP.FormValue("file")
	destination := req.HTTP.FormValue("destination")

	// TODO: lookup the destination!

	projectName := filepath.Dir(fullPath)
	// fileName := filepath.Base(fullPath)

	path, file, err := getLocalPath(fullPath, req.App)
	if err != nil {
		req.App.Log.Error(projectName, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}

	pusher := file.GetPusher(destination)
	if pusher != nil {
		finished := pusher.IsFinished()

		status := common.APIPushStatusPushing
		errMsg := ""

		if finished {
			status = common.APIPushStatusSuccess
			pErr := pusher.GetError()
			if pErr != nil {
				status = common.APIPushStatusError
				errMsg = pusher.GetError().Error()
			}
		}

		retData = common.APIPushStatus{
			Status: status,
			ETA:    pusher.GetETA(),
			Error:  errMsg,
		}
	} else {
		retData.Status = common.APIPushStatusPushing
		_, err = server.NewPusherMulch(file, path, &server.PusherConfig{
			Type: server.PusherTypeMulch,
			Name: "local",
			URL:  "http://127.0.0.1:8686",
			Key:  "5aFPOWVKw3ZGszIxPw8R5V8o0eWt9ywUsTPGLwUMPbAZKSThcWNp3vVcIY9Qi6ix",
		})
		if err != nil {
			req.App.Log.Error(projectName, err.Error())
			http.Error(req.Response, err.Error(), 500)
		}
	}

	// fmt.Println(path)
	// req.App.Log.Infof(projectName, "file '%s' (%s) is pushed to '%s' by key '%s'", fileName, projectName, destination, req.APIKey.Comment)

	enc := json.NewEncoder(req.Response)
	err = enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(projectName, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}
}
