package common

// APIProjectListEntries is a list of projet entries
type APIProjectListEntries []APIProjectListEntry

// APIProjectListEntry is a project entry
type APIProjectListEntry struct {
	Path             string
	FileCountCurrent int
	SizeCountCurrent int64
	CostCurrent      float64
}
