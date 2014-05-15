package functional

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/util"
	"github.com/coreos/fleet/version"
)

func TestClientVersionFlag(t *testing.T) {
	stdout, _, err := util.RunFleetctl("--version")
	if err != nil {
		t.Fatalf("Unexpected error while executing fleetctl: %v", err)
	}

	if strings.TrimSpace(stdout) != fmt.Sprintf("fleetctl version %s", version.Version) {
		t.Fatalf("Received unexpected output for `fleetctl --version`: '%s'", stdout)
	}
}

func TestClientVersionHelpOutput(t *testing.T) {
	stdout, _, err := util.RunFleetctl()
	if err != nil {
		t.Fatalf("Unexpected error while executing fleetctl: %v", err)
	}

	if !strings.Contains(stdout, fmt.Sprintf("%s", version.Version)) {
		t.Fatalf("Could not find expected version string (%s) in help output:\n%s", version.Version, stdout)
	}
}
