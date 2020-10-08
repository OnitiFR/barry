package main

import "time"

// File is a file in our DB (final leaf)
type File struct {
	Filename string
	Path     string
	ModTime  time.Time
	Size     int64
	AddedAt  time.Time
	// expirationLocal?
	// expirationRemote?
}

// FileMap is a map of File
type FileMap map[string]*File
