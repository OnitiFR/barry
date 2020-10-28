// +build dev

package server

import "time"

// FileStorageName is the name of the storage sub-directory
// where local files are stored
const FileStorageName = "files"

// LogHistorySize is the maximum number of messages in app log history
const LogHistorySize = 5000

// RetryDelay is used when an upload/move failed
const RetryDelay = 1 * time.Minute

// QueueScanDelay is the delay between consecutive queue scans
const QueueScanDelay = 3 * time.Second

// QueueStableDelay determine how long a file should stay the same (mtime+size)
// to be considered stable.
const QueueStableDelay = 6 * time.Second

// KeepAliveDelayDays is the number of days between each keep-alive/stats report
const KeepAliveDelayDays = 0

// CheckExpireEvery is the delay between each expire task
const CheckExpireEvery = 1 * time.Minute

// ProjectDefaultBackupEvery is the approximate delay between each backup
// of a projet (used by no-bockup alerts)
const ProjectDefaultBackupEvery = 24 * time.Hour

// NoBackupAlertSchedule is delay between each NoBackupAlert check
const NoBackupAlertSchedule = 1 * time.Hour