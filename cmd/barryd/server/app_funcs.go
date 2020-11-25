package server

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/OnitiFR/barry/common"
)

// This file hosts all App "callbacks", the core logic of barryd

// should we add this file to the WaitList ?
func (app *App) waitListFilter(dirName string, fileName string) bool {
	if app.ProjectDB.FindFile(dirName, fileName) != nil {
		// no, this file is already in the database
		return false
	}
	return true
}

// queueFile is called when a file is ready to be uploaded, we must be non-blocking!
func (app *App) queueFile(projectName string, file File) {

	project, err := app.ProjectDB.FindOrCreateProject(projectName)
	if err != nil {
		go app.unqueueFile(projectName, file, err)
		return
	}

	localExpiration, remoteExpiration, err := app.ProjectDB.GetProjectNextExpiration(project, file.ModTime)
	if err != nil {
		go app.unqueueFile(projectName, file, err)
		return
	}

	file.ExpireLocal = file.ModTime.Add(localExpiration.Keep)
	file.ExpireLocalOrg = localExpiration.Original
	file.ExpireRemote = file.ModTime.Add(remoteExpiration.Keep)
	file.ExpireRemoteOrg = remoteExpiration.Original
	file.RemoteKeep = remoteExpiration.Keep

	// we must no block the Scan, so we use a goroutine
	go func() {
		err = app.UploadAndStore(projectName, &file)
		if err != nil {
			go app.unqueueFile(projectName, file, err)
			return
		}
		// clear all segments used by the file
		runtime.GC()
	}()
}

// unqueueFile is used when something went wrong and we need to put
// the file back in the queue.
func (app *App) unqueueFile(projectName string, file File, errIn error) {
	errorMsg := fmt.Sprintf("error with '%s': %s, will retry in %s", file.Path, errIn, RetryDelay)
	app.Log.Errorf(projectName, errorMsg)

	app.AlertSender.Send(&Alert{
		Type:    AlertTypeBad,
		Subject: "Error",
		Content: errorMsg,
	})

	time.Sleep(RetryDelay)
	app.Log.Tracef(projectName, "set '%s' for a retry", file.Path)
	app.WaitList.RemoveFile(projectName, file.Filename)

}

// deleteLocal is called by ProjectDB when a local file must be removed
func (app *App) deleteLocal(file *File, filePath string) {
	app.Log.Tracef(file.ProjectName(), "deleting local storage file '%s'", file.Path)

	// TODO: remove this? During first days of production, expiration was
	// erroneously based on Now() instead of file.ModTime, so we made some
	// manual cleaning of storage, but now it should report an error.
	if !common.PathExist(filePath) {
		app.Log.Warningf(file.ProjectName(), "error deleting local storage file '%s': file does not exists", file.Path)
		return
	}

	err := os.Remove(filePath)
	if err != nil {
		msg := fmt.Sprintf("error deleting local storage file '%s': %s", file.Path, err)
		app.Log.Errorf(file.ProjectName(), msg)
		app.AlertSender.Send(&Alert{
			Type:    AlertTypeBad,
			Subject: "Error",
			Content: msg,
		})
		return
	}
	app.Log.Infof(file.ProjectName(), "local storage file '%s' deleted", file.Path)
}

// deleteRemote is called by ProjectDB when a remote file must be removed
func (app *App) deleteRemote(file *File) {
	app.Log.Tracef(file.ProjectName(), "deleting remote storage file '%s'", file.Path)
	err := app.Swift.Delete(file)
	if err != nil {
		msg := fmt.Sprintf("error deleting remote file '%s': %s", file.Path, err)
		app.Log.Errorf(file.ProjectName(), msg)
		app.AlertSender.Send(&Alert{
			Type:    AlertTypeBad,
			Subject: "Error",
			Content: msg,
		})
		return
	}
	app.Log.Infof(file.ProjectName(), "remote file '%s' deleted", file.Path)
}

func (app *App) sendNoBackupAlert(projects []*Project) {
	projectStrings := make([]string, 0)
	for _, project := range projects {
		projectStrings = append(projectStrings, fmt.Sprintf("%s (%s)", project.Path, project.BackupEvery))
	}

	msg := fmt.Sprintf("missing backup for project(s) : %s", strings.Join(projectStrings, ", "))
	app.AlertSender.Send(&Alert{
		Type:    AlertTypeBad,
		Subject: "Error",
		Content: msg,
	})
}
