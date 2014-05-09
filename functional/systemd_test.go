package functional

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/systemd"
	"github.com/coreos/fleet/unit"
)

func TestSystemdUnitFlow(t *testing.T) {
	uDir, err := ioutil.TempDir("", "fleet-")
	if err != nil {
		t.Fatalf("Failed creating tempdir: %v", err)
	}
	defer os.RemoveAll(uDir)

	mgr, err := systemd.NewSystemdManager(uDir)
	if err != nil {
		t.Fatalf("Failed initializing SystemdManager: %v", err)
	}

	units, err := mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if len(units) > 0 {
		t.Fatalf("Expected no units to be returned, got %v", units)
	}

	name := fmt.Sprintf("fleet-unit-%d.service", rand.Int63())
	uf := unit.NewUnit(`[Service]
ExecStart=/usr/bin/sleep 3000
`)
	j := job.NewJob(name, *uf)

	if err := mgr.Load(j.Name, j.Unit); err != nil {
		t.Fatalf("Failed loading job: %v", err)
	}

	units, err = mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if !reflect.DeepEqual([]string{name}, units){
		t.Fatalf("Expected [hello.service], got %v", units)
	}

	mgr.Unload(name)

	units, err = mgr.Units()
	if err != nil {
		t.Fatalf("Failed calling Units(): %v", err)
	}

	if len(units) > 0 {
		t.Fatalf("Expected no units to be returned, got %v", units)
	}
}
