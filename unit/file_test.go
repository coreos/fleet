package unit

import (
	"testing"
)

func TestDeserialize(t *testing.T) {
	contents := `
[Unit]
Description = Foo

[Service]
ExecStart=echo "ping";
ExecStop=echo "pong";

[Install]
WantedBy=fleet-ping.target
`

	unitFile := NewSystemdUnitFile(contents)

	section := unitFile.GetSection("Unit")
	if section["Description"][0] != "Foo" {
		t.Fatalf("Unit.Description is incorrect")
	}

	section = unitFile.GetSection("Service")
	if section["ExecStart"][0] != "echo \"ping\";" {
		t.Fatalf("Service.ExecStart is incorrect")
	}
	if section["ExecStop"][0] != "echo \"pong\";" {
		t.Fatalf("Service.ExecStop is incorrect")
	}

	section = unitFile.GetSection("Install")
	if section["WantedBy"][0] != "fleet-ping.target" {
		t.Fatalf("Install.WantedBy is incorrect")
	}
}

func TestSerializeDeserialize(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	deserialized := NewSystemdUnitFile(contents)
	section := deserialized.GetSection("Unit")
	if val, ok := section["Description"]; !ok || val[0] != "Foo" {
		t.Errorf("Failed to persist data through serialize/deserialize: %v", val)
	}

	serialized := deserialized.String()
	deserialized = NewSystemdUnitFile(serialized)

	section = deserialized.GetSection("Unit")
	if val, ok := section["Description"]; !ok || val[0] != "Foo" {
		t.Errorf("Failed to persist data through serialize/deserialize: %v", val)
	}
}

func TestSerializeDeserializeWithChanges(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	deserialized := NewSystemdUnitFile(contents)
	deserialized.ReplaceField("Unit", "Description", "Bar")
	deserialized.ReplaceField("NewSection", "Field", "Baz")
	serialized := deserialized.String()
	deserialized = NewSystemdUnitFile(serialized)

	section := deserialized.GetSection("Unit")
	if val, ok := section["Description"]; !ok || val[0] != "Bar" {
		t.Errorf("Failed to persist data through serialize/deserialize: %v", val)
	}

	section = deserialized.GetSection("NewSection")
	if val, ok := section["Field"]; !ok || val[0] != "Baz" {
		t.Errorf("Failed to persist data through serialize/deserialize: %v", val)
	}
}

func TestParseRequirements(t *testing.T) {
	contents := `
[X-Fleet]
X-Foo=Bar
Ping=Pong
X-Key=Value
`
	unitFile := NewSystemdUnitFile(contents)
	reqs := unitFile.Requirements()
	if len(reqs) != 2 {
		t.Fatalf("Incorrect number of requirements; got %d, expected 2", len(reqs))
	}

	if len(reqs["Foo"]) != 1 || reqs["Foo"][0] != "Bar" {
		t.Fatalf("Incorrect value %q of requirement 'Foo'", reqs["Foo"])
	}

	if len(reqs["Key"]) != 1 || reqs["Key"][0] != "Value" {
		t.Fatalf("Incorrect value %q of requirement 'Key'", reqs["Key"])
	}
}

func TestParseRequirementsMultipleValuesForKeyStack(t *testing.T) {
	contents := `
[X-Fleet]
X-Foo=Bar
X-Foo=Baz
X-Ping=Pong
X-Ping=Pang
`
	unitFile := NewSystemdUnitFile(contents)
	reqs := unitFile.Requirements()
	if len(reqs) != 2 {
		t.Fatalf("Received %d requirements, expected 2: %v", len(reqs), reqs)
	}

	if len(reqs["Foo"]) != 2 || reqs["Foo"][0] != "Bar" || reqs["Foo"][1] != "Baz" {
		t.Fatalf("Incorrect value %v of requirement 'Foo'", reqs["Foo"])
	}

	if len(reqs["Ping"]) != 2 || reqs["Ping"][0] != "Pong" || reqs["Ping"][1] != "Pang" {
		t.Fatalf("Incorrect value %v of requirement 'Ping'", reqs["Ping"])
	}
}

func TestParseRequirementsMissingSection(t *testing.T) {
	contents := `
[Unit]
Description=Timmy
`
	unitFile := NewSystemdUnitFile(contents)
	reqs := unitFile.Requirements()
	if len(reqs) != 0 {
		t.Fatalf("Incorrect number of requirements; got %d, expected 0", len(reqs))
	}
}

func TestGetSectionMissing(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile := NewSystemdUnitFile(contents)
	section := unitFile.GetSection("Missing")

	if len(section) != 0 {
		t.Fatalf("Returned unexpected data for undefined section")
	}
}

func TestDescription(t *testing.T) {
	contents := `
[Unit]
Description = Foo

[Service]
ExecStart=echo "ping";
ExecStop=echo "pong";

[Install]
WantedBy=fleet-ping.target
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

[Install]
WantedBy=fleet-ping.target
`

	unitFile := NewSystemdUnitFile(contents)
	if unitFile.Description() != "" {
		t.Fatalf("Unit.Description is incorrect")
	}
}

func TestReplaceFieldNewSection(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile := NewSystemdUnitFile(contents)
	unitFile.ReplaceField("NewSection", "Field", "Bar")

	section := unitFile.GetSection("NewSection")
	if val, ok := section["Field"]; !ok || val[0] != "Bar" {
		t.Fatalf("Failed to persist value in new section")
	}
}

func TestReplaceFieldExistingSectionNewOption(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile := NewSystemdUnitFile(contents)
	unitFile.ReplaceField("Unit", "Description", "Bar")

	section := unitFile.GetSection("Unit")
	if val, ok := section["Description"]; !ok || val[0] != "Bar" {
		t.Fatalf("Failed to persist value in existing section")
	}
}

func TestReplaceFieldExistingSectionExistingOption(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile := NewSystemdUnitFile(contents)
	unitFile.ReplaceField("Unit", "Field", "Baz")

	section := unitFile.GetSection("Unit")
	if val, ok := section["Field"]; !ok || val[0] != "Baz" {
		t.Fatalf("Failed to persist value in existing section")
	}
}

func TestReplaceFieldChangesPersist(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile := NewSystemdUnitFile(contents)
	unitFile.ReplaceField("NewSection", "Field", "Baz")
}
