package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

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
		msg := fmt.Sprintf("can't find file '%s' in project '%s'", fileName, projectName)
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
		return
	}
}

// getLocalPath will return the local path of an available file
func getLocalPath(fullPath string, app *server.App) (string, *server.File, error) {
	projectName := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)

	file := app.ProjectDB.FindFile(projectName, fileName)
	if file == nil {
		return "", nil, fmt.Errorf("can't file '%s' in project '%s'", fileName, projectName)
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
		return
	}

	req.App.Log.Infof(projectName, "file '%s' (%s) is downloaded by key '%s'", fileName, projectName, req.APIKey.Comment)

	http.ServeFile(req.Response, req.HTTP, path)
}

// FileUploadController will upload a file to the server
func FileUploadController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "text/plain")

	projectName := req.HTTP.FormValue("project")

	project, err := req.App.ProjectDB.GetByName(projectName)
	if err != nil {
		msg := fmt.Sprintf("can't find project '%s': %s", projectName, err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}

	formFile, header, err := req.HTTP.FormFile("file")
	if err != nil {
		msg := fmt.Sprintf("can't get file from form: %s", err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}
	defer formFile.Close()

	expireStr := req.HTTP.FormValue("expire")
	expire := time.Duration(0)
	if expireStr != "" {
		seconds, err := strconv.Atoi(expireStr)
		if err != nil {
			msg := fmt.Sprintf("can't convert expire value to int: %s", err.Error())
			req.App.Log.Error(projectName, msg)
			http.Error(req.Response, msg, 500)
			return
		}
		expire = time.Duration(seconds) * time.Second
	}

	modTimeStr := req.HTTP.FormValue("mod_time")
	modTime := time.Now()
	if modTimeStr != "" {
		modTime, err = time.Parse(time.RFC3339, modTimeStr)
		if err != nil {
			msg := fmt.Sprintf("can't parse mod_time value: %s", err.Error())
			req.App.Log.Error(projectName, msg)
			http.Error(req.Response, msg, 500)
			return
		}
	}

	// expireDate := time.Now().Add(expire)

	out, err := os.CreateTemp("", "barry-upload")
	if err != nil {
		msg := fmt.Sprintf("can't create temp file: %s", err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}
	defer os.Remove(out.Name())
	defer out.Close()

	_, err = io.Copy(out, formFile)
	if err != nil {
		msg := fmt.Sprintf("can't copy file: %s", err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}

	file := &server.File{
		Filename: header.Filename,
		Path:     out.Name(),
		ModTime:  modTime,
		Size:     header.Size,
		AddedAt:  time.Now(),
		Status:   server.FileStatusNew,
	}

	localExpiration, remoteExpiration, err := req.App.ProjectDB.GetProjectNextExpiration(project, file.ModTime)
	if err != nil {
		msg := fmt.Sprintf("can't get expiration: %s", err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}

	file.ExpireLocal = file.ModTime.Add(localExpiration.Keep)
	file.ExpireLocalOrg = localExpiration.Original
	if expire == 0 {
		file.ExpireRemote = file.ModTime.Add(remoteExpiration.Keep)
		file.ExpireRemoteOrg = remoteExpiration.Original
		file.RemoteKeep = remoteExpiration.Keep
	} else {
		file.ExpireRemote = time.Now().Add(expire)
		file.ExpireRemoteOrg = expireStr
		file.RemoteKeep = expire
	}

	err = req.App.UploadAndStore(projectName, file)
	if err != nil {
		msg := fmt.Sprintf("can't store file: %s", err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}

	// find (or create ?) project (GetByName / FindOrCreateProject)
	// build a File (wait_list.go:127)
	// expiration: see queueFile() in app_funcs.go
	// move file to the right place (UploadAndStore)
	// TODO: check that the file is added to the project

}

// FilePushStatusController will start to push the (available) file to a remote
// destination (if not already started) and return status
func FilePushStatusController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")
	var retData common.APIPushStatus

	fullPath := req.HTTP.FormValue("file")
	destination := req.HTTP.FormValue("destination")
	projectName := filepath.Dir(fullPath)

	pusherConfig, exists := req.App.Config.Pushers[destination]
	if !exists {
		msg := fmt.Sprintf("push destination '%s' not found", destination)
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 404)
		return
	}

	path, file, err := getLocalPath(fullPath, req.App)
	if err != nil {
		req.App.Log.Error(projectName, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}

	expireStr := req.HTTP.FormValue("expire")
	expire := time.Duration(0)
	if expireStr != "" {
		seconds, err := strconv.Atoi(expireStr)
		if err != nil {
			msg := fmt.Sprintf("invalid expire value '%s'", expireStr)
			req.App.Log.Error(projectName, msg)
			http.Error(req.Response, msg, 500)
			return
		}
		expire = time.Duration(seconds) * time.Second
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
		req.App.Log.Infof(projectName, "file '%s' (%s) is pushed to '%s' by key '%s'", file.Filename, projectName, destination, req.APIKey.Comment)

		retData.Status = common.APIPushStatusPushing

		switch pusherConfig.Type {
		case server.PusherTypeMulch:
			_, err = server.NewPusherMulch(file, path, expire, pusherConfig, req.App.Log)
		default:
			err = fmt.Errorf("pusher type '%s' not implemented", pusherConfig.Type)
		}

		if err != nil {
			req.App.Log.Error(projectName, err.Error())
			http.Error(req.Response, err.Error(), 500)
			return
		}
	}

	enc := json.NewEncoder(req.Response)
	err = enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(projectName, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}
}
