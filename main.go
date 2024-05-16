package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/tanuudev/tanuu-omni-nodes/cmd/menu"
	"github.com/tanuudev/tanuu-omni-nodes/cmd/utils"
)

func main() {
	utils.Setup()
	log.Info("starting up...")
	menu.Menu()

}
