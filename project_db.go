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

// CheckExpireEvery is the delay between each expire task
const CheckExpireEvery = 15 * time.Minute

// ProjectDatabase is a Project database holder
type ProjectDatabase struct {
	filename          string
	localStoragePath  string
	projects          ProjectMap
	defaultExpiration *ExpirationConfig
	log               *Log
	mutex             sync.Mutex
}

// NewProjectDatabase allocates a new ProjectDatabase
func NewProjectDatabase(filename string, localStoragePath string, defaultExpiration *ExpirationConfig, log *Log) (*ProjectDatabase, error) {
	db := &ProjectDatabase{
		filename:          filename,
		localStoragePath:  localStoragePath,
		projects:          make(ProjectMap),
		defaultExpiration: defaultExpiration,
		log:               log,
	}
	// if the file exists, load it
	if _, err := os.Stat(db.filename); err == nil {
		err = db.load()
		if err != nil {
			return nil, err
		}
	}

	// save the file to check if it's writable
	// (no mutex lock, we're still in the single main thread)
	err := db.save()
	if err != nil {
		return nil, err
	}

	return db, nil

}

// you should lock the mutex before calling save() & load()
func (db *ProjectDatabase) save() error {
	f, err := os.Create(db.filename)
	if err != nil {
		return err
	}
	defer f.Close()

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

// AddFile will add a file to the database to a specific project
func (db *ProjectDatabase) AddFile(projectName string, file *File) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	project, projectExists := db.projects[projectName]

	if !projectExists {
		project = NewProject(projectName, db.defaultExpiration)
		db.projects[projectName] = project
	}

	_, fileExists := project.Files[file.Filename]
	if fileExists {
		return fmt.Errorf("file '%s' already exists in database for project '%s", file.Filename, projectName)
	}

	project.Files[file.Filename] = file

	err := db.save()
	if err != nil {
		return err
	}

	db.log.Infof(projectName, "%s/%s added to ProjectDatabase", projectName, file.Filename)
	return nil
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
				// and we're locking the mutex. We'll deal with errors on our own.
				// TODO: deal with errors ;)
				go func(filePath string) {
					db.log.Infof(file.ProjectName(), "deleting local storage file '%s'", file.Path)
					err := os.Remove(filePath)
					if err != nil {
						db.log.Errorf(file.ProjectName(), "error deleting local storage file '%s': %s", file.Path, err)
					}
				}(filePath)

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
				file.ExpiredRemote = true
				dbModified = true
				db.log.Infof(file.ProjectName(), "remote file '%s' marked as expired", file.Path)
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
