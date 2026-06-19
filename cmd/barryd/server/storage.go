package server

import (
	"fmt"
	"io"
	"time"
)

// Storage manages every named storage connection and routes operations to
// the right Backend, based on the container a file lives in. It implements
// the Backend interface itself (routing by container).
type Storage struct {
	backends         map[string]Backend // by connection name
	containerBackend map[string]Backend // by container name
}

// NewStorage authenticates every [[storage]] connection and builds the
// container -> backend routing map.
func NewStorage(config *AppConfig) (*Storage, error) {
	s := &Storage{
		backends:         make(map[string]Backend),
		containerBackend: make(map[string]Backend),
	}

	// explicit segment container names, declared on [[upload_container]]
	segmentOverrides := make(map[string]string)
	for _, container := range config.Containers {
		if container.SegmentContainer != "" {
			segmentOverrides[container.Name] = container.SegmentContainer
		}
	}

	for _, sc := range config.Storages {
		var backend Backend
		var err error

		switch sc.Type {
		case StorageTypeSwift:
			backend, err = NewSwift(sc.Swift, config.QueuePath, segmentOverrides)
		default:
			return nil, fmt.Errorf("storage '%s': unknown type '%s'", sc.Name, sc.Type)
		}
		if err != nil {
			return nil, fmt.Errorf("storage '%s': %s", sc.Name, err)
		}

		s.backends[sc.Name] = backend
		for _, container := range sc.Containers {
			s.containerBackend[container] = backend
		}
	}

	return s, nil
}

// backendForContainer resolves the backend handling a given container
func (s *Storage) backendForContainer(container string) (Backend, error) {
	backend, ok := s.containerBackend[container]
	if !ok {
		return nil, fmt.Errorf("no storage connection found for container '%s'", container)
	}
	return backend, nil
}

// CheckContainer returns nil if the container is usable. checkSegments
// requires the segment container to exist too (upload targets only).
func (s *Storage) CheckContainer(name string, checkSegments bool) error {
	backend, err := s.backendForContainer(name)
	if err != nil {
		return err
	}
	return backend.CheckContainer(name, checkSegments)
}

// Upload a local file to the backend hosting file.Container
func (s *Storage) Upload(file *File, written *int64) error {
	backend, err := s.backendForContainer(file.Container)
	if err != nil {
		return err
	}
	return backend.Upload(file, written)
}

// Delete a File from the backend hosting file.Container
func (s *Storage) Delete(file *File) error {
	backend, err := s.backendForContainer(file.Container)
	if err != nil {
		return err
	}
	return backend.Delete(file)
}

// ObjectAvailability returns availability state and delay for an object
func (s *Storage) ObjectAvailability(container string, path string) (string, time.Duration, error) {
	backend, err := s.backendForContainer(container)
	if err != nil {
		return "", 0, err
	}
	return backend.ObjectAvailability(container, path)
}

// Unseal a "cold" file, returning availability ETA
func (s *Storage) Unseal(container string, path string) (time.Duration, error) {
	backend, err := s.backendForContainer(container)
	if err != nil {
		return 0, err
	}
	return backend.Unseal(container, path)
}

// ObjectOpen returns a ReadCloser on an (unsealed) object
func (s *Storage) ObjectOpen(container string, path string) (io.ReadCloser, error) {
	backend, err := s.backendForContainer(container)
	if err != nil {
		return nil, err
	}
	return backend.ObjectOpen(container, path)
}

// FilePutContent will create / overwrite a file with a content
func (s *Storage) FilePutContent(container string, path string, content io.Reader) error {
	backend, err := s.backendForContainer(container)
	if err != nil {
		return err
	}
	return backend.FilePutContent(container, path, content)
}

// FileGetContent will read a file to an io.Writer
func (s *Storage) FileGetContent(container string, path string, output io.Writer) error {
	backend, err := s.backendForContainer(container)
	if err != nil {
		return err
	}
	return backend.FileGetContent(container, path, output)
}
