package pkg

import (
	"os/user"
	"strings"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

// get file path considering user home directory
func ParseFilepath(path string) string {
	if strings.Index(path, "~") != 0 || len(path) != 1 && path[1] != '/' {
		return path
	}

	usr, err := user.Current()
	var newPath string
	if err == nil {
		newPath = strings.Replace(path, "~", usr.HomeDir, 1)
	} else {
		newPath = strings.Replace(path, "~", ".", 1)
		log.Errorf("Failed to get current home directory")
	}
	log.V(1).Infof("Parse %v from path %v", newPath, path)

	return newPath
}
