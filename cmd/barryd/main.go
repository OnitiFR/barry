package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/OnitiFR/barry/cmd/barryd/server"
)

var configPath = flag.String("path", "./etc/", "configuration path")
var configTrace = flag.Bool("trace", false, "show trace messages (debug)")
var configPretty = flag.Bool("pretty", false, "show pretty messages")
var configVersion = flag.Bool("version", false, "show version")

func main() {
	flag.Parse()

	if *configVersion == true {
		fmt.Println(server.Version)
		os.Exit(0)
	}

	config, err := server.NewAppConfigFromTomlFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	app, err := server.NewApp(config)
	if err != nil {
		log.Fatal(err)
	}

	err = app.Init(*configTrace, *configPretty)
	if err != nil {
		log.Fatal(err)
	}

	app.Run()

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
