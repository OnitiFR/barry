package main

import (
	"fmt"
	"os"
)

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
