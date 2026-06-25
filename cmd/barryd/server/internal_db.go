package server

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
)

// InternalDB is a small persistent key→value store for barryd's own internal
// state (scalar values generated/computed once, like the health check path).
//
// It is deliberately minimal: values are plain strings, there's no namespacing,
// no typing and no migration. It is NOT meant to replace the structured
// databases (APIKeyDatabase, ProjectDatabase), only to give sparse internal
// scalars a home instead of one bespoke file each.
type InternalDB struct {
	filename string
	mutex    sync.Mutex
	Values   map[string]string
}

// NewInternalDB loads the store from the given file, or creates an empty one if
// it does not exist yet.
func NewInternalDB(filename string, log *Log) (*InternalDB, error) {
	db := &InternalDB{
		filename: filename,
		Values:   make(map[string]string),
	}

	// if the file exists, load it
	if _, err := os.Stat(db.filename); err == nil {
		err = db.load()
		if err != nil {
			return nil, err
		}
		log.Tracef(MsgGlob, "found %d value(s) in internal database %s", len(db.Values), db.filename)
	} else {
		log.Tracef(MsgGlob, "no internal database found, creating a new one (%s)", db.filename)
	}

	// save the file to check if it's writable
	err := db.Save()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (db *InternalDB) load() error {
	f, err := os.Open(db.filename)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	requiredMode, err := strconv.ParseInt("0600", 8, 32)
	if err != nil {
		return err
	}

	if stat.Mode() != os.FileMode(requiredMode) {
		return fmt.Errorf("%s: only the owner should be able to read/write this file (mode 0600)", db.filename)
	}

	dec := json.NewDecoder(f)
	err = dec.Decode(&db.Values)
	if err != nil {
		return fmt.Errorf("decoding %s: %s", db.filename, err)
	}

	return nil
}

// Save the database to disk. The caller must NOT hold db.mutex.
func (db *InternalDB) Save() error {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	return db.saveUnlocked()
}

// saveUnlocked writes the database to disk; the caller must hold db.mutex.
func (db *InternalDB) saveUnlocked() error {
	f, err := os.OpenFile(db.filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return db.saveToWriter(f)
}

func (db *InternalDB) saveToWriter(writer io.Writer) error {
	enc := json.NewEncoder(writer)
	enc.SetIndent("", "  ")
	return enc.Encode(&db.Values)
}

// Get returns the value for the given key, and whether it exists.
func (db *InternalDB) Get(key string) (string, bool) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	val, exists := db.Values[key]
	return val, exists
}

// Set stores a value and persists the database.
func (db *InternalDB) Set(key string, value string) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	db.Values[key] = value
	return db.saveUnlocked()
}

// GetOrSet returns the existing value for the given key. If the key does not
// exist yet, it is generated using gen(), stored, persisted and returned. This
// is the canonical "compute once, then stable" pattern.
func (db *InternalDB) GetOrSet(key string, gen func() string) (string, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if val, exists := db.Values[key]; exists {
		return val, nil
	}

	val := gen()
	db.Values[key] = val
	if err := db.saveUnlocked(); err != nil {
		return "", err
	}
	return val, nil
}

// GetPath returns the path of the database file.
func (db *InternalDB) GetPath() string {
	return db.filename
}
