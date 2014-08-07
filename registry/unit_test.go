package registry

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/unit"
)

func TestMarshaling(t *testing.T) {
	units := []string{
		``,

		`[Service]
		ExecStart=/bin/sleep 1`,

		`[Unit]
		Description=Foo

		[Service]
		ExecStart=echo "foo"`,

		`[Path]
		PathExists=/foo`,
	}

	for _, contents := range units {
		u, err := unit.NewUnit(contents)
		if err != nil {
			t.Fatalf("unexpected error creating unit from %q: %v", contents, err)
		}
		json, err := marshal(u)
		if err != nil {
			t.Error("Error marshaling unit:", err)
		}
		var um unit.Unit
		err = unmarshal(json, &um)
		if err != nil {
			t.Error("Error unmarshaling unit:", err)
		}
		if !reflect.DeepEqual(*u, um) {
			t.Errorf("Unmarshaled unit does not match original!\nOriginal:\n%s\nUnmarshaled:\n%s", *u, um)
		}
	}

}
