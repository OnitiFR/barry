package server

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/OnitiFR/barry/common"
)

// App describes an application
type App struct {
	Config      *AppConfig
	ProjectDB   *ProjectDatabase
	WaitList    *WaitList
	Uploader    *Uploader
	Swift       *Swift
	Log         *Log
	LogHistory  *LogHistory
	AlertSender *AlertSender
	Stats       *Stats
	APIKeysDB   *APIKeyDatabase
	Rand        *rand.Rand
	MuxAPI      *http.ServeMux

	routesAPI map[string][]*Route
}

// NewApp create a new application
func NewApp(config *AppConfig) (*App, error) {

	app := &App{
		Config:    config,
		Rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
		routesAPI: make(map[string][]*Route),
		MuxAPI:    http.NewServeMux(),
	}
	return app, nil
}

// Init the application
func (app *App) Init(trace bool, pretty bool) error {
	app.LogHistory = NewLogHistory(LogHistorySize)
	app.Log = NewLog(trace, pretty, app.LogHistory)
	app.Log.Infof(MsgGlob, "starting barry version %s", common.ServerVersion)

	dataBaseFilename, err := app.LocalStoragePath("data", "projects.db")
	if err != nil {
		return err
	}

	localStoragePath, err := app.LocalStoragePath(FileStorageName, "")
	if err != nil {
		return err
	}

	app.AlertSender, err = NewAlertSender(app.Config.configPath, app.Log)
	if err != nil {
		return err
	}

	db, err := NewProjectDatabase(
		dataBaseFilename,
		localStoragePath,
		app.Config.Expiration,
		app.AlertSender,
		app.deleteLocal,
		app.deleteRemote,
		app.sendNoBackupAlert,
		app.Log)
	if err != nil {
		return err
	}
	app.ProjectDB = db

	waitList, err := NewWaitList(app.Config.QueuePath, app.waitListFilter, app.queueFile, app.Log)
	if err != nil {
		return err
	}
	app.WaitList = waitList

	app.Swift, err = NewSwift(app.Config)
	if err != nil {
		return err
	}
	app.Log.Trace(MsgGlob, "Swift connected")

	for _, container := range app.Config.Containers {
		err = app.Swift.CheckContainer(container.Name)
		if err != nil {
			return err
		}
		app.Log.Tracef(MsgGlob, "container '%s' is OK", container.Name)
	}

	app.Uploader = NewUploader(app.Config.NumUploaders, app.Swift, app.Log)

	app.Stats = NewStats()
	app.RunKeepAliveStats(KeepAliveDelayDays)

	keyDataBaseFilename, err := app.LocalStoragePath("data", "api-keys.db")
	if err != nil {
		return err
	}

	keysDB, err := NewAPIKeyDatabase(keyDataBaseFilename, app.Log, app.Rand)
	if err != nil {
		return err
	}
	app.APIKeysDB = keysDB

	// start services
	app.Uploader.Start()
	go app.ProjectDB.ScheduleExpireFiles()
	go app.ProjectDB.ScheduleNoBackupAlerts()
	go app.ScheduleScan()

	return nil
}

// ScheduleScan of the WaitList (block, will never return)
func (app *App) ScheduleScan() {
	for {
		err := app.WaitList.Scan()
		if err != nil {
			// TODO: add external error reporting
			app.Log.Errorf(MsgGlob, "queue scan error: %s", err)
		}
		time.Sleep(QueueScanDelay)
	}
}

// Run will start the app servers (foreground)
func (app *App) Run() {
	app.registerRouteHandlers(app.MuxAPI, app.routesAPI)

	errChan := make(chan error)

	go func() {
		// HTTP API Server
		app.Log.Infof(MsgGlob, "API server listening on %s (HTTP)", app.Config.API.Listen)
		err := http.ListenAndServe(app.Config.API.Listen, app.MuxAPI)
		errChan <- fmt.Errorf("ListenAndServe API server: %s", err)
	}()

	err := <-errChan
	log.Fatalf("error: %s", err)
}

// RunKeepAliveStats will send a keepalive alert with stats every X days
func (app *App) RunKeepAliveStats(daysInterval int) {
	go func() {
		for {
			if daysInterval != 0 {
				time.Sleep(24 * time.Hour * time.Duration(daysInterval))
			} else {
				// we're in dev mode
				time.Sleep(5 * time.Minute)
			}
			msg := app.Stats.Report(fmt.Sprintf("since %d day(s)", daysInterval))
			app.Log.Info(MsgGlob, msg)
			app.AlertSender.Send(&Alert{
				Type:    AlertTypeGood,
				Subject: "Hi",
				Content: msg,
			})
		}
	}()
}

// LocalStoragePath builds a path based on LocalStoragePath, and will create
// the (last) directory if needed
func (app *App) LocalStoragePath(dir string, filename string) (string, error) {
	path := app.Config.LocalStoragePath + "/" + dir
	err := CreateDirIfNeeded(path)
	if err != nil {
		return "", nil
	}
	return filepath.Clean(path + "/" + filename), nil
}

// MoveFileToStorage will move a file from the queue to our storage
func (app *App) MoveFileToStorage(file *File) error {
	source := filepath.Clean(app.Config.QueuePath + "/" + file.Path)
	dest, err := app.LocalStoragePath(FileStorageName, file.Path)
	if err != nil {
		return err
	}

	destDir := filepath.Dir(dest)
	err = os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		return err
	}

	err = os.Rename(source, dest)
	if err != nil {
		return err
	}
	app.Log.Infof(file.ProjectName(), "file '%s' moved to storage", file.Path)
	return nil
}

// UploadAndStore will upload and store a file
func (app *App) UploadAndStore(projectName string, file *File) error {
	// let's found the cheapest container for this file
	var minimumCost float64
	var bestContainer string
	for _, container := range app.Config.Containers {
		cost, err := container.Cost(file.Size, file.RemoteKeep)
		if err != nil {
			return fmt.Errorf("container cost evaluation error: %s", err)
		}
		app.Log.Tracef(projectName, "cost for container '%s': %f", container.Name, cost)
		if cost < minimumCost || bestContainer == "" {
			minimumCost = cost
			bestContainer = container.Name
		}
	}
	app.Log.Tracef(projectName, "using container '%s' for file '%s", bestContainer, file.Filename)

	file.Status = FileStatusUploading
	file.Cost = minimumCost
	file.Container = bestContainer

	upload := NewUpload(projectName, file)

	// send to upload worker, and wait
	app.Uploader.Channel <- upload
	err := <-upload.Result

	if err != nil {
		return fmt.Errorf("upload error: %s", err)
	}

	// move the file to the local storage
	err = app.MoveFileToStorage(file)
	if err != nil {
		return fmt.Errorf("move error: %s", err)
	}

	file.Status = FileStatusUploaded

	// add to database
	err = app.ProjectDB.AddFile(projectName, file)
	if err != nil {
		return err
	}

	app.Stats.Inc(1, file.Size)

	return nil
}

// MakeFileAvailable will do all the work needed to make the file available for download
// This action is asynchronous, the function will return current file status with an ETA.
// This function is designed to be called repetitively.
// TODO: add some sort of watcher to jump automatically from "unsealed" to "retrieving" status
func (app *App) MakeFileAvailable(file *File) (common.APIFileStatus, error) {
	var status common.APIFileStatus

	if file.ExpiredLocal == false {
		status.Status = common.APIFileStatusAvailable
		status.ETA = 0
		return status, nil
	}

	if file.RetrievedPath != "" {
		status.Status = common.APIFileStatusAvailable
		status.ETA = 0
		return status, nil
	}

	if file.retriever != nil {
		retriever := file.retriever

		if retriever.Finished {
			file.retriever = nil
			if retriever.Error != nil {
				return status, retriever.Error
			}
			app.Log.Infof(file.ProjectName(), "file '%s' retrieved", file.Filename)
			file.RetrievedPath = retriever.Path
			file.RetrievedDate = time.Now()
			app.ProjectDB.Save()

			status.Status = common.APIFileStatusAvailable
			status.ETA = 0
			return status, nil
		}

		status.Status = common.APIFileStatusRetrieving
		status.ETA = retriever.GetETA()
		return status, nil
	}

	// check remote status
	availability, eta, err := app.Swift.GetObjetAvailability(file.Container, file.Path)
	if err != nil {
		return status, err
	}

	switch availability {
	case SwiftObjectUnsealing:
		status.Status = common.APIFileStatusUnsealing
		status.ETA = eta
		return status, nil

	case SwiftObjectSealed:
		app.Log.Infof(file.ProjectName(), "unsealing '%s'", file.Path)
		eta, err := app.Swift.Unseal(file.Container, file.Path)
		if err != nil {
			return status, err
		}

		status.Status = common.APIFileStatusUnsealing
		status.ETA = eta
		return status, nil

	case SwiftObjectUnsealed:
		// TODO: allow two files from two different projects to be retrieved
		// even if they have the same name!
		path, err := app.LocalStoragePath(RetrievedStorageName, file.Filename)
		if err != nil {
			return status, err
		}
		file.retriever, err = NewRetriever(file, app.Swift, path)
		if err != nil {
			return status, err
		}
		app.Log.Infof(file.ProjectName(), "retrieving '%s'", file.Path)
		status.Status = common.APIFileStatusRetrieving
		time.Sleep(5 * time.Second)
		status.ETA = file.retriever.GetETA()
		return status, nil
	}

	return status, fmt.Errorf("unknown availability '%s'", availability)
}

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
