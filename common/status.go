package common

import "time"

// APIStatus describes server status
type APIStatus struct {
	StartTime     time.Time
	ProjectCount  int
	FileCount     int
	TotalFileSize int64    `format:"size"`
	TotalFileCost float64  `format:"money"`
	Workers       []string `format:"ignore"`
}
