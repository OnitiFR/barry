package server

import (
	"time"
)

// Pusher is an interface that will host an uploading thread implementation
type Pusher interface {
	GetETA() time.Duration
	GetError() error
	IsFinished() bool
}

// Pusher types
const (
	PusherTypeMulch = "mulch"
)

type PusherConfig struct {
	Name string
	Type string
	URL  string
	Key  string
}
