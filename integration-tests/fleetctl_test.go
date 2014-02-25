package integration

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/coreos/fleet/version"
)

func TestVersion(t *testing.T) {
	cmd := exec.Command("../bin/fleetctl", "--version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Received unexpected error: %v", err)
	}

	if strings.TrimSpace(string(output)) != fmt.Sprintf("fleetctl version %s", version.Version) {
		t.Fatalf("Received unexpected output for `fleetctl --version`: '%s'", output)
	}
}
