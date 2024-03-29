package server

import (
	"fmt"
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
	Encrypted       bool
	ReEncryptDate   time.Time
	RetrievedPath   string
	RetrievedDate   time.Time
	retriever       *Retriever
	pushers         map[string]Pusher // one push per destination
}

// FileMap is a map of File
type FileMap map[string]*File

// ProjectName is a small helper to return the dir/project name of the file
func (file *File) ProjectName() string {
	return filepath.Dir(file.Path)
}

// CheckInit will init all fields of the *File fileds that needs it
func (file *File) checkInit() {
	if file.pushers == nil {
		file.pushers = make(map[string]Pusher)
	}
}

func (file *File) GetPusher(destination string) Pusher {
	file.checkInit()
	return file.pushers[destination]
}

// GetLocalPath will return the hypothetical local path of the file (valid only if file is available)
func (file *File) GetLocalPath(app *App) (string, error) {
	path := ""
	if !file.ExpiredLocal {
		path, _ = app.LocalStoragePath(FileStorageName, file.Path)
	} else if file.RetrievedPath != "" {
		path = file.RetrievedPath
	}

	if path == "" {
		return "", fmt.Errorf("can't file local path for file '%s' of project '%s'", file.Filename, file.ProjectName())
	}

	return path, nil
}
