package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"sync"
	"time"
)

// ProjectDatabase is a Project database holder
type ProjectDatabase struct {
	filename          string
	localStoragePath  string
	projects          ProjectMap
	defaultExpiration *ExpirationConfig
	log               *Log
	mutex             sync.Mutex
	deleteLocalFunc   ProjectDBDeleteLocalFunc
	deleteRemoteFunc  ProjectDBDeleteRemoteFunc
	noBackupAlertFunc ProjectDBNoBackupAlertFunc
}

// ProjectDBDeleteLocalFunc is called when a local file expires (as a goroutine)
type ProjectDBDeleteLocalFunc func(file *File, filePath string)

// ProjectDBDeleteRemoteFunc is called when a remote file expirtes (as a goroutine)
type ProjectDBDeleteRemoteFunc func(file *File)

// ProjectDBNoBackupAlertFunc is called when a backup is missing for a project
type ProjectDBNoBackupAlertFunc func(projects []*Project)

// NewProjectDatabase allocates a new ProjectDatabase
func NewProjectDatabase(
	filename string,
	localStoragePath string,
	defaultExpiration *ExpirationConfig,
	deleteLocalFunc ProjectDBDeleteLocalFunc,
	deleteRemoteFunc ProjectDBDeleteRemoteFunc,
	noBackupAlertFunc ProjectDBNoBackupAlertFunc,
	log *Log,
) (*ProjectDatabase, error) {
	db := &ProjectDatabase{
		filename:          filename,
		localStoragePath:  localStoragePath,
		projects:          make(ProjectMap),
		defaultExpiration: defaultExpiration,
		deleteLocalFunc:   deleteLocalFunc,
		deleteRemoteFunc:  deleteRemoteFunc,
		noBackupAlertFunc: noBackupAlertFunc,
		log:               log,
	}
	// if the file exists, load it
	if _, err := os.Stat(db.filename); err == nil {
		err = db.load()
		if err != nil {
			return nil, err
		}
	}

	err := db.upgrade()
	if err != nil {
		return nil, err
	}

	// save the file to check if it's writable
	// (no mutex lock, we're still in the single main thread)
	err = db.save()
	if err != nil {
		return nil, err
	}

	return db, nil

}

// upgrade projects schema version
func (db *ProjectDatabase) upgrade() error {
	for _, project := range db.projects {
		err := project.upgrade()
		if err != nil {
			return err
		}
	}
	return nil
}

// you should lock the mutex before calling save() & load()
// TODO: add a cooldown, massive queuing stress this function a lot
func (db *ProjectDatabase) save() error {
	f, err := os.Create(db.filename)
	if err != nil {
		return err
	}
	defer f.Close()

	db.log.Trace(MsgGlob, "saving ProjectDatabase")

	enc := json.NewEncoder(f)
	err = enc.Encode(&db.projects)
	if err != nil {
		return err
	}

	return nil
}

func (db *ProjectDatabase) load() error {
	f, err := os.Open(db.filename)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	err = dec.Decode(&db.projects)
	if err != nil {
		return err
	}
	return nil
}

// Save the database to the disk (public mutex-protected version of save())
func (db *ProjectDatabase) Save() error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	return db.save()
}

// GetNames returns all projects names, sorted
func (db *ProjectDatabase) GetNames() []string {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	keys := make([]string, len(db.projects))
	i := 0
	for key := range db.projects {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	return keys
}

// GetFilenames returns all filenames of a project, sorted by mtime (newer last)
func (db *ProjectDatabase) GetFilenames(projectName string) ([]string, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	project, exists := db.projects[projectName]
	if !exists {
		return nil, fmt.Errorf("project '%s' not found", projectName)
	}

	// create a slice from the map
	slice := NewFileMtimeSort(project.Files)

	// sort by mtime
	sort.Sort(slice)

	// extract only filenames
	names := make([]string, 0, len(slice))
	for _, file := range slice {
		names = append(names, file.Filename)
	}

	return names, nil
}

// FindFile finds a file based on projectName and its fileName
func (db *ProjectDatabase) FindFile(projectName string, fileName string) *File {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	project, pExists := db.projects[projectName]
	if !pExists {
		return nil
	}

	file, fExists := project.Files[fileName]
	if !fExists {
		return nil
	}
	return file
}

// FindOrCreateProject will return an existing project or create a new one if needed
func (db *ProjectDatabase) FindOrCreateProject(projectName string) (*Project, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	project, projectExists := db.projects[projectName]

	if !projectExists {
		project = NewProject(projectName, db.defaultExpiration)
		db.projects[projectName] = project

		err := db.save()
		if err != nil {
			return nil, err
		}
	}

	return project, nil
}

// AddFile will add a file to the database to a specific project
func (db *ProjectDatabase) AddFile(projectName string, file *File) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	project, projectExists := db.projects[projectName]
	if !projectExists {
		return fmt.Errorf("project '%s' does not exists in database, can't add '%s'", projectName, file.Filename)
	}

	_, fileExists := project.Files[file.Filename]
	if fileExists {
		return fmt.Errorf("file '%s' already exists in database for project '%s", file.Filename, projectName)
	}

	project.Files[file.Filename] = file
	project.FileCount++
	project.SizeCount += file.Size
	project.LastNoBackupAlert = time.Time{}

	err := db.save()
	if err != nil {
		return err
	}

	db.log.Infof(projectName, "%s/%s added to ProjectDatabase", projectName, file.Filename)
	return nil
}

// GetProjectNextExpiration return next (= for next file) expiration values
func (db *ProjectDatabase) GetProjectNextExpiration(project *Project, modTime time.Time) (ExpirationResult, ExpirationResult, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	localExpiration := project.LocalExpiration.GetNext(modTime)
	remoteExpiration := project.RemoteExpiration.GetNext(modTime)

	// save, because GetNext have updated project's FileCount
	err := db.save()
	if err != nil {
		return ExpirationResult{}, ExpirationResult{}, err
	}

	return localExpiration, remoteExpiration, nil
}

// ScheduleExpireFiles will schedule file expiration tasks (call as a goroutine)
func (db *ProjectDatabase) ScheduleExpireFiles() {
	for {
		db.expireLocalFiles()
		db.expireRemoteFiles()
		db.expireClean()
		time.Sleep(CheckExpireEvery)
	}
}

// ExpireLocalFiles will scan the database, deleting expired files in local storage
func (db *ProjectDatabase) expireLocalFiles() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	dbModified := false

	for _, project := range db.projects {
		for _, file := range project.Files {
			if time.Now().After(file.ExpireLocal) && !file.ExpiredLocal {
				filePath := path.Clean(db.localStoragePath + "/" + file.Path)

				// deleting file in a subroutine, because it may take quite some time
				// and we're locking the mutex.
				go db.deleteLocalFunc(file, filePath)

				file.ExpiredLocal = true
				dbModified = true
			}
		}
	}

	if dbModified == true {
		err := db.save()
		if err != nil {
			db.log.Errorf(MsgGlob, "error saving database: %s", err)
		}
	}
}

// expireRemoteFiles will scan the database, marking expired remote entries
func (db *ProjectDatabase) expireRemoteFiles() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	dbModified := false

	for _, project := range db.projects {
		for _, file := range project.Files {
			if time.Now().After(file.ExpireRemote) && !file.ExpiredRemote {
				// mutex is lock, use a goroutine
				go db.deleteRemoteFunc(file)
				file.ExpiredRemote = true
				dbModified = true
			}
		}
	}

	if dbModified == true {
		err := db.save()
		if err != nil {
			db.log.Errorf(MsgGlob, "error saving database: %s", err)
		}
	}
}

// expireClean will removed expired entries from database
// TODO: remove empty projets
func (db *ProjectDatabase) expireClean() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	dbModified := false

	for _, project := range db.projects {
		for fileKey, file := range project.Files {
			if file.ExpiredLocal && file.ExpiredRemote {
				dbModified = true
				delete(project.Files, fileKey)
				db.log.Infof(file.ProjectName(), "file '%s' removed from databse", file.Path)
			}
		}
	}

	if dbModified == true {
		err := db.save()
		if err != nil {
			db.log.Errorf(MsgGlob, "error saving database: %s", err)
		}
	}
}

// ScheduleNoBackupAlerts will schedule NoBackupAlerts task
func (db *ProjectDatabase) ScheduleNoBackupAlerts() {
	for {
		db.NoBackupAlerts()
		time.Sleep(NoBackupAlertSchedule)
	}
}

// NoBackupAlerts will alert when no backup is found in time for a project
func (db *ProjectDatabase) NoBackupAlerts() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	now := time.Now()

	dbModified := false
	noBackupProjects := make([]*Project, 0)

	for _, project := range db.projects {
		modTime := project.ModTime()
		if modTime.IsZero() {
			continue
		}
		diff := now.Sub(modTime)
		threshold := project.BackupEvery + (project.BackupEvery / 2)
		if diff > threshold {
			// backup is missing, did we need to send another alert?
			if now.Sub(project.LastNoBackupAlert) > project.BackupEvery {
				db.log.Errorf(project.Path, "missing backup (BackupEvery=%s)", project.BackupEvery)
				noBackupProjects = append(noBackupProjects, project)
				project.LastNoBackupAlert = now
				dbModified = true
			}
		}
	}

	if len(noBackupProjects) > 0 {
		db.noBackupAlertFunc(noBackupProjects)
	}

	if dbModified == true {
		err := db.save()
		if err != nil {
			db.log.Errorf(MsgGlob, "error saving database: %s", err)
		}
	}

}
