package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/OnitiFR/barry/cmd/barryd/server"
	"github.com/OnitiFR/barry/common"
)

var configPath = flag.String("path", "./etc/", "configuration path")
var configTrace = flag.Bool("trace", false, "show trace messages (debug)")
var configPretty = flag.Bool("pretty", false, "show pretty messages")
var configVersion = flag.Bool("version", false, "show version")
var configRestore = flag.Bool("restore", false, "restore databases (emergency, will ERASE local projects and keys!)")
var configGenkey = flag.Bool("genkey", false, "generate non-existing encryption keys")

func main() {
	flag.Parse()

	if *configVersion {
		fmt.Println(common.ServerVersion)
		os.Exit(0)
	}

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	config, err := server.NewAppConfigFromTomlFile(*configPath, *configGenkey, rnd)
	if err != nil {
		log.Fatal(err)
	}

	app, err := server.NewApp(config, rnd)
	if err != nil {
		log.Fatal(err)
	}

	err = app.Init(*configTrace, *configPretty)
	if err != nil {
		log.Fatal(err)
	}

	if *configRestore {
		err = app.SelfRestore()
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	AddRoutes(app)
	app.Run()
}
