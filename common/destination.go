package common

// APIDestinationEntries is a list of entries for "get destinations" command
type APIDestinationEntries []APIDestinationEntry

// APIDestinationEntry is an entry for a destination
type APIDestinationEntry struct {
	Name string
	Type string
}
