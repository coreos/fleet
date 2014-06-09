package version

import (
	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"
)

const Version = "0.5.0-rc.1"

var SemVersion semver.Version

func init() {
	sv, err := semver.NewVersion(Version)
	if err != nil {
		panic("bad version string!")
	}
	SemVersion = *sv
}
