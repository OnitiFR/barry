package main

// Project is a project (directory with a leat one File) in our DB
type Project struct {
	Path  string
	Files FileMap
	// createdAt?
}

// ProjectMap is a map of Project
type ProjectMap map[string]*Project

// NewProject create a new Project struct
func NewProject(path string) *Project {
	return &Project{
		Path:  path,
		Files: make(FileMap),
	}
}
