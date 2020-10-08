package main

import (
	"fmt"
	"sync"
)

// WaitList stores all files of the source path we're waiting for.
// (waiting means the fime must be stable [no change of size of mtime] for
// a determined period of time)
type WaitList struct {
	// liste de File ou de Project ?
	// - de projets (et on se fait un accesseur pour avoir la liste de tt les fichiers à plat ?)
	// qui construit la liste des projets de project_db du coup ? (et elle se rebuild seule si on la modifie?)
	// - le passage de la waitlist à project_db
	// est-ce qu'on a besoin de la notion de projet dans l'outil du coup ? (ou simple filtre sur le path)
	// - oui pour avoir un jour des params spécifiques à un projet
	WatchPath string
	Projects  ProjectMap
	mutex     sync.Mutex
}

// NewWaitList allocates a new WaitList
func NewWaitList(watchPath string) (*WaitList, error) {

	return &WaitList{
		WatchPath: watchPath,
		Projects:  make(ProjectMap),
	}, nil
}

// Dump list content on stdout (debug)
func (wl *WaitList) Dump() {
	wl.mutex.Lock()
	defer wl.mutex.Unlock()

	for _, project := range wl.Projects {
		fmt.Printf("- %s:\n", project.Path)
		for _, file := range project.Files {
			fmt.Printf("%s ", file.Filename)
		}
		fmt.Printf("\n")
	}
}
