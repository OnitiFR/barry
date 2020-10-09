package main

import "path/filepath"

// App describes an application
type App struct {
	Config    *AppConfig
	ProjectDB *ProjectDatabase
	WaitList  *WaitList
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

	waitList, err := NewWaitList(app.Config.QueuePath)
	if err != nil {
		return err
	}
	app.WaitList = waitList

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
