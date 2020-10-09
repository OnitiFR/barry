package main

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// AppConfig describes the general configuration of an App
type AppConfig struct {
	QueuePath        string
	LocalStoragePath string
	configPath       string
}

type tomlAppConfig struct {
	QueuePath        string `toml:"queue_path"`
	LocalStoragePath string `toml:"local_storage_path"`
}

// NewAppConfigFromTomlFile return a AppConfig using a TOML file in configPath
func NewAppConfigFromTomlFile(configPath string) (*AppConfig, error) {
	filename := path.Clean(configPath + "/barry.toml")

	appConfig := &AppConfig{
		configPath: configPath,
	}

	// defaults (if not in the file)
	tConfig := &tomlAppConfig{
		QueuePath:        "var/queue",
		LocalStoragePath: "var/storage",
	}

	meta, err := toml.DecodeFile(filename, tConfig)
	if err != nil {
		return nil, err
	}

	undecoded := meta.Undecoded()
	for _, param := range undecoded {
		return nil, fmt.Errorf("unknown setting '%s'", param)
	}

	// Start checking settings and fill appConfig

	if tConfig.QueuePath == "" {
		return nil, errors.New("empty queue_path")
	}

	if isDir, err := IsDir(tConfig.QueuePath); !isDir {
		return nil, err
	}

	appConfig.QueuePath = filepath.Clean(tConfig.QueuePath)

	if tConfig.LocalStoragePath == "" {
		return nil, errors.New("empty local_storage_path")
	}

	err = CreateDirIfNeeded(tConfig.LocalStoragePath)
	if err != nil {
		return nil, err
	}

	appConfig.LocalStoragePath = tConfig.LocalStoragePath

	return appConfig, nil
}
