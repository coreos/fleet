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

func TestSupportedUnitTypes(t *testing.T) {
	ut := SupportedUnitTypes()
	if len(ut) < 1 {
		t.Fatalf("SupportedUnitTypes should return non-empty []string, got %v", ut)
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
