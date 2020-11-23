package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/OnitiFR/barry/cmd/barryd/server"
	"github.com/OnitiFR/barry/common"
)

// FileStatusController returns file status
func FileStatusController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")

	fullPath, err := url.PathUnescape(req.SubPath)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 404)
		return
	}

	projectName := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)

	file := req.App.ProjectDB.FindFile(projectName, fileName)
	if file == nil {
		msg := fmt.Sprintf("error with file '%s' in project '%s'", fileName, projectName)
		req.App.Log.Error(server.MsgGlob, msg)
		http.Error(req.Response, msg, 500)
		return
	}

	var retData common.APIFileStatus

	// … or retrieved!
	if file.ExpiredLocal == false {
		retData.Status = common.APIFileStatusAvailable
		retData.ETA = 0
	} else {
		// TODO: things :)
	}

	enc := json.NewEncoder(req.Response)
	err = enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}
}

// FileDownloadController return the file stream
func FileDownloadController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/octet-stream")

	fullPath, err := url.PathUnescape(req.SubPath)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 404)
		return
	}

	projectName := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)

	file := req.App.ProjectDB.FindFile(projectName, fileName)
	if file == nil {
		msg := fmt.Sprintf("error with file '%s' in project '%s'", fileName, projectName)
		req.App.Log.Error(server.MsgGlob, msg)
		http.Error(req.Response, msg, 500)
		return
	}
	// TODO: (double)check file status (unsealed + retrieved)
	dest, _ := req.App.LocalStoragePath(server.FileStorageName, file.Path)

	http.ServeFile(req.Response, req.HTTP, dest)
}
