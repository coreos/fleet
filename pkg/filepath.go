package pkg

import (
	"os/user"
	"path/filepath"
	"strings"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

// ParseFilepath expand ~ and ~user constructions.
// If user or its home directory is unknown, do nothing.
func ParseFilepath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	i := strings.Index(path, "/")
	if i < 0 {
		i = len(path)
	}
	var usr *user.User
	var err error
	if i == 1 {
		usr, err = user.Current()
		if err != nil {
			log.V(1).Infof("Failed to get current home directory: %v", err)
			return path
		}
	} else {
		usr, err = user.Lookup(path[1:i])
		if err != nil {
			log.V(1).Infof("Failed to get %v's home directory: %v", path[1:i], err)
			return path
		}
	}
	newPath := filepath.Join(usr.HomeDir, path[i:])
	log.V(2).Infof("Parse %v from path %v", newPath, path)
	return newPath
}
