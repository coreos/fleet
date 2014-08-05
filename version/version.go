package version

import (
	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
)

const Version = "0.6.2+git"

var SemVersion semver.Version

func init() {
	sv, err := semver.NewVersion(Version)
	if err != nil {
		panic("bad version string!")
	}
	SemVersion = *sv
}
