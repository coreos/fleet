package unit

import (
	"strings"
)

// This encoding should move to go-systemd.
// See https://github.com/coreos/go-systemd/issues/13
func serializeDbusPath(path string) string {
	path = strings.Replace(path, ".", "_2e", -1)
	path = strings.Replace(path, "-", "_2d", -1)
	path = strings.Replace(path, "@", "_40", -1)
	return path
}
