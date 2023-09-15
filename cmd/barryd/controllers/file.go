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

	path, err := file.GetLocalPath(app)
	if err != nil {
		return "", file, err
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
	req.Response.Header().Set("Content-Type", "application/x-ndtext")

	projectName := req.HTTP.FormValue("project")

	project, err := req.App.ProjectDB.GetByName(projectName)
	if err != nil {
		msg := fmt.Sprintf("can't find project '%s': %s", projectName, err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 404)
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
			http.Error(req.Response, msg, 400)
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
			http.Error(req.Response, msg, 400)
			return
		}
	}

	if req.App.ProjectDB.FileExists(projectName, header.Filename) {
		msg := fmt.Sprintf("file '%s' already exists in project '%s'", header.Filename, projectName)
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, http.StatusForbidden)
		return
	}

	if req.App.WaitList.FileExists(projectName, header.Filename) {
		msg := fmt.Sprintf("file '%s/%s' already exists in wait list", projectName, header.Filename)
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, http.StatusForbidden)
		return
	}

	path := filepath.Join(req.App.Config.QueuePath, project.Path, header.Filename)
	virtPath := filepath.Join(project.Path, header.Filename)

	if expire > 0 {
		expRes := server.ExpirationResult{
			Original: expire.String(),
			Keep:     expire,
		}
		req.App.ProjectDB.SetRemoteExpirationOverride(virtPath, expRes)
	}

	out, err := os.Create(path)
	if err != nil {
		msg := fmt.Sprintf("can't create file: %s", err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}

	_, err = io.Copy(out, formFile)
	if err != nil {
		msg := fmt.Sprintf("can't copy file: %s", err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}

	err = os.Chtimes(path, modTime, modTime)
	if err != nil {
		msg := fmt.Sprintf("can't set modTime: %s", err.Error())
		req.App.Log.Error(projectName, msg)
		http.Error(req.Response, msg, 500)
		return
	}

	req.App.Log.Infof(projectName, "file '%s' (%s) is manually uploaded by key '%s', expire: %s", header.Filename, projectName, req.APIKey.Comment, expire.String())

	message := fmt.Sprintf("file '%s' uploaded, waiting final storage for more details…\nIt may take a few minutes.\nYou can safely break the command now with CTRL+c if you want.\n", header.Filename)
	req.Response.WriteHeader(http.StatusOK)
	req.Response.Write([]byte(message))
	if f, ok := req.Response.(http.Flusher); ok {
		f.Flush()
	}

	// wait for the file to be added to the project
	var projectFile *server.File
	for {
		projectFile = req.App.ProjectDB.FindFile(projectName, header.Filename)
		if projectFile != nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	maxExpire := projectFile.ExpireRemote
	if projectFile.ExpireLocal.After(projectFile.ExpireRemote) {
		maxExpire = projectFile.ExpireLocal
	}

	message = fmt.Sprintf("---\nExpire: %s\nLifetime cost: %.2f\n", maxExpire.Format("2006-01-02 15:04"), projectFile.Cost)
	req.Response.Write([]byte(message))
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
