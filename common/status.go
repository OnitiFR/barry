package common

import "time"

// APIStatus describes server status
type APIStatus struct {
	Version          string
	StartTime        time.Time
	ProjectCount     int
	FileCount        int
	TotalFileSize    int64   `format:"size"`
	TotalFileCost    float64 `format:"money"`
	UploadQueueSize  int
	EncryptQueueSize int
	Uploaders        []string `format:"ignore"`
	Encrypters       []string `format:"ignore"`
}
