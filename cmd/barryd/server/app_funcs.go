package server

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/ncw/swift/v2"
)

// This file hosts all App "callbacks", the core logic of barryd

// should we add this file to the WaitList ?
func (app *App) waitListFilter(dirName string, fileName string) bool {
	// do not add the file again if it's already in the db
	return !app.ProjectDB.FileExists(dirName, fileName)
}

// queueFile is called when a file is ready to be uploaded, we must be non-blocking!
func (app *App) queueFile(projectName string, file File) {

	project, err := app.ProjectDB.FindOrCreateProject(projectName)
	if err != nil {
		go app.unqueueFile(projectName, file, err)
		return
	}

	localExpiration, remoteExpiration, err := app.ProjectDB.GetProjectNextExpiration(project, &file)
	if err != nil {
		go app.unqueueFile(projectName, file, err)
		return
	}

	prevFile := project.GetLatestFile()
	if prevFile != nil {
		if prevFile.Size > DiffAlertDisableIfLessThan || file.Size > DiffAlertDisableIfLessThan {
			sizeDiff := float64(file.Size-prevFile.Size) / float64(prevFile.Size) * 100

			if math.Abs(sizeDiff) > DiffAlertThresholdPerc {
				msg := fmt.Sprintf("size diff for '%s' is %.1f%% (was %s, now %s)", file.Path, sizeDiff, datasize.ByteSize(prevFile.Size).HR(), datasize.ByteSize(file.Size).HR())
				app.Log.Error(projectName, msg)
				app.AlertSender.Send(&Alert{
					Type:    AlertTypeBad,
					Subject: "Warning",
					Content: msg,
				})
			}
		}
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
	app.Log.Error(projectName, errorMsg)

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

	err := os.Remove(filePath)
	if err != nil {
		msg := fmt.Sprintf("error deleting local storage file '%s': %s", file.Path, err)
		app.Log.Error(file.ProjectName(), msg)
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
	for {
		app.Log.Tracef(file.ProjectName(), "deleting remote storage file '%s'", file.Path)

		err := app.Swift.Delete(file)

		// no error? log and exit
		if err == nil {
			app.Log.Infof(file.ProjectName(), "remote file '%s' deleted", file.Path)
			return
		}

		// not found? no need to retry → log, exit
		if err == swift.ObjectNotFound {
			msg := fmt.Sprintf("remote file '%s' not found", file.Path)
			app.Log.Error(file.ProjectName(), msg)
			app.AlertSender.Send(&Alert{
				Type:    AlertTypeBad,
				Subject: "Error",
				Content: msg,
			})
			return
		}

		// log and schedule a retry
		msg := fmt.Sprintf("error deleting remote file '%s': %s, will retry in %s", file.Path, err, RetryDelay)
		app.Log.Error(file.ProjectName(), msg)
		app.AlertSender.Send(&Alert{
			Type:    AlertTypeBad,
			Subject: "Error",
			Content: msg,
		})

		// wait before next try…
		time.Sleep(RetryDelay)
	}
}

func (app *App) sendNoBackupAlert(projects []*Project) {
	projectStrings := make([]string, 0)
	for _, project := range projects {
		diff := time.Since(project.ModTime()).Truncate(time.Second)
		projectStrings = append(projectStrings, fmt.Sprintf("%s (need every %s, was %s ago)", project.Path, project.BackupEvery, diff))
	}

	msg := fmt.Sprintf("missing backup for project(s) : %s", strings.Join(projectStrings, ", "))
	app.AlertSender.Send(&Alert{
		Type:    AlertTypeBad,
		Subject: "Error",
		Content: msg,
	})
}
