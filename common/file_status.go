package common

import "time"

// File status (for download)
const (
	APIFileStatusAvailable  = "available"
	APIFileStatusRetrieving = "retrieving"
	APIFileStatusUnsealing  = "unsealing"
)

// APIFileStatus is the status of a file from barryd PoV
type APIFileStatus struct {
	Status string
	ETA    time.Duration
}
