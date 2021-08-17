package server

import (
	"errors"
	"fmt"
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

// PusherConfig is a decoded tomlPushDestination config
type PusherConfig struct {
	Name string
	Type string
	URL  string
	Key  string
}

type tomlPushDestination struct {
	Name string
	Type string
	URL  string
	Key  string
}

// NewPushersConfigFromToml will "parse" TOML push destinations
func NewPushersConfigFromToml(tDestinations []*tomlPushDestination) (map[string]*PusherConfig, error) {
	res := make(map[string]*PusherConfig)

	for _, tDestination := range tDestinations {
		if tDestination.Name == "" {
			return nil, errors.New("push_destination must have a 'name' setting")
		}

		_, exists := res[tDestination.Name]
		if exists {
			return nil, fmt.Errorf("duplicate push_destination '%s'", tDestination.Name)
		}

		conf := PusherConfig{
			Name: tDestination.Name,
			Type: tDestination.Type,
		}

		switch tDestination.Type {
		case PusherTypeMulch:
			if tDestination.URL == "" {
				return nil, fmt.Errorf("push_destination %s: 'url' is needed", tDestination.Name)
			}
			conf.URL = tDestination.URL

			if tDestination.Key == "" {
				return nil, fmt.Errorf("push_destination %s: 'key' is needed", tDestination.Name)
			}
			conf.Key = tDestination.Key

		default:
			return nil, fmt.Errorf("type '%s' unsupported (%s)", tDestination.Type, tDestination.Name)
		}

		res[tDestination.Name] = &conf
	}

	return res, nil
}
