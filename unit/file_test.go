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

[Fleet]
X-ConditionMachineMetadata=foo=bar
X-ConditionMachineMetadata=baz=qux
`

	expected := map[string]map[string][]string{
		"Unit": {
			"Description": {"Foo"},
		},
		"Service": {
			"ExecStart": {`echo "ping";`},
			"ExecStop":  {`echo "pong"`, "echo post"},
		},
		"Fleet": {
			"X-ConditionMachineMetadata": {"foo=bar", "baz=qux"},
		},
	}

	unitFile, err := NewUnit(contents)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", contents, err)
	}

	if !reflect.DeepEqual(expected, unitFile.Contents) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", unitFile.Contents, expected)
	}
}

func TestDeserializedUnitGarbage(t *testing.T) {
	contents := `
>>>>>>>>>>>>>
[Service]
ExecStart=jim
# As long as a line has an equals sign, systemd is happy, so we should pass it through
<<<<<<<<<<<=bar
`
	expected := map[string]map[string][]string{
		"Service": {
			"ExecStart":   {"jim"},
			"<<<<<<<<<<<": {"bar"},
		},
	}
	unitFile, err := NewUnit(contents)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", contents, err)
	}

	if !reflect.DeepEqual(expected, unitFile.Contents) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", unitFile.Contents, expected)
	}
}

func TestDeserializeEscapedMultilines(t *testing.T) {
	contents := `
[Service]
ExecStart=echo \
  "pi\
  ng"
ExecStop=\
echo "po\
ng"
# comments within continuation should not be ignored
ExecStopPre=echo\
#pang
ExecStopPost=echo\
#peng\
pung
`
	expected := map[string]map[string][]string{
		"Service": {
			"ExecStart":    {`echo    "pi   ng"`},
			"ExecStop":     {`echo "po ng"`},
			"ExecStopPre":  {`echo #pang`},
			"ExecStopPost": {`echo #peng pung`},
		},
	}
	unitFile, err := NewUnit(contents)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", contents, err)
	}

	if !reflect.DeepEqual(expected, unitFile.Contents) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", unitFile.Contents, expected)
	}
}

func TestSerializeDeserialize(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	deserialized, err := NewUnit(contents)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", contents, err)
	}
	section := deserialized.Contents["Unit"]
	if val, ok := section["Description"]; !ok || val[0] != "Foo" {
		t.Errorf("Failed to persist data through serialize/deserialize: %v", val)
	}

	serialized := deserialized.String()
	deserialized, err = NewUnit(serialized)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", serialized, err)
	}

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

	unitFile, err := NewUnit(contents)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", contents, err)
	}
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

	unitFile, err := NewUnit(contents)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", contents, err)
	}
	if unitFile.Description() != "" {
		t.Fatalf("Unit.Description is incorrect")
	}
}

func TestNewSystemdUnitFileFromLegacyContents(t *testing.T) {
	legacy := map[string]map[string]string{
		"Unit": {
			"Description": "foobar",
		},
		"Service": {
			"Type":      "oneshot",
			"ExecStart": "/usr/bin/echo bar",
		},
	}

	expected := map[string]map[string][]string{
		"Unit": {
			"Description": {"foobar"},
		},
		"Service": {
			"Type":      {"oneshot"},
			"ExecStart": {"/usr/bin/echo bar"},
		},
	}

	u, err := NewUnitFromLegacyContents(legacy)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", legacy, err)
	}
	actual := u.Contents

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", actual, expected)
	}
}

func TestBadUnitsFail(t *testing.T) {
	bad := []string{
		`
[Unit]

[Service]
<<<<<<<<<<<<<<<<
`,
		`
[Unit]
nonsense upon stilts
`,
	}
	for _, tt := range bad {
		if _, err := NewUnit(tt); err == nil {
			t.Fatalf("Did not get expected error creating Unit from %q", tt)
		}
	}
}

func TestParseMultivalueLine(t *testing.T) {
	tests := []struct {
		in  string
		out []string
	}{
		{`"bar" "ping" "pong"`, []string{`bar`, `ping`, `pong`}},
		{`"bar"`, []string{`bar`}},
		{``, []string{""}},
		{`""`, []string{``}},
		{`"bar`, []string{`"bar`}},
		{`bar"`, []string{`bar"`}},
		{`foo\"bar`, []string{`foo\"bar`}},

		{
			`"bar" "`,
			[]string{`bar`, ``},
			//TODO(bcwaldon): should be something like this:
			// []string{`bar`},
		},

		{
			`"foo\"bar"`,
			[]string{`foo\bar`},
			//TODO(bcwaldon): should be something like this:
			// []string{`foo\"bar`},
		},
	}
	for i, tt := range tests {
		out := parseMultivalueLine(tt.in)
		if !reflect.DeepEqual(tt.out, out) {
			t.Errorf("case %d:, epected %v, got %v", i, tt.out, out)
		}
	}
}
