package machine

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestStackState(t *testing.T) {
	top := MachineState{"c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}, "1"}
	bottom := MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, ""}
	stacked := stackState(top, bottom)

	if stacked.BootID != "c31e44e1-f858-436e-933e-59c642517860" {
		t.Errorf("Unexpected BootID value %s", stacked.BootID)
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
	bottom := MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, ""}
	stacked := stackState(top, bottom)

	if stacked.BootID != "595989bb-cbb7-49ce-8726-722d6e157b4e" {
		t.Errorf("Unexpected BootID value %s", stacked.BootID)
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

var shortBootIDTests = []struct {
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
		},
		s: "5959",
		l: "5959",
	},
}

func TestStateShortBootID(t *testing.T) {
	for i, tt := range shortBootIDTests {
		if g := tt.m.ShortBootID(); g != tt.s {
			t.Errorf("#%d: got %q, want %q", i, g, tt.s)
		}
	}
}

func TestStateMatchBootID(t *testing.T) {
	for i, tt := range shortBootIDTests {
		if tt.s != "" {
			if ok := tt.m.MatchBootID(""); ok {
				t.Errorf("#%d: expected %v", i, false)
			}
		}

		if ok := tt.m.MatchBootID("foobar"); ok {
			t.Errorf("#%d: expected %v", i, false)
		}

		if ok := tt.m.MatchBootID(tt.l); !ok {
			t.Errorf("#%d: expected %v", i, true)
		}

		if ok := tt.m.MatchBootID(tt.s); !ok {
			t.Errorf("#%d: expected %v", i, true)
		}
	}
}

func TestReadLocalBootIDMissing(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "fleet-")
	if err != nil {
		t.Fatalf("Failed creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	if bootID := readLocalBootID(dir); bootID != "" {
		t.Fatalf("Received incorrect bootID: %s", bootID)
	}
}

func TestReadLocalBootIDFound(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "fleet-")
	if err != nil {
		t.Fatalf("Failed creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	tmpBootIDPath := filepath.Join(dir, "/proc/sys/kernel/random/boot_id")
	err = os.MkdirAll(filepath.Dir(tmpBootIDPath), os.FileMode(0755))
	if err != nil {
		t.Fatalf("Failed setting up fake boot ID path: %v", err)
	}

	err = ioutil.WriteFile(tmpBootIDPath, []byte("pingpong"), os.FileMode(0644))
	if err != nil {
		t.Fatalf("Failed writing fake boot ID file: %v", err)
	}

	if bootID := readLocalBootID(dir); bootID != "pingpong" {
		t.Fatalf("Received incorrect bootID %q, expected 'pingpong'", bootID)
	}
}
