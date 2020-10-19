package main

import "time"

// Project is a project (directory with a leat one File) in our DB
type Project struct {
	Path             string
	Files            FileMap
	LocalExpiration  Expiration
	RemoteExpiration Expiration
}

// ProjectMap is a map of Project
type ProjectMap map[string]*Project

// NewProject create a new Project struct
func NewProject(path string, expirationConfig *ExpirationConfig) *Project {
	project := &Project{
		Path:  path,
		Files: make(FileMap),
	}

	if expirationConfig != nil {
		project.LocalExpiration = expirationConfig.Local
		project.RemoteExpiration = expirationConfig.Remote

		now := time.Now()
		project.LocalExpiration.ReferenceDate = now
		project.RemoteExpiration.ReferenceDate = now
	} else {
		project.LocalExpiration = Expiration{}
		project.RemoteExpiration = Expiration{}
	}

	return project
}
