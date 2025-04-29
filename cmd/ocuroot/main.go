package main

import (
	"os"

	"github.com/ocuroot/ocuroot/client/commands"
)

func main() {
	if err := commands.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
