package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/OnitiFR/barry/cmd/barryd/server"
	"github.com/OnitiFR/barry/common"
)

func getEntryFromRequest(req *server.Request) (*server.Project, error) {
	projectName := req.HTTP.FormValue("project")

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
			req.App.Log.Error(name, msg)
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
		req.App.Log.Error(project.Path, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}

	for _, name := range fileNames {
		file := req.App.ProjectDB.FindFile(project.Path, name)
		if file == nil {
			msg := fmt.Sprintf("error with file '%s'", name)
			req.App.Log.Error(project.Path, msg)
			http.Error(req.Response, msg, 500)
			return
		}
		retrieved := false
		if file.RetrievedPath != "" {
			retrieved = true
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
			Retrieved:     retrieved,
		})
	}

	enc := json.NewEncoder(req.Response)
	err = enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(project.Path, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}
}

// InfosProjectController will return various infos and settings of a project
func InfosProjectController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")

	project, err := getEntryFromRequest(req)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 404)
		return
	}

	var totalSize int64
	var totalCost float64
	newestModTime := time.Time{}
	finalExpiration := time.Time{}
	for _, file := range project.Files {
		totalSize += file.Size
		totalCost += file.Cost
		if file.ModTime.After(newestModTime) {
			newestModTime = file.ModTime
		}
		// get farthest expiration (remote vs local)
		lastExpire := file.ExpireRemote
		if file.ExpireLocal.After(lastExpire) {
			lastExpire = file.ExpireLocal
		}

		if finalExpiration.IsZero() {
			finalExpiration = lastExpire
		}
		if lastExpire.After(finalExpiration) {
			finalExpiration = lastExpire
		}
	}

	retData := common.APIProjectInfos{
		FileCountCurrent:    len(project.Files),
		SizeCountCurrent:    totalSize,
		CostCurrent:         totalCost,
		Archived:            project.Archived,
		BackupEvery:         project.BackupEvery,
		NewestModTime:       newestModTime,
		FinalExpiration:     finalExpiration,
		LocalExpirationStr:  project.LocalExpiration.String(),
		RemoteExpirationStr: project.RemoteExpiration.String(),
	}

	enc := json.NewEncoder(req.Response)
	err = enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(project.Path, err.Error())
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

func SettingProjectController(req *server.Request) {
	project, err := getEntryFromRequest(req)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 404)
		return
	}
	setting := req.HTTP.FormValue("setting")
	value := req.HTTP.FormValue("value")

	var exp server.Expiration

	switch setting {
	case "backup_every":
		err = projectControllerSetBackupEvery(project, value)
	case "local_expiration":
		exp, err = projectControllerParseExpiration(project, value)
		if err == nil {
			project.LocalExpiration = exp
		}
	case "remote_expiration":
		exp, err = projectControllerParseExpiration(project, value)
		fmt.Println(err)
		if err == nil {
			project.RemoteExpiration = exp
		}
	default:
		err = fmt.Errorf("unknown setting '%s'", setting)
	}

	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 400)
		return
	}

	err = req.App.ProjectDB.Save()
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}

	req.Printf("project '%s': setting '%s' updated\n", project.Path, setting)
}

func projectControllerSetBackupEvery(project *server.Project, value string) error {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return err
	}

	if duration < 1*time.Minute {
		return errors.New("duration is too low")
	}

	project.BackupEvery = duration
	return nil
}

func projectControllerParseExpiration(project *server.Project, value string) (server.Expiration, error) {
	parts := strings.Split(value, ",")
	exp, err := server.ParseExpiration(parts)
	if err != nil {
		return exp, err
	}

	// this now a custom settings, it will not be erased on restart
	exp.Custom = true
	return exp, nil
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
