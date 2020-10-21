package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileStorageName is the name of the storage sub-directory
// where local files are stored
const FileStorageName = "files"

// LogHistorySize is the maximum number of messages in app log history
const LogHistorySize = 5000

// RetryDelay is used when an upload/move failed
const RetryDelay = 15 * time.Minute

// QueueScanDelay is the delay between consecutive queue scans
const QueueScanDelay = 1 * time.Minute

// QueueStableDelay determine how long a file should stay the same (mtime+size)
// to be considered stable.
const QueueStableDelay = 2 * time.Minute

// KeepAliveDelayDays is the number of days between each keep-alive/stats report
const KeepAliveDelayDays = 5

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
}

// NewApp create a new application
func NewApp(config *AppConfig) (*App, error) {

	app := &App{
		Config: config,
	}
	return app, nil
}

// Init the application
func (app *App) Init(trace bool, pretty bool) error {
	app.LogHistory = NewLogHistory(LogHistorySize)
	app.Log = NewLog(trace, pretty, app.LogHistory)
	app.Log.Infof(MsgGlob, "starting barry version %s", Version)

	dataBaseFilename, err := app.LocalStoragePath("data", "projects.db")
	if err != nil {
		return err
	}

	localStoragePath, err := app.LocalStoragePath(FileStorageName, "")
	if err != nil {
		return err
	}

	db, err := NewProjectDatabase(dataBaseFilename, localStoragePath, app.Config.Expiration, app.Log)
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

	app.Uploader = NewUploader(app.Config.NumUploaders, app.Swift, app.Log)

	app.AlertSender, err = NewAlertSender(app.Config.configPath, app.Log)
	if err != nil {
		return err
	}

	app.Stats = NewStats()
	app.RunKeepAliveStats(KeepAliveDelayDays)

	// start services
	app.Uploader.Start()
	go app.ProjectDB.ScheduleExpireFiles()

	return nil
}

// Run the app (block, will never return)
func (app *App) Run() {
	for {
		err := app.WaitList.Scan()
		if err != nil {
			// TODO: add external error reporting
			app.Log.Errorf(MsgGlob, "queue scan error: %s", err)
		}
		time.Sleep(QueueScanDelay)
	}
}

// RunKeepAliveStats will send a keepalive alert with stats every X days
func (app *App) RunKeepAliveStats(daysInterval int) {
	go func() {
		for {
			time.Sleep(24 * time.Hour * time.Duration(daysInterval))
			app.AlertSender.Send(&Alert{
				Type:    AlertTypeGood,
				Subject: "Hi",
				Content: app.Stats.Report(fmt.Sprintf("since %d day(s)", daysInterval)),
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
	file.Status = FileStatusUploading

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

	file.ExpireLocal = time.Now().Add(localExpiration.Keep)
	file.ExpireLocalOrg = localExpiration.Original
	file.ExpireRemote = time.Now().Add(remoteExpiration.Keep)
	file.ExpireRemoteOrg = remoteExpiration.Original

	// we must no block the Scan, so we use a goroutine
	go func() {
		err = app.UploadAndStore(projectName, &file)
		if err != nil {
			go app.unqueueFile(projectName, file, err)
			return
		}
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
