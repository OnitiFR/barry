package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StableDelay determine how long a file should stay the same (mtime+size)
// to be considered stable.
const StableDelay = 5 * time.Second

// WaitList stores all files of the source path we're waiting for.
// (waiting means the fime must be stable [no change of size of mtime] for
// a determined period of time)
type WaitList struct {
	watchPath  string
	projects   ProjectMap
	filterFunc WaitListFilterFunc
	queueFunc  WaitListQueueFunc
	mutex      sync.Mutex
}

// WaitListFilterFunc is used to filter incomming files
// return true to add the file to the WaitList, false to reject it
type WaitListFilterFunc func(dirName string, fileName string) bool

// WaitListQueueFunc is called when a new file is ready and has been
// queued in the WaitList. Called as a goroutine.
type WaitListQueueFunc func(projectName string, file File)

// NewWaitList allocates a new WaitList
func NewWaitList(watchPath string, filterFunc WaitListFilterFunc, queueFunc WaitListQueueFunc) (*WaitList, error) {
	if isDir, err := IsDir(watchPath); !isDir {
		return nil, fmt.Errorf("unable to watch directory '%s': %s", watchPath, err)
	}

	return &WaitList{
		watchPath:  watchPath,
		projects:   make(ProjectMap),
		filterFunc: filterFunc,
		queueFunc:  queueFunc,
	}, nil
}

// Dump list content on stdout (debug)
func (wl *WaitList) Dump() {
	wl.mutex.Lock()
	defer wl.mutex.Unlock()

	for _, project := range wl.projects {
		fmt.Printf("- %s:\n", project.Path)
		for _, file := range project.Files {
			fmt.Printf("  - %s ", file.Filename)
		}
		fmt.Printf("\n")
	}
}

// Scan the source directory to detect new files and add them to the list
func (wl *WaitList) Scan() error {
	wl.mutex.Lock()
	defer wl.mutex.Unlock()

	err := filepath.Walk(wl.watchPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("Walk: %s", err.Error())
			}

			if info.IsDir() {
				// reject directories starting with a dot
				if strings.HasPrefix(info.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}

			// reject files starting with a dot
			if strings.HasPrefix(info.Name(), ".") {
				return nil
			}

			relPath := strings.TrimPrefix(path, wl.watchPath+"/")
			dirName := filepath.Dir(relPath)
			fileName := filepath.Base(relPath)

			// apply filter
			if wl.filterFunc(dirName, fileName) == false {
				return nil
			}

			project, projectExists := wl.projects[dirName]

			if !projectExists {
				project = NewProject(dirName)
				wl.projects[dirName] = project
			}

			file, fileExists := project.Files[fileName]
			if fileExists {
				if file.Status == FileStatusQueued {
					return nil // already queued, ignore
				}
				if !info.ModTime().Equal(file.ModTime) || info.Size() != file.Size {
					// file changed, continue waiting
					file.ModTime = info.ModTime()
					file.Size = info.Size()
					file.AddedAt = time.Now()
					fmt.Printf("%s/%s changed, moar waiting\n", dirName, fileName)
				} else {
					// are we waiting for long enough?
					if file.AddedAt.Add(StableDelay).Before(time.Now()) {
						file.Status = FileStatusQueued
						go wl.queueFunc(project.Path, *file)
						fmt.Printf("%s/%s is ready, queued\n", dirName, fileName)
					} else {
						fmt.Printf("%s/%s still waiting\n", dirName, fileName)
					}
				}
			} else {
				file := &File{
					Filename: fileName,
					Path:     relPath,
					ModTime:  info.ModTime(),
					Size:     info.Size(),
					AddedAt:  time.Now(),
					Status:   FileStatusNew,
				}
				project.Files[fileName] = file
				fmt.Printf("%s/%s added to wait queue\n", dirName, fileName)
			}

			return nil
		})
	return err
}
