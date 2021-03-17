package server

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

// Possible object availability
const (
	SwiftObjectSealed    = "sealed"
	SwiftObjectUnsealing = "unsealing"
	SwiftObjectUnsealed  = "unsealed"
)

type tomlSwiftConfig struct {
	UserName  string            `toml:"username"`
	APIKey    string            `toml:"api_key"`
	AuthURL   string            `toml:"auth_url"`
	Domain    string            `toml:"domain"`
	Region    string            `toml:"region"`
	ChunkSize datasize.ByteSize `toml:"chunk_size"`
}

// SwiftConfig stores final settings for Swift
type SwiftConfig struct {
	UserName   string
	APIKey     string
	AuthURL    string
	Domain     string
	Region     string
	ChunckSize uint64
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

	if tConfig.ChunkSize < 1*datasize.MB {
		return nil, fmt.Errorf("chuck_size is to small (%s), use at least 1MB", tConfig.ChunkSize)
	}
	config.ChunckSize = tConfig.ChunkSize.Bytes()

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

// CheckContainer will returnb nil if container and container_segments exists
func (s *Swift) CheckContainer(name string) error {
	_, _, err := s.Conn.Container(name)
	if err != nil {
		return fmt.Errorf("container '%s' does not exists", name)
	}

	segmentsContainer := name + "_segments"
	_, _, err = s.Conn.Container(segmentsContainer)
	if err != nil {
		return fmt.Errorf("you must create container '%s' manually, a different pricing may be used if created via the API with default policy", segmentsContainer)
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

	// Currently, with Openstack object expiration + ncw/swift, only the
	// manifest will expire, not the segments. We now schedule deletion on
	// our side.
	// expireDuration := file.ExpireRemote.Sub(time.Now())
	// deleteAfterSeconds := int(expireDuration / time.Second)

	dest, err := s.Conn.DynamicLargeObjectCreate(&swift.LargeObjectOpts{
		Container:  file.Container,
		ObjectName: file.Path,
		ChunkSize:  int64(s.Config.Swift.ChunckSize),
		// Headers: swift.Headers{
		// 	"X-Delete-After": strconv.Itoa(deleteAfterSeconds),
		// },
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

// Delete a File
func (s *Swift) Delete(file *File) error {
	err := s.Conn.DynamicLargeObjectDelete(file.Container, file.Path)
	if err != nil {
		return err
	}
	return nil
}

// GetObjetAvailability returns availability, explained with two values:
// - state (sealed, unsealing, unsealed)
// - delay in seconds (0 meaning that is file is ready to be downloaded)
func (s *Swift) GetObjetAvailability(container string, path string) (string, time.Duration, error) {
	_, headers, err := s.Conn.Object(container, path)
	if err != nil {
		return "", 0, err
	}

	state, stateExists := headers["X-Ovh-Retrieval-State"]
	if !stateExists {
		// let's check that all chunks are available, with some providers
		// it can take a few seconds
		file, headers, err := s.Conn.ObjectOpen(container, path, false, nil)
		if err != nil {
			return "", 0, err
		}
		size, err := file.Length()
		if err != nil {
			return "", 0, err
		}

		_, isDLO := headers["X-Object-Manifest"]
		if isDLO && size == 0 {
			return SwiftObjectUnsealing, 10 * time.Second, nil // wait a bit
		}
		return SwiftObjectUnsealed, 0, nil
	}

	switch state {
	case SwiftObjectSealed:
		return state, 0, nil
	case SwiftObjectUnsealing:
		delay, delayExists := headers["X-Ovh-Retrieval-Delay"]
		if !delayExists {
			return "", 0, fmt.Errorf("can't find X-Ovh-Retrieval-Delay for unsealing path %s", path)
		}
		secs, err := strconv.Atoi(delay)
		if err != nil {
			return "", 0, fmt.Errorf("can't convert '%s' to seconds for path %s", delay, path)
		}
		d := time.Duration(secs) * time.Second
		return state, time.Duration(d), nil
	case SwiftObjectUnsealed:
		return state, 0, nil
	default:
		return state, 0, fmt.Errorf("unknown state '%s' for path %s", state, path)
	}
}

// Unseal a "cold" file, return availability ETA
func (s *Swift) Unseal(container string, path string) (time.Duration, error) {
	// fire "unseal" action or open the file if available
	file, _, err := s.Conn.ObjectOpen(container, path, false, nil)

	if err == nil {
		// was not sealed
		file.Close()
		return 0, nil
	}

	if err == swift.ObjectNotFound {
		return 0, err
	}

	// TooManyRequests = sealed
	if err == swift.TooManyRequests {
		state, delay, err := s.GetObjetAvailability(container, path)
		if err != nil {
			return 0, err
		}
		switch state {
		case SwiftObjectSealed:
			return 0, fmt.Errorf("unable to unseal path %s", path)
		case SwiftObjectUnsealing:
			return delay, nil
		case SwiftObjectUnsealed:
			// immediate unseal? (never seen in the wild :)
			return 0, nil
		default:
			return 0, fmt.Errorf("unknown state '%s' for path %s", state, path)
		}
	}

	// any other ObjectOpen error
	return 0, err
}

// ObjectOpen a Swift Object, returning a ReadCloser
func (s *Swift) ObjectOpen(container string, path string) (io.ReadCloser, error) {
	availability, _, err := s.GetObjetAvailability(container, path)
	if err != nil {
		return nil, err
	}

	if availability != SwiftObjectUnsealed {
		return nil, errors.New("file is not unsealed")
	}

	file, _, err := s.Conn.ObjectOpen(container, path, false, nil)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// FilePutContent will create / overwrite a file with a content
func (s *Swift) FilePutContent(container string, path string, content io.Reader) error {
	dest, err := s.Conn.ObjectCreate(container, path, false, "", "", nil)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, content)
	if err != nil {
		return err
	}

	return nil
}

// FileGetContent will read a file to io.Writer
func (s *Swift) FileGetContent(container string, path string, output io.Writer) error {
	source, err := s.ObjectOpen(container, path)
	if err != nil {
		return err
	}
	defer source.Close()

	_, err = io.Copy(output, source)
	if err != nil {
		return err
	}

	return nil
}
