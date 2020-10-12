package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// ProjectDatabase is a Project database holder
type ProjectDatabase struct {
	filename string
	projects ProjectMap
	mutex    sync.Mutex
}

// NewProjectDatabase allocates a new ProjectDatabase
func NewProjectDatabase(filename string) (*ProjectDatabase, error) {
	db := &ProjectDatabase{
		filename: filename,
		projects: make(ProjectMap),
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
