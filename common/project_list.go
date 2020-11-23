package common

// APIProjectListEntries is a list of project entries
type APIProjectListEntries []APIProjectListEntry

// APIProjectListEntry is a project entry
type APIProjectListEntry struct {
	Path             string
	FileCountCurrent int
	SizeCountCurrent int64
	CostCurrent      float64
	Archived         bool
}
