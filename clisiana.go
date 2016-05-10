package main

import (
	"os"

	"github.com/casimir/xdg-go"
)

var config *Config

func main() {
	config = &Config{}
	config.XDGApp = xdg.App{Name: "clisiana"}

	config.MainTextChannel = make(chan string)

	config.CLIApp = commandLineSetup()
	config.CLIApp.Action = run

	config.CLIApp.Run(os.Args)
}
