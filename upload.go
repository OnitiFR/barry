package main

import (
	"fmt"
	"time"
)

// Upload is for the whole lifecycle of an upload, from request to completion
type Upload struct {
	// input parameters
	ProjectName string
	File        *File

	// output chan
	Result chan error

	// lifecycle members
	Tries   int
	LastTry time.Time
	// LastError error
}

// Uploader will manage workers
type Uploader struct {
	NumWorkers int
	Channel    chan *Upload
	Swift      *Swift
}

// NewUploader initialize a new instance
func NewUploader(numWorkers int, swift *Swift) *Uploader {
	return &Uploader{
		NumWorkers: numWorkers,
		Channel:    make(chan *Upload, numWorkers),
		Swift:      swift,
	}
}

// NewUpload initialize a new instance
func NewUpload(projectName string, file *File) *Upload {
	return &Upload{
		ProjectName: projectName,
		File:        file,
		Result:      make(chan error, 1),
	}
}

// Start the Uploader (run workers)
func (up *Uploader) Start() {
	for i := 0; i < up.NumWorkers; i++ {
		go func(id int) {
			for {
				up.worker(id + 1)
			}
		}(i)
	}
}

func (up *Uploader) worker(id int) {
	var err error
	err = nil

	fmt.Printf("worker %d: waiting\n", id)
	upload := <-up.Channel

	// make sure we always fill result chan
	defer func() {
		upload.Result <- err
	}()

	upload.Tries++
	upload.LastTry = time.Now()

	fmt.Printf("worker %d: uploading %s\n", id, upload.File.Filename)
	err = up.Swift.Upload(upload.File)
	if err != nil {
		fmt.Printf("worker %d: error with %s: %s\n", id, upload.File.Filename, err)
	} else {
		fmt.Printf("worker %d: done %s\n", id, upload.File.Filename)
	}
}
