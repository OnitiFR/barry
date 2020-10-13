package main

import (
	"fmt"
	"path/filepath"
)

// App describes an application
type App struct {
	Config    *AppConfig
	ProjectDB *ProjectDatabase
	WaitList  *WaitList
	Uploader  *Uploader
	Swift     *Swift
}

// NewApp create a new application
func NewApp(config *AppConfig) (*App, error) {

	app := &App{
		Config: config,
	}
	return app, nil
}

// Init the application
func (app *App) Init() error {
	dataBaseFilename, err := app.LocalStoragePath("data", "projects.db")
	if err != nil {
		return err
	}

	db, err := NewProjectDatabase(dataBaseFilename)
	if err != nil {
		return err
	}
	app.ProjectDB = db

	waitList, err := NewWaitList(app.Config.QueuePath, app.waitListFilter, app.queueFile)
	if err != nil {
		return err
	}
	app.WaitList = waitList

	app.Swift, err = NewSwift(app.Config.Swift)
	if err != nil {
		return err
	}

	app.Uploader = NewUploader(app.Config.NumUploaders)

	// start services
	app.Uploader.Start()

	return nil
}

// LocalStoragePath builds a path based on LocalStoragePath
func (app *App) LocalStoragePath(dir string, filename string) (string, error) {
	path := app.Config.LocalStoragePath + "/" + dir
	err := CreateDirIfNeeded(path)
	if err != nil {
		return "", nil
	}
	return filepath.Clean(path + "/" + filename), nil
}

// should we add this file to the WaitList ?
func (app *App) waitListFilter(dirName string, fileName string) bool {
	if app.ProjectDB.FindFile(dirName, fileName) != nil {
		// this file is already in the database
		return false
	}
	return true
}

func (app *App) queueFile(projectName string, file File) {
	file.Status = FileStatusUploading

	upload := NewUpload(projectName, &file)

	// send to and wait upload worker
	app.Uploader.Channel <- upload
	err := <-upload.Result

	// what about errors? the file should return to the queue?
	// - we set its status back? retry from here?
	if err != nil {
		fmt.Printf("upload error: %s\n", err)
		return
	}

	file.Status = FileStatusUploaded

	// add to database
	app.ProjectDB.AddFile(projectName, &file)
}
