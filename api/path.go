package api

import (
	"path"
	"strings"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"
)

func isCollectionPath(base, p string) bool {
	return p == base
}

func isItemPath(base, p string) (item string, matched bool) {
	if strings.HasSuffix(p, "/") {
		return
	}

	var err error
	matched, err = path.Match(path.Join(base, "*"), p)
	// err will only be non-nil in the event that our pattern is bad, not due
	// to user-provided data
	if err != nil {
		log.Error("Failed to determine if %q is an item path: %v", p, err)
		matched = false
	} else if matched {
		item = path.Base(p)
	}

	return
}
