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
	if section["Description"] != "Foo" {
		t.Fatalf("Unit.Description is incorrect")
	}

	section = unitFile.GetSection("Service")
	if section["ExecStart"] != "echo \"ping\";" {
		t.Fatalf("Service.ExecStart is incorrect")
	}
	if section["ExecStop"] != "echo \"pong\";" {
		t.Fatalf("Service.ExecStop is incorrect")
	}

	section = unitFile.GetSection("Install")
	if section["WantedBy"] != "fleet-ping.target" {
		t.Fatalf("Install.WantedBy is incorrect")
	}
}

func TestSerializeDeserialize(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	deserialized := NewSystemdUnitFile(contents)
	serialized := deserialized.String()
	deserialized = NewSystemdUnitFile(serialized)

	section := deserialized.GetSection("Unit")
	if val, ok := section["Description"]; !ok || val != "Foo" {
		t.Fatalf("Failed to persist data through serialize/deserialize")
	}
}

func TestSerializeDeserializeWithChanges(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	deserialized := NewSystemdUnitFile(contents)
	deserialized.SetField("Unit", "Description", "Bar")
	deserialized.SetField("NewSection", "Field", "Baz")
	serialized := deserialized.String()
	deserialized = NewSystemdUnitFile(serialized)

	section := deserialized.GetSection("Unit")
	if val, ok := section["Description"]; !ok || val != "Bar" {
		t.Fatalf("Failed to persist data through serialize/deserialize")
	}

	section = deserialized.GetSection("NewSection")
	if val, ok := section["Field"]; !ok || val != "Baz" {
		t.Fatalf("Failed to persist data through serialize/deserialize")
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

func TestSetFieldNewSection(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile := NewSystemdUnitFile(contents)
	unitFile.SetField("NewSection", "Field", "Bar")

	section := unitFile.GetSection("NewSection")
	if val, ok := section["Field"]; !ok || val != "Bar" {
		t.Fatalf("Failed to persist value in new section")
	}
}

func TestSetFieldExistingSectionNewOption(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile := NewSystemdUnitFile(contents)
	unitFile.SetField("Unit", "Description", "Bar")

	section := unitFile.GetSection("Unit")
	if val, ok := section["Description"]; !ok || val != "Bar" {
		t.Fatalf("Failed to persist value in existing section")
	}
}

func TestSetFieldExistingSectionExistingOption(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile := NewSystemdUnitFile(contents)
	unitFile.SetField("Unit", "Field", "Baz")

	section := unitFile.GetSection("Unit")
	if val, ok := section["Field"]; !ok || val != "Baz" {
		t.Fatalf("Failed to persist value in existing section")
	}
}

func TestSetFieldChangesPersist(t *testing.T) {
	contents := `
[Unit]
Description = Foo
`
	unitFile := NewSystemdUnitFile(contents)
	unitFile.SetField("NewSection", "Field", "Baz")


}
