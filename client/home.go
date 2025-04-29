package client

import (
	"os"
	"os/user"
	"path/filepath"
)

// HomeDir returns the Ocuroot home directory
// By default this is ~/.ocuroot, but can be overriden by setting the OCUROOT_HOME
// environment variable
func HomeDir() string {
	if os.Getenv("OCUROOT_HOME") != "" {
		return os.Getenv("OCUROOT_HOME")
	}

	u, err := user.Current()
	if err != nil {
		return ""
	}
	return filepath.Join(u.HomeDir, ".ocuroot")
}
