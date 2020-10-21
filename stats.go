package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/c2h5oh/datasize"
)

// Stats hosts statisctics about the app
type Stats struct {
	LastReportTime time.Time
	FileCount      int
	SizeCount      int64
	mutex          sync.Mutex
}

// NewStats return a new Stats object
func NewStats() *Stats {
	return &Stats{
		LastReportTime: time.Now(),
	}
}

// Inc will increment stats
func (s *Stats) Inc(fileCount int, sizeCount int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.FileCount += fileCount
	s.SizeCount += sizeCount
}

// Report as a string and reset stats
func (s *Stats) Report(intro string) string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	// since := now.Sub(s.LastReportTime)

	size := datasize.ByteSize(s.SizeCount)
	str := fmt.Sprintf("%s: %d file(s) sent for a total of %s", intro, s.FileCount, size)

	s.LastReportTime = now
	s.FileCount = 0
	s.SizeCount = 0
	return str
}
