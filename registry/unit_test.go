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
		u, _ := unit.NewUnit(contents)
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

func TestLegacyPayload(t *testing.T) {
	unitContents := `
[Service]
ExecStart=/bin/sleep 30000
`[1:]
	legacyPayloadContents := `{"Name":"sleep.service","Unit":{"Contents":{"Service":{"ExecStart":"/bin/sleep 30000"}},"Raw":"[Service]\nExecStart=/bin/sleep 30000\n"}}`
	want, _ := unit.NewUnit(unitContents)
	var ljp LegacyJobPayload
	err := unmarshal(legacyPayloadContents, &ljp)
	if err != nil {
		t.Error("Error unmarshaling legacy payload:", err)
	}
	got := ljp.Unit
	if !reflect.DeepEqual(*want, got) {
		t.Errorf("Unit from legacy payload does not match expected!\nwant:\n%s\ngot:\n%s", *want, got)
	}
}
