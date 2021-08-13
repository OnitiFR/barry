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
func getLocalPath(fullPath string, app *server.App) (string, error) {
	projectName := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)

	file := app.ProjectDB.FindFile(projectName, fileName)
	if file == nil {
		return "", fmt.Errorf("error with file '%s' in project '%s'", fileName, projectName)
	}

	availability, err := app.MakeFileAvailable(file)
	if err != nil {
		return "", err
	}

	if availability.Status != common.APIFileStatusAvailable {
		return "", fmt.Errorf("file '%s' in project '%s' is not available", fileName, projectName)
	}

	path := ""
	if !file.ExpiredLocal {
		path, _ = app.LocalStoragePath(server.FileStorageName, file.Path)
	} else if file.RetrievedPath != "" {
		path = file.RetrievedPath
	}

	if path == "" {
		return "", fmt.Errorf("can't file local path for file '%s' in project '%s'", fileName, projectName)
	}

	return path, nil
}

// FileDownloadController return the file stream
func FileDownloadController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/octet-stream")

	fullPath := req.HTTP.FormValue("file")

	projectName := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)

	path, err := getLocalPath(fullPath, req.App)
	if err != nil {
		req.App.Log.Error(projectName, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}

	req.App.Log.Infof(projectName, "file '%s' (%s) is downloaded by key '%s'", fileName, projectName, req.APIKey.Comment)

	http.ServeFile(req.Response, req.HTTP, path)
}

// FilePushController will push the (available) file to a remote destination
func FilePushController(req *server.Request) {
	fullPath := req.HTTP.FormValue("file")
	destination := req.HTTP.FormValue("destination")

	// lookup the destination

	projectName := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)

	path, err := getLocalPath(fullPath, req.App)
	if err != nil {
		req.App.Log.Error(projectName, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}

	fmt.Println(path)
	req.App.Log.Infof(projectName, "file '%s' (%s) is pushed to '%s' by key '%s'", fileName, projectName, destination, req.APIKey.Comment)
}
