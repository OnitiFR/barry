package server

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/c2h5oh/datasize"
)

// progressReader wraps an io.Reader and atomically counts the bytes read,
// allowing upload progress to be reported concurrently.
type progressReader struct {
	reader  io.Reader
	written *int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	atomic.AddInt64(pr.written, int64(n))
	return n, err
}

// statusRefreshInterval is how often the upload progress status is refreshed
const statusRefreshInterval = 5 * time.Second

// statusSpeedSmoothing is the weight (0..1) given to the latest measured speed
// in the exponential moving average. Higher = more reactive, lower = smoother.
const statusSpeedSmoothing = 0.3

// statusMaxETA caps the displayed ETA: above it we consider the estimate
// meaningless (and avoid time.Duration overflow on near-zero speeds).
const statusMaxETA = 30 * 24 * time.Hour

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

	statusMutex sync.Mutex
	status      []string
}

// NewUploader initialize a new instance
func NewUploader(numWorkers int, swift *Swift, log *Log) *Uploader {
	return &Uploader{
		NumWorkers: numWorkers,
		Channel:    make(chan *Upload),
		Swift:      swift,
		Log:        log,
		status:     make([]string, numWorkers),
	}
}

// setStatus updates the status of worker id (1-based) in a concurrency-safe way
func (up *Uploader) setStatus(id int, status string) {
	up.statusMutex.Lock()
	up.status[id-1] = status
	up.statusMutex.Unlock()
}

// StatusSnapshot returns a copy of all workers status, safe to read concurrently
func (up *Uploader) StatusSnapshot() []string {
	up.statusMutex.Lock()
	defer up.statusMutex.Unlock()
	out := make([]string, len(up.status))
	copy(out, up.status)
	return out
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

	up.setStatus(id, "idle")
	up.Log.Tracef(MsgGlob, "upload worker %d: waiting", id)
	upload := <-up.Channel

	// make sure we always fill result chan
	defer func() {
		upload.Result <- err
	}()

	upload.Tries++
	upload.LastTry = time.Now()

	up.setStatus(id, fmt.Sprintf("uploading %s (%s)", upload.File.Filename, upload.File.Container))
	up.Log.Infof(upload.File.ProjectName(), "worker %d: uploading %s", id, upload.File.Filename)

	// report upload progress in the worker status
	var written int64
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(statusRefreshInterval)
		defer ticker.Stop()

		lastTime := time.Now()
		var lastWritten int64
		var speed float64 // exponential moving average, bytes/sec

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				now := time.Now()
				w := atomic.LoadInt64(&written)

				// instantaneous speed over the real elapsed time, smoothed
				// into an exponential moving average to absorb the bursts
				// caused by chunked-upload buffering.
				if dt := now.Sub(lastTime).Seconds(); dt > 0 {
					inst := float64(w-lastWritten) / dt
					if speed == 0 {
						speed = inst
					} else {
						speed = statusSpeedSmoothing*inst + (1-statusSpeedSmoothing)*speed
					}
				}
				lastWritten = w
				lastTime = now

				status := fmt.Sprintf("uploading %s (%s)", upload.File.Filename, upload.File.Container)
				if upload.File.Size > 0 {
					status += fmt.Sprintf(" %d%%", w*100/upload.File.Size)
				}
				if speed > 0 {
					status += fmt.Sprintf(" %s/s", datasize.ByteSize(speed).HR())
					if upload.File.Size > 0 {
						remaining := upload.File.Size - w
						if remaining < 0 {
							remaining = 0
						}
						// guard on the seconds value before building a
						// time.Duration, to avoid int64-nanosecond overflow.
						if etaSeconds := float64(remaining) / speed; etaSeconds < statusMaxETA.Seconds() {
							eta := time.Duration(etaSeconds) * time.Second
							status += fmt.Sprintf(" ETA %s", eta.Round(time.Second))
						}
					}
				}
				up.setStatus(id, status)
			}
		}
	}()

	err = up.Swift.Upload(upload.File, &written)
	close(done)
	<-finished
	if err != nil {
		up.Log.Errorf(upload.File.ProjectName(), "worker %d: upload error with %s: %s", id, upload.File.Filename, err)
	} else {
		up.Log.Infof(upload.File.ProjectName(), "worker %d: done uploading %s", id, upload.File.Filename)
	}
}
