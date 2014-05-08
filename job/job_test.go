package job

import (
	"testing"

	"github.com/coreos/fleet/unit"
)

func TestNewJob(t *testing.T) {
	j1 := NewJob("pong.service", *unit.NewUnit("Echo"))

	if j1.Name != "pong.service" {
		t.Error("job.Job.Name != 'pong.service'")
	}
}

func TestJobWithPeers(t *testing.T) {
	j := NewJob("echo.service", *unit.NewUnit(``))
	peers := j.Peers()

	if len(peers) != 0 {
		t.Fatalf("Unexpected number of peers %d, expected 0", len(peers))
	}
}

func TestJobWithoutPeers(t *testing.T) {
	contents := `[X-Fleet]
X-ConditionMachineOf="foo.service" "bar.service"
`
	j := NewJob("echo.service", *unit.NewUnit(contents))
	peers := j.Peers()

	if len(peers) != 2 {
		t.Fatalf("Unexpected number of peers %d, expected 2", len(peers))
	}

	if peers[0] != "foo.service" {
		t.Errorf("Expected first peer to be foo.service, got %s", peers[0])
	}

	if peers[1] != "bar.service" {
		t.Errorf("Expected second peer to be bar.service, got %s", peers[1])
	}
}

func TestJobConflicts(t *testing.T) {
	contents := `[Unit]
Description=Testing

[X-Fleet]
X-Conflicts=*bar*
`
	j := NewJob("echo.service", *unit.NewUnit(contents))
	conflicts := j.Conflicts()

	if len(conflicts) != 1 {
		t.Errorf("Expected 1 conflict, received %v", conflicts)
	}

	if conflicts[0] != "*bar*" {
		t.Errorf("Expected first conflict to be '*bar*', received %s", conflicts[1])
	}
}

func TestJobConflictsNotProvided(t *testing.T) {
	j := NewJob("echo.socket", *unit.NewUnit(""))
	conflicts := j.Conflicts()

	if len(conflicts) > 0 {
		t.Fatalf("Expected no conflicts, received %v", conflicts)
	}
}

func TestParseRequirements(t *testing.T) {
	contents := `
[X-Fleet]
X-Foo=Bar
Ping=Pong
X-Key=Value
`
	j := NewJob("foo.service", *unit.NewUnit(contents))
	reqs := j.Requirements()
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
	j := NewJob("foo.service", *unit.NewUnit(contents))
	reqs := j.Requirements()
	if len(reqs) != 2 {
		t.Fatalf("Incorrect number of requirements; got %d, expected 2: %v", len(reqs), reqs)
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
	j := NewJob("foo.service", *unit.NewUnit(contents))
	reqs := j.Requirements()
	if len(reqs) != 0 {
		t.Fatalf("Incorrect number of requirements; got %d, expected 0", len(reqs))
	}
}

func TestJobConditionMachineID(t *testing.T) {
	tests := []struct {
		unit string
		outS string
		outB bool
	}{
		// Simplest case
		{
			`[X-Fleet]
X-ConditionMachineID=123
`,
			"123",
			true,
		},

		// First value wins
		// TODO(bcwaldon): maybe the last one should win?
		{
			`[X-Fleet]
X-ConditionMachineID="123" "456"
`,
			"123",
			true,
		},

		// No value provided
		{
			`[X-Fleet]`,
			"",
			false,
		},

		// Ensure we fall back to the legacy boot ID option
		{
			`[X-Fleet]
X-ConditionMachineBootID=123
`,
			"123",
			true,
		},

		// Fall back to legacy option only if non-boot ID is absent
		{
			`[X-Fleet]
X-ConditionMachineBootID=123
X-ConditionMachineID=456
`,
			"456",
			true,
		},
	}

	for _, tt := range tests {
		j := NewJob("echo.service", *unit.NewUnit(tt.unit))
		outS, outB := j.RequiredTarget()

		if outS != tt.outS {
			t.Errorf("Expected target requirement %s, got %s", tt.outS, outS)
		}

		if outB != tt.outB {
			t.Errorf("Expected target requirement ok-val %t, got %t", tt.outB, outB)
		}
	}
}
