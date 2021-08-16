package common

import "time"

// Pusher status (for download)
const (
	APIPushStatusPushing = "pushing"
	APIPushStatusSuccess = "success"
	APIPushStatusError   = "error"
)

// APIPushStatus is the status of a pusher
type APIPushStatus struct {
	Status string
	ETA    time.Duration
	Error  string
}
