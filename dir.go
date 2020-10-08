package main

import (
	"fmt"
	"os"
)

// CreateDirIfNeeded will create the directory if it does not exists
func CreateDirIfNeeded(path string) error {
	stat, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(path, os.ModePerm)
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}

	if !stat.Mode().IsDir() {
		return fmt.Errorf("is not a directory '%s' (and it should be)", path)
	}

	return nil
}

// IsDir return true if path exists and is a directory
func IsDir(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if !stat.Mode().IsDir() {
		return false, fmt.Errorf("'%s' in not a directory", path)
	}
	return true, nil
}
