package common

import "time"

// APIProjectInfos contains informations about a project
type APIProjectInfos struct {
	FileCountCurrent    int
	SizeCountCurrent    int64   `format:"size"`
	CostCurrent         float64 `format:"money"`
	Archived            bool
	BackupEvery         time.Duration
	NewestModTime       time.Time
	FinalExpiration     time.Time
	LocalExpirationStr  string
	RemoteExpirationStr string
}
