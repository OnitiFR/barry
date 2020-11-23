package common

import "time"

// APIFileListEntries is a list of file entries
type APIFileListEntries []APIFileListEntry

// APIFileListEntry is a file entry
type APIFileListEntry struct {
	Filename      string
	ModTime       time.Time
	Size          int64
	ExpireLocal   time.Time // expiration date
	ExpireRemote  time.Time // (same)
	RemoteKeep    time.Duration
	ExpiredLocal  bool
	ExpiredRemote bool
	Container     string
}
