package server

import (
	"errors"
	"fmt"
	"math/rand"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/c2h5oh/datasize"
)

// AppConfig describes the general configuration of an App
type AppConfig struct {
	QueuePath           string
	LocalStoragePath    string
	TempPath            string
	NumUploaders        int
	NumEncrypters       int
	SelfBackupContainer string
	Expiration          *ExpirationConfig
	Swift               *SwiftConfig
	API                 *APIConfig
	Containers          []*Container
	Pushers             map[string]*PusherConfig
	Encryptions         map[string]*EncryptionConfig
	configPath          string
}

// APIConfig describes API server configuration
type APIConfig struct {
	Listen string
}

type tomlAppConfig struct {
	QueuePath           string `toml:"queue_path"`
	LocalStoragePath    string `toml:"local_storage_path"`
	TempPath            string `toml:"temp_path"`
	NumUploaders        int    `toml:"num_uploaders"`
	NumEncrypters       int    `toml:"num_encrypters"`
	SelfBackupContainer string `toml:"self_backup_container"`
	Expiration          *tomlExpiration
	Swift               *tomlSwiftConfig
	API                 *tomlAPIConfig
	Containers          []*tomlContainer       `toml:"container"`
	PushDestinations    []*tomlPushDestination `toml:"push_destination"`
	Encryptions         []*tomlEncryption      `toml:"encryption"`
}

type tomlAPIConfig struct {
	Listen string
}

// NewAppConfigFromTomlFile return a AppConfig using a TOML file in configPath
func NewAppConfigFromTomlFile(configPath string, autogenKey bool, rand *rand.Rand) (*AppConfig, error) {
	filename := path.Clean(configPath + "/barry.toml")

	appConfig := &AppConfig{
		configPath: configPath,
	}

	// defaults (if not in the file)
	tConfig := &tomlAppConfig{
		QueuePath:        "var/queue",
		LocalStoragePath: "var/storage",
		NumUploaders:     2,
		NumEncrypters:    2,
		Expiration: &tomlExpiration{
			Local:  []string{"keep 30 days"},
			Remote: []string{"keep 30 days", "keep 90 days every 7 files"},
		},
		Swift: &tomlSwiftConfig{
			Domain:    "Default",
			ChunkSize: 512 * datasize.MB,
		},
		API: &tomlAPIConfig{
			Listen: ":8787",
		},
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

	if isDir, err := IsDir(tConfig.LocalStoragePath); !isDir {
		return nil, err
	}
	appConfig.LocalStoragePath = filepath.Clean(tConfig.LocalStoragePath)

	// crude check that path1 and path2 are not the same
	aPath1, err := filepath.Abs(appConfig.QueuePath)
	if err != nil {
		return nil, err
	}

	aPath2, err := filepath.Abs(appConfig.LocalStoragePath)
	if err != nil {
		return nil, err
	}

	if aPath1 == aPath2 {
		return nil, errors.New("queue_path and local_storage_path can't be the same")
	}

	sameDevice, err := AreDirsOnSameDevice(appConfig.QueuePath, appConfig.LocalStoragePath)
	if err != nil {
		return nil, err
	}
	if !sameDevice {
		return nil, fmt.Errorf("'%s' and '%s' must be on the same disk/device/partition", appConfig.QueuePath, appConfig.LocalStoragePath)
	}

	appConfig.TempPath = tConfig.TempPath

	if tConfig.NumUploaders < 1 {
		return nil, errors.New("at least one uploader is needed (num_uploaders setting)")
	}
	appConfig.NumUploaders = tConfig.NumUploaders

	if tConfig.NumEncrypters < 1 {
		return nil, errors.New("at least one encrypter is needed (num_encrypters setting)")
	}
	appConfig.NumEncrypters = tConfig.NumEncrypters

	appConfig.SelfBackupContainer = tConfig.SelfBackupContainer

	appConfig.Expiration, err = NewExpirationConfigFromToml(tConfig.Expiration)
	if err != nil {
		return nil, err
	}

	// API server configuration
	appConfig.API = &APIConfig{}
	partsL := strings.Split(tConfig.API.Listen, ":")

	if len(partsL) != 2 {
		return nil, fmt.Errorf("listen: '%s': wrong format (ex: ':8787')", tConfig.API.Listen)
	}
	_, err = strconv.Atoi(partsL[1])

	if err != nil {
		return nil, fmt.Errorf("listen: '%s': wrong port number", tConfig.API.Listen)
	}

	appConfig.API.Listen = tConfig.API.Listen

	// spew.Dump(appConfig.Expiration)

	appConfig.Swift, err = NewSwiftConfigFromToml(tConfig.Swift)
	if err != nil {
		return nil, err
	}

	appConfig.Containers, err = NewContainersConfigFromToml(tConfig.Containers)
	if err != nil {
		return nil, err
	}

	appConfig.Pushers, err = NewPushersConfigFromToml(tConfig.PushDestinations)
	if err != nil {
		return nil, err
	}

	appConfig.Encryptions, err = NewEncryptionsConfigFromToml(tConfig.Encryptions, autogenKey, rand, configPath)
	if err != nil {
		return nil, err
	}

	return appConfig, nil
}
