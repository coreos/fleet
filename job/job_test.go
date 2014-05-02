package job

import (
	"fmt"
	"testing"

	"github.com/coreos/fleet/unit"
)

func TestNewJob(t *testing.T) {
	j1 := NewJob("pong.service", *unit.NewUnit("Echo"))

	if j1.Name != "pong.service" {
		t.Error("job.Job.Name != 'pong.service'")
	}

	if jt, _ := j1.Type(); jt != "service" {
		t.Errorf("Job has unexpected Type '%s'", jt)
	}
}

func TestNewJobBadType(t *testing.T) {
	if _, err := NewJob("foo.unknown", *unit.NewUnit("echo")).Type(); err == nil {
		t.Errorf("Expected non-nil error, got %v", err)
	}

}

func TestNewJobGoodTypes(t *testing.T) {
	cases := []string{
		"service",
		"socket",
	}

	test := func(ut string) {
		name := fmt.Sprintf("foo.%s", ut)
		if _, err := NewJob(name, *unit.NewUnit("echo")).Type(); err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	}

	for _, c := range cases {
		test(c)
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
	conflicts := j.Unit.Conflicts()

	if len(conflicts) != 1 {
		t.Errorf("Expected 1 conflict, received %v", conflicts)
	}

	if conflicts[0] != "*bar*" {
		t.Errorf("Expected first conflict to be '*bar*', received %s", conflicts[1])
	}
}

func TestJobConflictsNotProvided(t *testing.T) {
	j := NewJob("echo.socket", *unit.NewUnit(""))
	conflicts := j.Unit.Conflicts()

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
