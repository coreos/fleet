package unit

import "testing"
import "reflect"
import "github.com/coreos/fleet/machine"

const (
	// $ echo -n "foo" | sha1sum
	// 0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33 -
	testData      = "foo"
	testShaString = "0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33"
)

func TestUnitHash(t *testing.T) {
	u := NewUnit(testData)
	h := u.Hash()
	if h.String() != testShaString {
		t.Fatalf("Unit Hash (%s) does not match expected (%s)", h.String(), testShaString)
	}

	eh := &Hash{}
	if !eh.Empty() {
		t.Fatalf("Empty hash check failed: %v", eh.Empty())
	}
}

func TestRecognizedUnitTypes(t *testing.T) {
	tts := []struct{
		name string
		ok   bool
	}{
		{"foo.service", true},
		{"foo.socket", true},
		{"foo.path", true},
		{"foo.timer", true},
		{"foo.mount", true},
		{"foo.automount", true},
		{"foo.device", true},
		{"foo.unknown", false},
	}

	for _, tt := range tts {
		ok := RecognizedUnitType(tt.name)
		if ok != tt.ok {
			t.Errorf("Case failed: name=%s expect=%b result=%b", tt.name, tt.ok, ok)
		}
	}
}

func TestDefaultUnitType(t *testing.T) {
	tts := []struct{
		name string
		out  string
	}{
		{"foo", "foo.service"},
		{"foo.service", "foo.service.service"},
		{"foo.link", "foo.link.service"},
	}

	for _, tt := range tts {
		out := DefaultUnitType(tt.name)
		if out != tt.out {
			t.Errorf("Case failed: name=%s expect=%s result=%s", tt.name, tt.out, out)
		}
	}
}

func TestNewUnitState(t *testing.T) {
	ms := &machine.MachineState{"id", "ip", nil, "version"}
	want := &UnitState{
		LoadState:    "ls",
		ActiveState:  "as",
		SubState:     "ss",
		MachineState: ms,
	}

	got := NewUnitState("ls", "as", "ss", ms)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewUnitState did not create a correct UnitState: got %s, want %s", got, want)
	}

}
