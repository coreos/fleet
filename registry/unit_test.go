package registry

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/unit"
)

func TestLegacyContents(t *testing.T) {
	contents := map[string]map[string][]string{
		"Unit": map[string][]string{
			"Description": []string{"foobar"},
			"Wants":       []string{},
		},
		"Service": map[string][]string{
			"Type":      []string{"oneshot"},
			"ExecStart": []string{"foo", "bar"},
		},
	}
	expected := map[string]map[string]string{
		"Unit": map[string]string{
			"Description": "foobar",
		},
		"Service": map[string]string{
			"Type":      "oneshot",
			"ExecStart": "bar",
		},
	}

	uf := unit.Unit{Contents: contents}
	actual := getLegacyUnitContents(uf)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", actual, expected)
	}
}
