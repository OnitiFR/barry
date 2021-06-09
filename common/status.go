package common

import "time"

// APIStatus describes server status
type APIStatus struct {
	Version       string
	StartTime     time.Time
	ProjectCount  int
	FileCount     int
	TotalFileSize int64   `format:"size"`
	TotalFileCost float64 `format:"money"`
	QueueSize     int
	Workers       []string `format:"ignore"`
}
