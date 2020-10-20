package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

var configPath = flag.String("path", "./etc/", "configuration path")
var configTrace = flag.Bool("trace", false, "show trace messages (debug)")
var configPretty = flag.Bool("pretty", false, "show pretty messages")
var configVersion = flag.Bool("version", false, "show version")

func main() {
	flag.Parse()

	if *configVersion == true {
		fmt.Println(Version)
		os.Exit(0)
	}

	config, err := NewAppConfigFromTomlFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	app, err := NewApp(config)
	if err != nil {
		log.Fatal(err)
	}

	err = app.Init(*configTrace, *configPretty)
	if err != nil {
		log.Fatal(err)
	}

	for {
		err = app.WaitList.Scan()
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(1 * time.Second)
	}

	// app.WaitList.Dump()

	// err = app.ProjectDB.Save()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// for _, projectName := range db.GetNames() {
	// 	fmt.Printf("- %s\n", projectName)
	// 	fileNames, err := db.GetFilenames(projectName)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	for _, fileName := range fileNames {
	// 		fmt.Printf("    - %s\n", fileName)
	// 	}
	// }
}
