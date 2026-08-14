// Package main is the GoModel menu-bar app entry point. It runs the gateway
// in the background and exposes a status-bar icon with service status, an
// Open Dashboard action, config reload, and Quit. Build the macOS release
// with `make app`; the resulting GoModel.app needs no command line.
package main

import (
	"os"

	"github.com/enterpilot/gomodel/internal/tray"
)

func main() {
	if err := tray.Run(); err != nil {
		os.Exit(1)
	}
}
