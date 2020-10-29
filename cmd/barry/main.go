package main

import (
	"os"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/OnitiFR/barry/cmd/barry/topics"
)

func main() {
	client.InitExitMessage()

	err := topics.Execute()

	msg := client.GetExitMessage()
	msg.Display()

	if err != nil {
		os.Exit(1)
	}
}
