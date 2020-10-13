package main

import (
	"io"
	"os"
	"strconv"
	"time"

	"github.com/ncw/swift"
)

type tomlSwiftConfig struct {
	UserName  string `toml:"username"`
	APIKey    string `toml:"api_key"`
	AuthURL   string `toml:"auth_url"`
	Domain    string `toml:"domain"`
	Region    string `toml:"region"`
	Container string `toml:"container"`
	ChunkSize string `toml:"chunk_size"`
}

// SwiftConfig stores final settings for Swift
type SwiftConfig struct {
	UserName   string
	APIKey     string
	AuthURL    string
	Domain     string
	Region     string
	Container  string
	ChunckSize int64
}

// Swift host connection and configuration
type Swift struct {
	Config *SwiftConfig
	Conn   swift.Connection
}

// NewSwift will create a new Swift instance from config
func NewSwift(config *SwiftConfig) (*Swift, error) {
	swift := &Swift{
		Config: config,
	}
	err := swift.connect()
	if err != nil {
		return nil, err
	}
	return swift, nil
}

// NewSwiftConfigFromToml will check tomlSwiftConfig and create a SwiftConfig
func NewSwiftConfigFromToml(tConfig *tomlSwiftConfig) (*SwiftConfig, error) {
	// TODO: write the function ;)
	// convert chunksize from string to int64
	return nil, nil
}

// Connect and authenticate to the Swift API
func (s *Swift) connect() error {
	s.Conn = swift.Connection{
		UserName: s.Config.UserName,
		ApiKey:   s.Config.APIKey,
		AuthUrl:  s.Config.AuthURL,
		Domain:   s.Config.Domain,
		Region:   s.Config.Region,
	}

	err := s.Conn.Authenticate()
	if err != nil {
		return err
	}
	return nil
}

// Upload a local file to Swift provider
func (s *Swift) Upload(file *File, deleteAfter time.Duration) error {
	source, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer source.Close()

	deleteAfterSeconds := int(deleteAfter / time.Second)

	dest, err := s.Conn.DynamicLargeObjectCreate(&swift.LargeObjectOpts{
		Container:  s.Config.Container,
		ObjectName: file.Path,
		ChunkSize:  s.Config.ChunckSize,
		Headers: swift.Headers{
			"X-Delete-After": strconv.Itoa(deleteAfterSeconds),
		},
	})
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	if err != nil {
		return err
	}
	return nil
}
