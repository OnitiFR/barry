package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/OnitiFR/barry/common"
)

// Pusher is a structure that will host an uploading thread
type PusherMulch struct {
	ETA      time.Duration
	Error    error
	Finished bool

	file   *File
	config *PusherConfig

	dest         io.Writer
	src          *os.File
	startedAt    time.Time
	uploadedSize int64
	mutex        sync.Mutex
}

// from mulch common/message.go file
type mulchMessage struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Target  string    `json:"target"`
	Message string    `json:"message"`
}

// NewPusherMulch create a new Pusher to mulch
func NewPusherMulch(file *File, path string, expire time.Duration, config *PusherConfig, log *Log) (Pusher, error) {
	p := &PusherMulch{
		startedAt: time.Now(),
		file:      file,
		config:    config,
	}

	if config.Type != PusherTypeMulch {
		return nil, errors.New("invalid pusher type for PusherMulch")
	}

	// add ourself to *File pushers (or return an existing instance, if any)
	previous := file.GetPusher(config.Name)
	if previous != nil {
		return previous, nil
	}
	file.pushers[config.Name] = p

	apiURL, err := common.CleanURL(config.URL + "/backup")
	if err != nil {
		return nil, err
	}

	var req *http.Request

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)

	// goroutine: copy
	go func() {
		defer pipeWriter.Close()

		var err error

		// set expire form field
		err = multipartWriter.WriteField("expire", strconv.Itoa(int(expire.Seconds())))
		if err != nil {
			p.error(err)
			return
		}

		p.dest, err = multipartWriter.CreateFormFile("file", file.Filename)
		if err != nil {
			p.error(err)
			return
		}

		p.src, err = os.Open(path)
		if err != nil {
			p.error(err)
			return
		}
		defer p.src.Close()

		err = p.copy(1 * 1024 * 1024)
		if err != nil {
			p.error(err)
			return
		}

		err = multipartWriter.Close()
		if err != nil {
			p.error(err)
			return
		}
	}()

	req, err = http.NewRequest("POST", apiURL, pipeReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	req.Header.Set("Mulch-Key", config.Key)
	req.Header.Set("Mulch-Protocol", strconv.Itoa(1))

	// goroutine: do and fetch the result
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			p.error(err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				p.error(err)
				return
			}
			p.error(fmt.Errorf("mulch returned %d: %s", resp.StatusCode, body))
		}

		// parse all lines of mulch to get success or failure
		var lastError error

		dec := json.NewDecoder(resp.Body)
		for {
			var m mulchMessage
			err := dec.Decode(&m)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Errorf(MsgGlob, "unable to parse Mulch message: %s", err.Error())
			}

			if m.Type == "FAILURE" {
				lastError = fmt.Errorf("mulch FAILURE: %s", m.Message)
			}
		}

		p.error(lastError)
	}()

	return p, nil
}

func (p *PusherMulch) error(err error) {
	p.mutex.Lock()
	p.Error = err
	p.ETA = 0
	p.Finished = true
	p.mutex.Unlock()

	// defer our own removal from pushers after some time
	defer func() {
		time.Sleep(30 * time.Second)
		delete(p.file.pushers, p.config.Name)
	}()
}

func (p *PusherMulch) copy(bufferSize int64) error {
	buf := make([]byte, bufferSize)
	for {
		n, err := p.src.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		if _, err := p.dest.Write(buf[:n]); err != nil {
			return err
		}

		// update ETA
		p.uploadedSize += int64(n)
		done := float64(p.uploadedSize) / float64(p.file.Size)
		elapsed := time.Since(p.startedAt).Seconds()

		p.mutex.Lock()
		p.ETA = time.Second*time.Duration(elapsed/done) - time.Second*time.Duration(elapsed)
		p.mutex.Unlock()
	}

	return nil
}

// GetETA of upload end
func (p *PusherMulch) GetETA() time.Duration {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.ETA
}

// GetError of upload (if any)
func (p *PusherMulch) GetError() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.Error
}

// IsFinished uploading?
func (p *PusherMulch) IsFinished() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.Finished
}
