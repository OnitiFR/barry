package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/OnitiFR/barry/cmd/barryd/server"
	"github.com/OnitiFR/barry/common"
)

var configPath = flag.String("path", "./etc/", "configuration path")
var configTrace = flag.Bool("trace", false, "show trace messages (debug)")
var configPretty = flag.Bool("pretty", false, "show pretty messages")
var configVersion = flag.Bool("version", false, "show version")

//var configRestore = flag.Bool("restore", false, "restore databases (emergency)")

func main() {
	flag.Parse()

	if *configVersion == true {
		fmt.Println(common.ServerVersion)
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

	AddRoutes(app)
	app.Run()
}
