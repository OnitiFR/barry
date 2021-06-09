package server

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
	Log        *Log
	Status     []string
}

// NewUploader initialize a new instance
func NewUploader(numWorkers int, swift *Swift, log *Log) *Uploader {
	return &Uploader{
		NumWorkers: numWorkers,
		Channel:    make(chan *Upload),
		Swift:      swift,
		Log:        log,
		Status:     make([]string, numWorkers),
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

	up.Status[id-1] = "idle"
	up.Log.Tracef(MsgGlob, "worker %d: waiting", id)
	upload := <-up.Channel

	// make sure we always fill result chan
	defer func() {
		upload.Result <- err
	}()

	upload.Tries++
	upload.LastTry = time.Now()

	up.Status[id-1] = fmt.Sprintf("uploading %s (%s)", upload.File.Filename, upload.File.Container)
	up.Log.Infof(upload.File.ProjectName(), "worker %d: uploading %s", id, upload.File.Filename)
	err = up.Swift.Upload(upload.File)
	if err != nil {
		up.Log.Errorf(upload.File.ProjectName(), "worker %d: upload error with %s: %s", id, upload.File.Filename, err)
	} else {
		up.Log.Infof(upload.File.ProjectName(), "worker %d: done uploading %s", id, upload.File.Filename)
	}
}
