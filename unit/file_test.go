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
