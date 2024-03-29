package server

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"sync"
	"time"
)

// ProjectDatabase is a Project database holder
type ProjectDatabase struct {
	filename                  string
	localStoragePath          string
	projects                  ProjectMap
	defaultExpiration         *ExpirationConfig
	remoteExpirationOverrides map[string]ExpirationResult
	log                       *Log
	mutex                     sync.Mutex
	alertSender               *AlertSender
	deleteLocalFunc           ProjectDBDeleteLocalFunc
	deleteRemoteFunc          ProjectDBDeleteRemoteFunc
	noBackupAlertFunc         ProjectDBNoBackupAlertFunc
}

// ProjectDBStats hosts stats about the projects and files
type ProjectDBStats struct {
	ProjectCount int
	FileCount    int
	TotalSize    int64
	TotalCost    float64
}

// ProjectDBDeleteLocalFunc is called when a local file expires (as a goroutine)
type ProjectDBDeleteLocalFunc func(file *File, filePath string)

// ProjectDBDeleteRemoteFunc is called when a remote file expirtes (as a goroutine)
type ProjectDBDeleteRemoteFunc func(file *File)

// ProjectDBNoBackupAlertFunc is called when a backup is missing for a project
type ProjectDBNoBackupAlertFunc func(projects []*Project)

// NewProjectDatabase allocates a new ProjectDatabase
func NewProjectDatabase(
	filename string,
	localStoragePath string,
	defaultExpiration *ExpirationConfig,
	alertSender *AlertSender,
	deleteLocalFunc ProjectDBDeleteLocalFunc,
	deleteRemoteFunc ProjectDBDeleteRemoteFunc,
	noBackupAlertFunc ProjectDBNoBackupAlertFunc,
	log *Log,
) (*ProjectDatabase, error) {
	db := &ProjectDatabase{
		filename:                  filename,
		localStoragePath:          localStoragePath,
		projects:                  make(ProjectMap),
		defaultExpiration:         defaultExpiration,
		remoteExpirationOverrides: make(map[string]ExpirationResult),
		deleteLocalFunc:           deleteLocalFunc,
		deleteRemoteFunc:          deleteRemoteFunc,
		noBackupAlertFunc:         noBackupAlertFunc,
		log:                       log,
		alertSender:               alertSender,
	}
	// if the file exists, load it
	if _, err := os.Stat(db.filename); err == nil {
		err = db.load()
		if err != nil {
			return nil, err
		}
	}

	err := db.upgrade()
	if err != nil {
		return nil, err
	}

	err = db.updateExpirations()
	if err != nil {
		return nil, err
	}

	// save the file to check if it's writable
	// (no mutex lock, we're still in the single main thread)
	err = db.save()
	if err != nil {
		return nil, err
	}

	return db, nil
}

// update default expiration
// update default alert setting, too?
func (db *ProjectDatabase) updateExpirations() error {
	for _, project := range db.projects {
		if !project.LocalExpiration.Custom {
			project.LocalExpiration.Lines = db.defaultExpiration.Local.Lines
		}
	}
	for _, project := range db.projects {
		if !project.RemoteExpiration.Custom {
			project.RemoteExpiration.Lines = db.defaultExpiration.Remote.Lines
		}
	}
	return nil
}

// upgrade projects schema version
func (db *ProjectDatabase) upgrade() error {
	for _, project := range db.projects {
		err := project.upgrade()
		if err != nil {
			return err
		}
	}
	return nil
}

// you should lock the mutex before calling save() & load()
// TODO: add a cooldown, massive queuing stress this function a lot
func (db *ProjectDatabase) save() error {
	f, err := os.Create(db.filename)
	if err != nil {
		return err
	}
	defer f.Close()

	db.log.Trace(MsgGlob, "saving ProjectDatabase")

	enc := json.NewEncoder(f)
	err = enc.Encode(&db.projects)
	if err != nil {
		return err
	}

	return nil
}

func (db *ProjectDatabase) load() error {
	f, err := os.Open(db.filename)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	err = dec.Decode(&db.projects)
	if err != nil {
		return fmt.Errorf("decoding %s: %s", db.filename, err)
	}
	return nil
}

// Save the database to the disk (public mutex-protected version of save())
func (db *ProjectDatabase) Save() error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	return db.save()
}

// SaveToWriter will save the database to a writer (mutex-protected)
func (db *ProjectDatabase) SaveToWriter(writer io.Writer) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	enc := json.NewEncoder(writer)
	err := enc.Encode(&db.projects)
	if err != nil {
		return err
	}
	return nil
}

// GetNames returns all projects names, sorted
func (db *ProjectDatabase) GetNames() []string {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	keys := make([]string, len(db.projects))
	i := 0
	for key := range db.projects {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	return keys
}

// GetByName returns a project, using its name
// Warning: do not mutate returned project, it's not thread safe
func (db *ProjectDatabase) GetByName(name string) (*Project, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	entry, exists := db.projects[name]
	if !exists {
		return nil, fmt.Errorf("project %s not found in database", name)
	}
	return entry, nil

}

// GetFilenames returns all filenames of a project, sorted by mtime (newer last)
func (db *ProjectDatabase) GetFilenames(projectName string) ([]string, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	project, exists := db.projects[projectName]
	if !exists {
		return nil, fmt.Errorf("project '%s' not found", projectName)
	}

	// create a slice from the map
	slice := NewFileMtimeSort(project.Files)

	// sort by mtime
	sort.Sort(slice)

	// extract only filenames
	names := make([]string, 0, len(slice))
	for _, file := range slice {
		names = append(names, file.Filename)
	}

	return names, nil
}

// FindFile finds a file based on projectName and its fileName
func (db *ProjectDatabase) FindFile(projectName string, fileName string) *File {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	project, pExists := db.projects[projectName]
	if !pExists {
		return nil
	}

	file, fExists := project.Files[fileName]
	if !fExists {
		return nil
	}
	return file
}

// FileExists returns true if the file exists in the project
func (db *ProjectDatabase) FileExists(projectName string, fileName string) bool {
	return db.FindFile(projectName, fileName) != nil
}

// FindOrCreateProject will return an existing project or create a new one if needed
func (db *ProjectDatabase) FindOrCreateProject(projectName string) (*Project, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	project, projectExists := db.projects[projectName]

	if !projectExists {
		project = NewProject(projectName, db.defaultExpiration)
		db.projects[projectName] = project

		err := db.save()
		if err != nil {
			return nil, err
		}
	}

	return project, nil
}

// AddFile will add a file to the database to a specific project
func (db *ProjectDatabase) AddFile(projectName string, file *File) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	project, projectExists := db.projects[projectName]
	if !projectExists {
		return fmt.Errorf("project '%s' does not exists in database, can't add '%s'", projectName, file.Filename)
	}

	_, fileExists := project.Files[file.Filename]
	if fileExists {
		return fmt.Errorf("file '%s' already exists in database for project '%s", file.Filename, projectName)
	}

	if project.Archived {
		project.Archived = false
		db.alertSender.Send(&Alert{
			Type:    AlertTypeBad,
			Subject: "Warning",
			Content: fmt.Sprintf("project '%s' was archived, but a new backup appeared so it's now unarchived", projectName),
		})
	}

	project.Files[file.Filename] = file
	project.FileCount++
	project.SizeCount += file.Size
	project.CostCount += file.Cost

	err := db.save()
	if err != nil {
		return err
	}

	db.log.Infof(projectName, "%s/%s added to ProjectDatabase", projectName, file.Filename)
	return nil
}

// GetProjectNextExpiration return next (= for next file) expiration values
func (db *ProjectDatabase) GetProjectNextExpiration(project *Project, file *File) (ExpirationResult, ExpirationResult, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	modTime := file.ModTime

	localExpiration := project.LocalExpiration.GetNext(modTime)
	remoteExpiration := project.RemoteExpiration.GetNext(modTime)

	// check if any override is set for this file (file.Path)
	override, exists := db.remoteExpirationOverrides[file.Path]
	if exists {
		remoteExpiration = override
		if localExpiration.Keep > remoteExpiration.Keep {
			localExpiration = override
		}

		delete(db.remoteExpirationOverrides, file.Path)
	}

	// save, because GetNext have updated project's FileCount
	err := db.save()
	if err != nil {
		return ExpirationResult{}, ExpirationResult{}, err
	}

	return localExpiration, remoteExpiration, nil
}

// ScheduleExpireFiles will schedule file expiration tasks (call as a goroutine)
func (db *ProjectDatabase) ScheduleExpireFiles() {
	for {
		db.expireLocalFiles()
		db.expireRemoteFiles()
		db.expireClean()
		time.Sleep(CheckExpireEvery)
	}
}

// ScheduleReEncryptFiles will schedule file re-encryption tasks (call as a goroutine)
func (db *ProjectDatabase) ScheduleReEncryptFiles(app *App) {
	for {
		err := db.reEncryptFiles(app)
		if err != nil {
			db.log.Errorf(MsgGlob, "error re-encrypting files: %s", err)
		}
		time.Sleep(ReEncryptDelay)
	}
}

// ExpireLocalFiles will scan the database, deleting expired files in local storage
func (db *ProjectDatabase) expireLocalFiles() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	dbModified := false

	for _, project := range db.projects {
		for _, file := range project.Files {
			if time.Now().After(file.ExpireLocal) && !file.ExpiredLocal {
				filePath := path.Clean(db.localStoragePath + "/" + file.Path)

				// deleting file in a subroutine, because it may take quite some time
				// and we're locking the mutex.
				go db.deleteLocalFunc(file, filePath)

				file.ExpiredLocal = true

				// would have be re-encrypted if not expired
				if !file.Encrypted && !file.ReEncryptDate.IsZero() {
					file.Encrypted = true
					file.ReEncryptDate = time.Time{}
				}

				dbModified = true
			}
		}
	}

	if dbModified {
		err := db.save()
		if err != nil {
			db.log.Errorf(MsgGlob, "error saving database: %s", err)
		}
	}
}

// expireRemoteFiles will scan the database, marking expired remote entries
func (db *ProjectDatabase) expireRemoteFiles() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	dbModified := false

	for _, project := range db.projects {
		for _, file := range project.Files {
			if time.Now().After(file.ExpireRemote) && !file.ExpiredRemote {
				// mutex is lock, use a goroutine
				go db.deleteRemoteFunc(file)
				file.ExpiredRemote = true
				dbModified = true
			}
		}
	}

	if dbModified {
		err := db.save()
		if err != nil {
			db.log.Errorf(MsgGlob, "error saving database: %s", err)
		}
	}
}

// expireClean will removed expired entries from database (and empty archived projects)
func (db *ProjectDatabase) expireClean() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	dbModified := false

	for _, project := range db.projects {
		for fileKey, file := range project.Files {
			if file.ExpiredLocal && file.ExpiredRemote {
				// if the file was retrieved, also delete the local copy
				if file.RetrievedPath != "" {
					go db.deleteLocalFunc(file, file.RetrievedPath) // goroutine, because the mutex is locked
				}

				dbModified = true
				delete(project.Files, fileKey)
				db.log.Infof(file.ProjectName(), "file '%s' removed from database", file.Path)
			}
		}
	}

	for projectKey, project := range db.projects {
		if project.Archived && len(project.Files) == 0 {
			dbModified = true
			delete(db.projects, projectKey)
			db.log.Infof(project.Path, "archived project '%s' was empty, removed from database", project.Path)
		}
	}

	if dbModified {
		err := db.save()
		if err != nil {
			db.log.Errorf(MsgGlob, "error saving database: %s", err)
		}
	}
}

func (db *ProjectDatabase) reEncryptFiles(app *App) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	defEncrypt := app.Config.GetDefaultEncryption()
	if defEncrypt == nil {
		return nil
	}

	dbModified := false
	for _, project := range db.projects {
		for _, file := range project.Files {
			if file.Encrypted || file.ReEncryptDate.IsZero() {
				continue
			}

			if time.Now().Before(file.ReEncryptDate) {
				continue
			}

			path, err := file.GetLocalPath(app)
			if err != nil {
				return err
			}

			// i'm not happy with this, re-encryption can take a long time and we're locking the mutex :(
			// (a goroutine is not a solution, because we need to update the database on success only)
			err = defEncrypt.EncryptFileInPlace(path, app.Rand, app.Log)
			if err != nil {
				return err
			}

			dbModified = true
			file.Encrypted = true
			file.ReEncryptDate = time.Time{}
		}
	}

	if dbModified {
		err := db.save()
		if err != nil {
			return fmt.Errorf("error saving database: %c", err)
		}
	}

	return nil
}

// ScheduleNoBackupAlerts will schedule NoBackupAlerts task
func (db *ProjectDatabase) ScheduleNoBackupAlerts() {
	for {
		db.NoBackupAlerts()
		time.Sleep(NoBackupAlertSchedule)
	}
}

// NoBackupAlerts will alert when no backup is found in time for a project
func (db *ProjectDatabase) NoBackupAlerts() {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	now := time.Now()

	dbModified := false
	noBackupProjects := make([]*Project, 0)

	for _, project := range db.projects {
		modTime := project.ModTime()
		if modTime.IsZero() || project.Archived {
			continue
		}
		diff := now.Sub(modTime)
		threshold := project.BackupEvery + (project.BackupEvery / 2)
		if diff > threshold {
			// backup is missing, did we need to send another alert?
			if now.Sub(project.LastNoBackupAlert) > project.BackupEvery {
				db.log.Errorf(project.Path, "missing backup for project '%s' (BackupEvery=%s)", project.Path, project.BackupEvery)
				noBackupProjects = append(noBackupProjects, project)
				project.LastNoBackupAlert = now
				dbModified = true
			}
		}
	}

	if len(noBackupProjects) > 0 {
		db.noBackupAlertFunc(noBackupProjects)
	}

	if dbModified {
		err := db.save()
		if err != nil {
			db.log.Errorf(MsgGlob, "error saving database: %s", err)
		}
	}

}

// Stats return DB content stats using a ProjectDBStats object
func (db *ProjectDatabase) Stats() (ProjectDBStats, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	var res ProjectDBStats

	for _, project := range db.projects {
		res.ProjectCount++
		for _, file := range project.Files {
			res.FileCount++
			res.TotalCost += file.Cost
			res.TotalSize += file.Size
		}
	}
	return res, nil
}

// GetPath of the database
func (db *ProjectDatabase) GetPath() string {
	return db.filename
}

// SetRemoteExpirationOverride force an expiration for a future coming file
func (db *ProjectDatabase) SetRemoteExpirationOverride(filePath string, exp ExpirationResult) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	db.remoteExpirationOverrides[filePath] = exp
}
