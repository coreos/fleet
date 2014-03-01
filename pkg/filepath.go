package pkg

import (
	"os/user"
	"strings"
)

// get file path considering user home directory
func ParseFilepath(path string) string {
	if strings.Index(path, "~") != 0 {
		return path
	}

	usr, err := user.Current()
	if err == nil {
		path = strings.Replace(path, "~", usr.HomeDir, 1)
	}

	return path
}
