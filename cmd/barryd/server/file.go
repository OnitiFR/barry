package server

import (
	"path/filepath"
	"time"
)

// FileStatus list all possible status for a file in WaitList and ProjectDB
const (
	FileStatusNew       = "new"
	FileStatusQueued    = "queued"
	FileStatusUploading = "uploading"
	FileStatusUploaded  = "uploaded"
)

// File is a file in our DB (final leaf)
type File struct {
	Filename        string
	Path            string
	ModTime         time.Time
	Size            int64
	AddedAt         time.Time
	Status          string
	ExpireLocal     time.Time // expiration date
	ExpireRemote    time.Time // (same)
	ExpireLocalOrg  string    // original expire string
	ExpireRemoteOrg string    // (same)
	RemoteKeep      time.Duration
	ExpiredLocal    bool
	ExpiredRemote   bool
	Container       string
	Cost            float64
	RetrievedPath   string
	RetrievedDate   time.Time
	retriever       *Retriever `json:"-"`
}

// FileMap is a map of File
type FileMap map[string]*File

// ProjectName is a small helper to return the dir/project name of the file
func (file *File) ProjectName() string {
	return filepath.Dir(file.Path)
}
