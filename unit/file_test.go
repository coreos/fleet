package unit

import (
	"reflect"
	"testing"
)

func TestDeserialize(t *testing.T) {
	contents := `
This=Ignored
[Unit]
;ignore this guy
Description = Foo

[Service]
ExecStart=echo "ping";
ExecStop=echo "pong"
# ignore me, too
ExecStop=echo post
`

	expected := map[string]map[string][]string{
		"Unit": map[string][]string{
			"Description": []string{"Foo"},
		},
		"Service": map[string][]string{
			"ExecStart": []string{"echo \"ping\";"},
			"ExecStop":  []string{"echo \"pong\"", "echo post"},
		},
	}

	unitFile := NewSystemdUnitFile(contents)

	if !reflect.DeepEqual(expected, unitFile.Contents) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", unitFile.Contents, expected)
	}
}

func TestSerializeDeserialize(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	deserialized := NewSystemdUnitFile(contents)
	section := deserialized.Contents["Unit"]
	if val, ok := section["Description"]; !ok || val[0] != "Foo" {
		t.Errorf("Failed to persist data through serialize/deserialize: %v", val)
	}

	serialized := deserialized.String()
	deserialized = NewSystemdUnitFile(serialized)

	section = deserialized.Contents["Unit"]
	if val, ok := section["Description"]; !ok || val[0] != "Foo" {
		t.Errorf("Failed to persist data through serialize/deserialize: %v", val)
	}
}

func TestDescription(t *testing.T) {
	contents := `
[Unit]
Description = Foo

[Service]
ExecStart=echo "ping";
ExecStop=echo "pong";
`

	unitFile := NewSystemdUnitFile(contents)
	if unitFile.Description() != "Foo" {
		t.Fatalf("Unit.Description is incorrect")
	}
}

func TestDescriptionNotDefined(t *testing.T) {
	contents := `
[Unit]

[Service]
ExecStart=echo "ping";
ExecStop=echo "pong";
`

	unitFile := NewSystemdUnitFile(contents)
	if unitFile.Description() != "" {
		t.Fatalf("Unit.Description is incorrect")
	}
}

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

	uf := &SystemdUnitFile{Contents: contents}
	actual := uf.LegacyContents()

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", actual, expected)
	}
}

func TestNewSystemdUnitFileFromLegacyContents(t *testing.T) {
	legacy := map[string]map[string]string{
		"Unit": map[string]string{
			"Description": "foobar",
		},
		"Service": map[string]string{
			"Type":      "oneshot",
			"ExecStart": "/usr/bin/echo bar",
		},
	}

	expected := map[string]map[string][]string{
		"Unit": map[string][]string{
			"Description": []string{"foobar"},
		},
		"Service": map[string][]string{
			"Type":      []string{"oneshot"},
			"ExecStart": []string{"/usr/bin/echo bar"},
		},
	}

	actual := NewSystemdUnitFileFromLegacyContents(legacy).Contents

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", actual, expected)
	}
}
