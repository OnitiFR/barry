package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/ncw/swift/v2"
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

// Swift host connection and configuration. It implements the Backend
// interface. QueuePath is the app-level queue path (source of uploads).
type Swift struct {
	Config    *SwiftConfig
	QueuePath string
	Conn      swift.Connection
	// segmentOverrides maps a container name to an explicit segment container
	// name. Empty/missing means the default "<name>_segments" convention.
	segmentOverrides map[string]string
}

// NewSwift will create a new Swift instance from a connection config
func NewSwift(config *SwiftConfig, queuePath string, segmentOverrides map[string]string) (*Swift, error) {
	swift := &Swift{
		Config:           config,
		QueuePath:        queuePath,
		segmentOverrides: segmentOverrides,
	}
	err := swift.connect()
	if err != nil {
		return nil, err
	}

	return swift, nil
}

// segmentContainer returns the segment container name for a data container:
// the explicit override if set, otherwise the default "<name>_segments".
func (s *Swift) segmentContainer(container string) string {
	if seg := s.segmentOverrides[container]; seg != "" {
		return seg
	}
	return container + "_segments"
}

// NewSwiftConfigFromToml will check tomlSwiftConfig and create a SwiftConfig
func NewSwiftConfigFromToml(tConfig *tomlSwiftConfig) (*SwiftConfig, error) {
	config := &SwiftConfig{}

	// defaults (per-connection, since [[storage]] is an array)
	if tConfig.Domain == "" {
		tConfig.Domain = "Default"
	}
	if tConfig.ChunkSize == 0 {
		tConfig.ChunkSize = 512 * datasize.MB
	}

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
		UserName: s.Config.UserName,
		ApiKey:   s.Config.APIKey,
		AuthUrl:  s.Config.AuthURL,
		Domain:   s.Config.Domain,
		Region:   s.Config.Region,
	}

	err := s.Conn.Authenticate(context.Background())
	if err != nil {
		return err
	}
	return nil
}

// CheckContainer will return nil if the container exists. When checkSegments
// is true, the related segments container must exist too (needed for upload).
func (s *Swift) CheckContainer(name string, checkSegments bool) error {
	ctx := context.Background()
	_, _, err := s.Conn.Container(ctx, name)
	if err != nil {
		return fmt.Errorf("container '%s' does not exists", name)
	}

	if !checkSegments {
		return nil
	}

	segmentsContainer := s.segmentContainer(name)
	_, _, err = s.Conn.Container(ctx, segmentsContainer)
	if err != nil {
		return fmt.Errorf("you must create container '%s' manually, a different pricing may be used if created via the API with default policy", segmentsContainer)
	}

	return nil
}

// Upload a local file to Swift provider. If written is not nil, it is
// atomically updated with the number of bytes read from the source file,
// so callers can track upload progress.
func (s *Swift) Upload(file *File, written *int64) error {
	sourcePath := path.Clean(s.QueuePath + "/" + file.Path)
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	var reader io.Reader = source
	if written != nil {
		reader = &progressReader{reader: source, written: written}
	}

	// Currently, with Openstack object expiration + ncw/swift, only the
	// manifest will expire, not the segments. We now schedule deletion on
	// our side.
	// expireDuration := file.ExpireRemote.Sub(time.Now())
	// deleteAfterSeconds := int(expireDuration / time.Second)

	// NoBuffer kills throughput (down to a few KB/s) and a CopyBuffer()
	// make things very unstable with OVH. Back to memory-hungry-mode.

	dest, err := s.Conn.DynamicLargeObjectCreate(context.Background(), &swift.LargeObjectOpts{
		Container:        file.Container,
		ObjectName:       file.Path,
		ChunkSize:        int64(s.Config.ChunckSize),
		SegmentContainer: s.segmentContainer(file.Container),
		// NoBuffer:   true,
		// Headers: swift.Headers{
		// 	"X-Delete-After": strconv.Itoa(deleteAfterSeconds),
		// },
	})
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, reader)
	if err != nil {
		return err
	}
	return nil
}

// Delete a File
func (s *Swift) Delete(file *File) error {
	err := s.Conn.DynamicLargeObjectDelete(context.Background(), file.Container, file.Path)
	if err != nil {
		return err
	}
	return nil
}

// ObjectAvailability returns availability, explained with two values:
// - state (sealed, unsealing, unsealed)
// - delay in seconds (0 meaning that is file is ready to be downloaded)
func (s *Swift) ObjectAvailability(container string, path string) (string, time.Duration, error) {
	ctx := context.Background()
	_, headers, err := s.Conn.Object(ctx, container, path)
	if err != nil {
		return "", 0, err
	}

	state, stateExists := headers["X-Ovh-Retrieval-State"]
	if !stateExists {
		// let's check that all chunks are available, with some providers
		// it can take a few seconds
		file, headers, err := s.Conn.ObjectOpen(ctx, container, path, false, nil)
		if err != nil {
			return "", 0, err
		}
		size, err := file.Length(ctx)
		if err != nil {
			return "", 0, err
		}

		_, isDLO := headers["X-Object-Manifest"]
		if isDLO && size == 0 {
			return ObjectUnsealing, 10 * time.Second, nil // wait a bit
		}
		return ObjectUnsealed, 0, nil
	}

	switch state {
	case ObjectSealed:
		return state, 0, nil
	case ObjectUnsealing:
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
	case ObjectUnsealed:
		return state, 0, nil
	default:
		return state, 0, fmt.Errorf("unknown state '%s' for path %s", state, path)
	}
}

// Unseal a "cold" file, return availability ETA
func (s *Swift) Unseal(container string, path string) (time.Duration, error) {
	// fire "unseal" action or open the file if available
	file, _, err := s.Conn.ObjectOpen(context.Background(), container, path, false, nil)

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
		state, delay, err := s.ObjectAvailability(container, path)
		if err != nil {
			return 0, err
		}
		switch state {
		case ObjectSealed:
			return 0, fmt.Errorf("unable to unseal path %s", path)
		case ObjectUnsealing:
			return delay, nil
		case ObjectUnsealed:
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
	availability, _, err := s.ObjectAvailability(container, path)
	if err != nil {
		return nil, err
	}

	if availability != ObjectUnsealed {
		return nil, errors.New("file is not unsealed")
	}

	file, _, err := s.Conn.ObjectOpen(context.Background(), container, path, false, nil)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// FilePutContent will create / overwrite a file with a content
func (s *Swift) FilePutContent(container string, path string, content io.Reader) error {
	dest, err := s.Conn.ObjectCreate(context.Background(), container, path, false, "", "", nil)
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
