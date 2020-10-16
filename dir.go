package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// AreDirsOnSameDevice return true of both directories are on the same
// disk/device/partition, so a file can be moved from path1 to path2
// without the need of data copy
func AreDirsOnSameDevice(path1, path2 string) (bool, error) {
	file1 := filepath.Clean(path1 + "/.tmp.delme")
	file2 := filepath.Clean(path2 + "/.tmp.delme")

	// create source file
	f1, err := os.Create(file1)
	if err != nil {
		return false, err
	}
	f1.Close()

	// will fail if the file is successfully moved below ;)
	defer os.Remove(file1)

	// check that we're able to write in path2
	f2, err := os.Create(file2)
	if err != nil {
		return false, err
	}
	f2.Close()

	// OK, let's remove this test
	err = os.Remove(file2)
	if err != nil {
		return false, err
	}

	// os.Rename is not able to move a file between devices, if it fails,
	// it's very likely that it's the issue
	err = os.Rename(file1, file2)
	if err != nil {
		return false, nil
	}

	// remove the moved file
	err = os.Remove(file2)
	if err != nil {
		return false, err
	}

	return true, nil
}
