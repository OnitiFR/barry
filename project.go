package main

import "time"

// Project is a project (directory with a leat one File) in our DB
type Project struct {
	Path              string
	Files             FileMap
	FileCount         int
	SizeCount         int64
	LocalExpiration   Expiration
	RemoteExpiration  Expiration
	BackupEvery       time.Duration
	LastNoBackupAlert time.Time

	SchemaVersion int
}

// ProjectMap is a map of Project
type ProjectMap map[string]*Project

// ProjectNewestVersion is needed because each project record have a version
// and may be upgraded as application version goes. (see Upgrade() below)
// v0: original
// v1: added SchemaVersion + BackupEvery + LastNoBackupAlert
const ProjectNewestVersion = 1

// NewProject create a new Project struct
func NewProject(path string, expirationConfig *ExpirationConfig) *Project {
	project := &Project{
		Path:        path,
		Files:       make(FileMap),
		BackupEvery: ProjectDefaultBackupEvery,
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

// ModTime is the modification time of the project (aka the latest file's ModTime)
// If the project is empty, result is 0 (see IsZero())
func (p *Project) ModTime() time.Time {
	latest := time.Time{}
	for _, file := range p.Files {
		if file.ModTime.After(latest) {
			latest = file.ModTime
		}
	}
	return latest
}

// upgrade Project record to a newest schema version if needed
func (p *Project) upgrade() error {

	// v0 to v1
	if p.SchemaVersion == 0 {
		p.BackupEvery = ProjectDefaultBackupEvery
		p.SchemaVersion = 1
	}

	return nil
}
