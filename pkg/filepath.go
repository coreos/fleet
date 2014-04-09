package pkg

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

// ParseFilepath expands ~ and ~user constructions.
// If user or $HOME is unknown, do nothing.
func ParseFilepath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	i := strings.Index(path, "/")
	if i < 0 {
		i = len(path)
	}
	var home string
	if i == 1 {
		if home = os.Getenv("HOME"); home == "" {
			usr, err := user.Current()
			if err != nil {
				log.V(1).Infof("Failed to get current home directory: %v", err)
				return path
			}
			home = usr.HomeDir
		}
	} else {
		usr, err := user.Lookup(path[1:i])
		if err != nil {
			log.V(1).Infof("Failed to get %v's home directory: %v", path[1:i], err)
			return path
		}
		home = usr.HomeDir
	}
	path = filepath.Join(home, path[i:])
	log.V(2).Infof("Parse out path %v", path)
	return path
}
