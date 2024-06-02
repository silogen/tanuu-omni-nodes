package main

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/tanuudev/tanuu-omni-nodes/cmd"
	"github.com/tanuudev/tanuu-omni-nodes/cmd/menu"
	"github.com/tanuudev/tanuu-omni-nodes/cmd/utils"
)

func main() {
	utils.Setup()
	log.Info("starting up...")
	// check if command line arguments are passed
	if len(os.Args) > 1 {
		cmd.Execute()
	} else {
		menu.Menu()
	}

}
