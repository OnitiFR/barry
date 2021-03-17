package server

import (
	"io"
	"os"
	"sync"
	"time"
)

// Retriever is a structure that will host a Swift downloading thread
type Retriever struct {
	Path     string
	ETA      time.Duration
	Error    error
	Finished bool

	startedAt      time.Time
	totalSize      int64
	downloadedSize int64
	swiftFile      io.ReadCloser
	localFile      *os.File
	mutex          sync.Mutex
}

// NewRetriever create a new Retriever
func NewRetriever(file *File, swift *Swift, outputFilename string) (*Retriever, error) {
	res := &Retriever{
		startedAt: time.Now(),
		totalSize: file.Size,
		Path:      outputFilename,
	}

	var err error
	res.swiftFile, err = swift.ObjectOpen(file.Container, file.Path)
	if err != nil {
		return nil, err
	}

	res.localFile, err = os.Create(outputFilename)
	if err != nil {
		res.swiftFile.Close()
		return nil, err
	}

	go res.copy(1 * 1024 * 1024)
	return res, nil
}

func (r *Retriever) close(err error) {
	r.swiftFile.Close()
	r.localFile.Close()

	r.mutex.Lock()
	r.Error = err
	r.ETA = 0
	r.Finished = true
	r.mutex.Unlock()
}

func (r *Retriever) copy(bufferSize int64) {
	buf := make([]byte, bufferSize)
	for {
		n, err := r.swiftFile.Read(buf)
		if err != nil && err != io.EOF {
			r.close(err)
		}
		if n == 0 {
			break
		}

		if _, err := r.localFile.Write(buf[:n]); err != nil {
			r.close(err)
		}

		// update ETA
		r.downloadedSize += int64(n)
		done := float64(r.downloadedSize) / float64(r.totalSize)
		elapsed := time.Since(r.startedAt).Seconds()

		r.mutex.Lock()
		r.ETA = time.Second*time.Duration(elapsed/done) - time.Second*time.Duration(elapsed)
		r.mutex.Unlock()

	}

	r.close(nil)
}

// GetETA of download end
func (r *Retriever) GetETA() time.Duration {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.ETA
}
