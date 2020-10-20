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

// App describes an application
type App struct {
	Config     *AppConfig
	ProjectDB  *ProjectDatabase
	WaitList   *WaitList
	Uploader   *Uploader
	Swift      *Swift
	Log        *Log
	LogHistory *LogHistory
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

	// start services
	app.Uploader.Start()
	go app.ProjectDB.ScheduleExpireFiles()

	return nil
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
	project, err := app.ProjectDB.FindOrCreateProject(projectName)
	if err != nil {
		return err
	}

	localExpiration, remoteExpiration, err := app.ProjectDB.GetProjectNextExpiration(project, file.ModTime)
	if err != nil {
		return err
	}

	file.ExpireLocal = time.Now().Add(localExpiration.Keep)
	file.ExpireLocalOrg = localExpiration.Original
	file.ExpireRemote = time.Now().Add(remoteExpiration.Keep)
	file.ExpireRemoteOrg = remoteExpiration.Original
	file.Status = FileStatusUploading

	upload := NewUpload(projectName, file)

	// send to upload worker, and wait
	app.Uploader.Channel <- upload
	err = <-upload.Result

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

// queueFile is called when a file is ready to be uploaded
// We're called by the WaitList as a go routine, so we have no way to
// "inform back" the list in case of a failure. Something must be done about
// this ;)
// - the file should return to the queue?
// - we set its status back using a callback ?
// - retry from here?
func (app *App) queueFile(projectName string, file File) {

	err := app.UploadAndStore(projectName, &file)
	if err != nil {
		app.Log.Errorf(projectName, "error with '%s': %s", file.Path, err)
	}
}
