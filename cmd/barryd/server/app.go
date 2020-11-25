package server

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/OnitiFR/barry/common"
)

// App describes an application
type App struct {
	StartTime   time.Time
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
		StartTime: time.Now(),
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

// Status returns informations about server
func (app *App) Status() (*common.APIStatus, error) {
	var ret common.APIStatus

	dbStats, err := app.ProjectDB.Stats()
	if err != nil {
		return nil, err
	}

	ret.StartTime = app.StartTime
	ret.ProjectCount = dbStats.ProjectCount
	ret.FileCount = dbStats.FileCount
	ret.TotalFileSize = dbStats.TotalSize
	ret.TotalFileCost = dbStats.TotalCost

	return &ret, nil
}
