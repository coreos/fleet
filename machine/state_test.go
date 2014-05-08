package machine

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestStackState(t *testing.T) {
	top := MachineState{"c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}, "1", nil}
	bottom := MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, "", nil}
	stacked := stackState(top, bottom)

	if stacked.ID != "c31e44e1-f858-436e-933e-59c642517860" {
		t.Errorf("Unexpected ID value %s", stacked.ID)
	}

	if stacked.PublicIP != "1.2.3.4" {
		t.Errorf("Unexpected PublicIp value %s", stacked.PublicIP)
	}

	if len(stacked.Metadata) != 1 || stacked.Metadata["ping"] != "pong" {
		t.Errorf("Unexpected Metadata %v", stacked.Metadata)
	}

	if stacked.Version != "1" {
		t.Errorf("Unexpected Version value %s", stacked.Version)
	}
}

func TestStackStateEmptyTop(t *testing.T) {
	top := MachineState{}
	bottom := MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, "", nil}
	stacked := stackState(top, bottom)

	if stacked.ID != "595989bb-cbb7-49ce-8726-722d6e157b4e" {
		t.Errorf("Unexpected ID value %s", stacked.ID)
	}

	if stacked.PublicIP != "5.6.7.8" {
		t.Errorf("Unexpected PublicIp value %s", stacked.PublicIP)
	}

	if len(stacked.Metadata) != 1 || stacked.Metadata["foo"] != "bar" {
		t.Errorf("Unexpected Metadata %v", stacked.Metadata)
	}

	if stacked.Version != "" {
		t.Errorf("Unexpected Version value %s", stacked.Version)
	}
}

var shortIDTests = []struct {
	m MachineState
	s string
	l string
}{
	{
		m: MachineState{},
		s: "",
		l: "",
	},
	{
		m: MachineState{
			"595989bb-cbb7-49ce-8726-722d6e157b4e",
			"5.6.7.8",
			map[string]string{"foo": "bar"},
			"",
			nil,
		},
		s: "595989bb",
		l: "595989bb-cbb7-49ce-8726-722d6e157b4e",
	},
	{
		m: MachineState{
			"5959",
			"5.6.7.8",
			map[string]string{"foo": "bar"},
			"",
			nil,
		},
		s: "5959",
		l: "5959",
	},
}

func TestStateShortID(t *testing.T) {
	for i, tt := range shortIDTests {
		if g := tt.m.ShortID(); g != tt.s {
			t.Errorf("#%d: got %q, want %q", i, g, tt.s)
		}
	}
}

func TestStateMatchID(t *testing.T) {
	for i, tt := range shortIDTests {
		if tt.s != "" {
			if ok := tt.m.MatchID(""); ok {
				t.Errorf("#%d: expected %v", i, false)
			}
		}

		if ok := tt.m.MatchID("foobar"); ok {
			t.Errorf("#%d: expected %v", i, false)
		}

		if ok := tt.m.MatchID(tt.l); !ok {
			t.Errorf("#%d: expected %v", i, true)
		}

		if ok := tt.m.MatchID(tt.s); !ok {
			t.Errorf("#%d: expected %v", i, true)
		}
	}
}

func TestReadLocalMachineIDMissing(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "fleet-")
	if err != nil {
		t.Fatalf("Failed creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	if machID := readLocalMachineID(dir); machID != "" {
		t.Fatalf("Received incorrect machID: %s", machID)
	}
}

func TestReadLocalMachineIDFound(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "fleet-")
	if err != nil {
		t.Fatalf("Failed creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	tmpMachineIDPath := filepath.Join(dir, "/etc/machine-id")
	err = os.MkdirAll(filepath.Dir(tmpMachineIDPath), os.FileMode(0755))
	if err != nil {
		t.Fatalf("Failed setting up fake mach ID path: %v", err)
	}

	err = ioutil.WriteFile(tmpMachineIDPath, []byte("pingpong"), os.FileMode(0644))
	if err != nil {
		t.Fatalf("Failed writing fake mach ID file: %v", err)
	}

	if machID := readLocalMachineID(dir); machID != "pingpong" {
		t.Fatalf("Received incorrect machID %q, expected 'pingpong'", machID)
	}
}
