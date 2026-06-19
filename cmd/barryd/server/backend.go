package server

import (
	"io"
	"time"
)

// Storage type names (used as 'type' in [[storage]] config)
const (
	StorageTypeSwift = "swift"
)

// Object availability states (backend-neutral). Backends translate their
// own native states (e.g. OVH retrieval state, S3 Glacier restore) to these.
const (
	ObjectSealed    = "sealed"
	ObjectUnsealing = "unsealing"
	ObjectUnsealed  = "unsealed"
)

// Backend is a storage backend (Swift, S3, …) able to store and retrieve
// files in named containers. Swift is the first implementation.
type Backend interface {
	// CheckContainer returns nil if the container is usable
	CheckContainer(name string) error

	// Upload a local file. If written is not nil, it is atomically updated
	// with the number of bytes read from the source file (progress tracking).
	Upload(file *File, written *int64) error

	// Delete a File
	Delete(file *File) error

	// ObjectAvailability returns availability state (sealed, unsealing,
	// unsealed) and a delay (0 meaning ready to download).
	ObjectAvailability(container string, path string) (string, time.Duration, error)

	// Unseal a "cold" file, returning availability ETA
	Unseal(container string, path string) (time.Duration, error)

	// ObjectOpen returns a ReadCloser on an (unsealed) object
	ObjectOpen(container string, path string) (io.ReadCloser, error)

	// FilePutContent will create / overwrite a file with a content
	FilePutContent(container string, path string, content io.Reader) error

	// FileGetContent will read a file to an io.Writer
	FileGetContent(container string, path string, output io.Writer) error
}
