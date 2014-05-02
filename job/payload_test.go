package job

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/coreos/fleet/unit"
)

func TestNewJobPayloadBadType(t *testing.T) {
	if _, err := NewJobPayload("foo.unknown", *unit.NewSystemdUnitFile("echo")).Type(); err == nil {
		t.Errorf("Expected non-nil error, got %v", err)
	}

}

func TestNewJobPayloadGoodTypes(t *testing.T) {
	cases := []string{
		"service",
		"socket",
	}

	test := func(ut string) {
		name := fmt.Sprintf("foo.%s", ut)
		if _, err := NewJobPayload(name, *unit.NewSystemdUnitFile("echo")).Type(); err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	}

	for _, c := range cases {
		test(c)
	}
}

func TestNewJobPayload(t *testing.T) {
	payload := NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))

	if payload.Name != "echo.service" {
		t.Errorf("Payload has unexpected name '%s'", payload.Name)
	}

	_, err := payload.Type()
	if err != nil {
		t.Error(err)
	}
}

func TestJobPayloadServiceDefaultPeers(t *testing.T) {
	unitFile := unit.NewSystemdUnitFile("")
	payload := NewJobPayload("echo.service", *unitFile)
	peers, err := payload.Peers()
	if err != nil {
		t.Fatal(err)
	}

	if len(peers) != 0 {
		t.Fatalf("Unexpected number of peers %d, expected 0", len(peers))
	}
}

func TestJobPayloadSocketDefaultPeers(t *testing.T) {
	unitFile := unit.NewSystemdUnitFile("")
	payload := NewJobPayload("echo.socket", *unitFile)
	peers, err := payload.Peers()
	if err != nil {
		t.Fatal(err)
	}

	if len(peers) != 1 {
		t.Fatalf("Unexpected number of peers %d, expected 1", len(peers))
	}

	if peers[0] != "echo.service" {
		t.Fatalf("Unexpected peers: %v", peers)
	}
}

func TestJobPayloadConflicts(t *testing.T) {
	contents := `[Unit]
Description=Testing

[X-Fleet]
X-Conflicts=*bar*
`
	unitFile := unit.NewSystemdUnitFile(contents)
	payload := NewJobPayload("echo.service", *unitFile)
	conflicts := payload.Conflicts()

	if len(conflicts) != 1 {
		t.Errorf("Expected 1 conflict, received %v", conflicts)
	}

	if conflicts[0] != "*bar*" {
		t.Errorf("Expected first conflict to be '*bar*', received %s", conflicts[1])
	}
}

func TestJobPayloadConflictsNotProvided(t *testing.T) {
	unitFile := unit.NewSystemdUnitFile("")
	payload := NewJobPayload("echo.socket", *unitFile)
	conflicts := payload.Conflicts()

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
	unitFile := unit.NewSystemdUnitFile(contents)
	jp := NewJobPayload("foo.service", *unitFile)
	reqs := jp.Requirements()
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
	unitFile := unit.NewSystemdUnitFile(contents)
	jp := NewJobPayload("foo.service", *unitFile)
	reqs := jp.Requirements()
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
	unitFile := unit.NewSystemdUnitFile(contents)
	jp := NewJobPayload("foo.service", *unitFile)
	reqs := jp.Requirements()
	if len(reqs) != 0 {
		t.Fatalf("Incorrect number of requirements; got %d, expected 0", len(reqs))
	}
}

var pathJobTestExamples = []struct {
	content string
	peers   []string
}{
	{"[Path]\n[X-Fleet]\nX-ConditionMachineOf=bar.service", []string{"bar.service"}},
	{"[Path]\nUnit=bar.service", []string{"bar.service"}},
	{"[Path]", []string{"foo.service"}},
}

func TestPeersForPathUnits(t *testing.T) {
	for _, s := range pathJobTestExamples {
		unitFile := unit.NewSystemdUnitFile(s.content)
		jp := NewJobPayload("foo.path", *unitFile)
		g, err := jp.Peers()
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(g, s.peers) {
			t.Errorf("Unexpected peers for %q.\n\tgot %q, want %q", s.content, g, s.peers)
		}
	}
}

func TestPeersForUnknowUnit(t *testing.T) {
	contents := `
[Unit]
Description=Timmy
`
	unitFile := unit.NewSystemdUnitFile(contents)
	jp := NewJobPayload("foo.foo", *unitFile)
	_, err := jp.Peers()
	if err == nil {
		t.Fatal("Expected to return an error getting peers for unknown unit types")
	}
}
