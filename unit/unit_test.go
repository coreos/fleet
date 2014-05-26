package unit

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/resource"
)

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
	tts := []struct {
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
		{"foo.swap", false},
		{"foo.target", false},
		{"foo.snapshot", false},
		{"foo.network", false},
		{"foo.netdev", false},
		{"foo.link", false},
		{"foo.unknown", false},
	}

	for _, tt := range tts {
		ok := RecognizedUnitType(tt.name)
		if ok != tt.ok {
			t.Errorf("Case failed: name=%s expect=%t result=%t", tt.name, tt.ok, ok)
		}
	}
}

func TestDefaultUnitType(t *testing.T) {
	tts := []struct {
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
	ms := &machine.MachineState{"id", "ip", nil, "version", resource.ResourceTuple{}}
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

func TestInstanceUnit(t *testing.T) {
	tts := []struct {
		name string
		tmpl string
		pref string
		inst string
	}{
		// Everything past the first @ and before the last . is the instance
		{"foo@.service", "foo@.service", "foo", ""},
		{"foo@bar.service", "foo@.service", "foo", "bar"},
		{"foo@bar@baz.service", "foo@.service", "foo", "bar@baz"},
		{"foo.1@.service", "foo.1@.service", "foo.1", ""},
		{"foo.1@2.service", "foo.1@.service", "foo.1", "2"},
		{"ssh@.socket", "ssh@.socket", "ssh", ""},
		{"ssh@1.socket", "ssh@.socket", "ssh", "1"},
	}
	for _, tt := range tts {
		u := UnitNameToInstance(tt.name)
		if u == nil {
			t.Errorf("UnitNameToInstance(%s) returned nil InstanceUnit!", tt.name)
			continue
		}
		if u.FullName != tt.name {
			t.Errorf("UnitNameToInstance(%s) returned bad name: got %s, want %s", tt.name, u.FullName, tt.name)
		}
		if u.Template != tt.tmpl {
			t.Errorf("UnitNameToInstance(%s) returned bad template name: got %s, want %s", tt.name, u.Template, tt.tmpl)
		}
		if u.Prefix != tt.pref {
			t.Errorf("UnitNameToInstance(%s) returned bad prefix name: got %s, want %s", tt.name, u.Prefix, tt.pref)
		}
		if u.Instance != tt.inst {
			t.Errorf("UnitNameToInstance(%s) returned bad instance name: got %s, want %s", tt.name, u.Instance, tt.inst)
		}
	}

	bad := []string{"foo.service", "foo@", "bar.socket", "ssh.1.service"}
	for _, tt := range bad {
		if UnitNameToInstance(tt) != nil {
			t.Errorf("UnitNameToInstance returned non-nil InstanceUnit unexpectedly")
		}
	}

}
