package client

import "os"

func Command() string {
	if os.Getenv("OCU_COMMAND_OVERRIDE") != "" {
		return os.Getenv("OCU_COMMAND_OVERRIDE")
	}
	return "ocuroot"
}
