package main

import (
	"fmt"
	"log"
	"path"
)

func main() {
	// sourcePath := path.Clean("/home/xfennec/Quiris/Go/src/local/swift///./30-j")
	// sourcePath := path.Clean(".././/30-j/")

	sourcePath := path.Clean("../queue")
	localStoragePath := path.Clean("/home/xfennec/Quiris/Go/src/local/swift/storage")
	dataBaseFilename := path.Clean("../data/projects.db")

	err := CreateDirIfNeeded(localStoragePath)
	if err != nil {
		log.Fatal(err)
	}

	db, err := NewProjectDatabase(dataBaseFilename)
	if err != nil {
		log.Fatal(err)
	}

	waitList := NewWaitList()
	waitList.Dump()

	err = db.Walk(sourcePath)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Save()
	if err != nil {
		log.Fatal(err)
	}

	for _, projectName := range db.GetNames() {
		fmt.Printf("- %s\n", projectName)
		fileNames, err := db.GetFilenames(projectName)
		if err != nil {
			log.Fatal(err)
		}
		for _, fileName := range fileNames {
			fmt.Printf("    - %s\n", fileName)
		}
	}
}
