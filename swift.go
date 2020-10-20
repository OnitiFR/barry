package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/ncw/swift"
)

type tomlSwiftConfig struct {
	UserName  string            `toml:"username"`
	APIKey    string            `toml:"api_key"`
	AuthURL   string            `toml:"auth_url"`
	Domain    string            `toml:"domain"`
	Region    string            `toml:"region"`
	Container string            `toml:"container"`
	ChunkSize datasize.ByteSize `toml:"chunk_size"`
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
	Config *AppConfig
	Conn   swift.Connection
}

// NewSwift will create a new Swift instance from config
func NewSwift(config *AppConfig) (*Swift, error) {
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
	config := &SwiftConfig{}

	if tConfig.UserName == "" {
		return nil, errors.New("swift username setting cannot be empty")
	}
	config.UserName = tConfig.UserName

	if tConfig.APIKey == "" {
		return nil, errors.New("swift api_key setting cannot be empty")
	}
	config.APIKey = tConfig.APIKey

	if tConfig.AuthURL == "" {
		return nil, errors.New("swift auth_url setting cannot be empty")
	}
	config.AuthURL = tConfig.AuthURL

	if tConfig.Domain == "" {
		return nil, errors.New("swift domain setting cannot be empty")
	}
	config.Domain = tConfig.Domain

	if tConfig.Region == "" {
		return nil, errors.New("swift region setting cannot be empty")
	}
	config.Region = tConfig.Region

	if tConfig.Container == "" {
		return nil, errors.New("swift container setting cannot be empty")
	}
	config.Container = tConfig.Container

	if tConfig.ChunkSize < 1*datasize.MB {
		return nil, fmt.Errorf("chuck_size is to small (%s), use at least 1MB", tConfig.ChunkSize)
	}

	return config, nil
}

// Connect and authenticate to the Swift API
func (s *Swift) connect() error {
	s.Conn = swift.Connection{
		UserName: s.Config.Swift.UserName,
		ApiKey:   s.Config.Swift.APIKey,
		AuthUrl:  s.Config.Swift.AuthURL,
		Domain:   s.Config.Swift.Domain,
		Region:   s.Config.Swift.Region,
	}

	err := s.Conn.Authenticate()
	if err != nil {
		return err
	}
	return nil
}

// Upload a local file to Swift provider
func (s *Swift) Upload(file *File) error {
	sourcePath := path.Clean(s.Config.QueuePath + "/" + file.Path)
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	expireDuration := file.ExpireRemote.Sub(time.Now())
	deleteAfterSeconds := int(expireDuration / time.Second)

	dest, err := s.Conn.DynamicLargeObjectCreate(&swift.LargeObjectOpts{
		Container:  s.Config.Swift.Container,
		ObjectName: file.Path,
		ChunkSize:  s.Config.Swift.ChunckSize,
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