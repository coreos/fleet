package job

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/unit"
)

func TestNewJobPayloadBadType(t *testing.T) {
	j := NewJobPayload("foo.unknown", *unit.NewSystemdUnitFile("echo"))
	_, err := j.Type()

	if err == nil {
		t.Fatal("Expected non-nil error")
	}
}

func TestNewJobPayload(t *testing.T) {
	payload := NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))

	if payload.Name != "echo.service" {
		t.Errorf("Payload has unexpected name '%s'", payload.Name)
	}

	if pt, _ := payload.Type(); pt != "systemd-service" {
		t.Errorf("Payload has unexpected Type '%s'", pt)
	}
}

func TestJobPayloadServiceDefaultPeers(t *testing.T) {
	unitFile := unit.NewSystemdUnitFile("")
	payload := NewJobPayload("echo.service", *unitFile)
	peers := payload.Peers()

	if len(peers) != 0 {
		t.Fatalf("Unexpected number of peers %d, expected 0", len(peers))
	}
}

func TestJobPayloadSocketDefaultPeers(t *testing.T) {
	unitFile := unit.NewSystemdUnitFile("")
	payload := NewJobPayload("echo.socket", *unitFile)
	peers := payload.Peers()

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

func TestMapUnitContentsV0ToV1(t *testing.T) {
	contents := map[string]map[string]string{
		"key1": map[string]string{
			"subkey1": "value1",
			"subkey2": "value2",
		},
		"key2": map[string]string{
			"subkey3": "value3",
			"subkey4": "value4",
		},
	}

	expected := map[string]map[string][]string{
		"key1": map[string][]string{
			"subkey1": []string{"value1"},
			"subkey2": []string{"value2"},
		},
		"key2": map[string][]string{
			"subkey3": []string{"value3"},
			"subkey4": []string{"value4"},
		},
	}

	actual := mapUnitContentsV0ToV1(contents)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", actual, expected)
	}
}

func TestMapUnitContentsV1ToV0(t *testing.T) {
	contents := map[string]map[string][]string{
		"key1": map[string][]string{
			"subkey1": []string{"value1", "altvalue1"},
			"subkey2": []string{"value2"},
		},
		"key2": map[string][]string{
			"subkey3": []string{"value3"},
			"subkey4": []string{"value4", "altvalue2"},
		},
	}

	expected := map[string]map[string]string{
		"key1": map[string]string{
			"subkey1": "altvalue1",
			"subkey2": "value2",
		},
		"key2": map[string]string{
			"subkey3": "value3",
			"subkey4": "altvalue2",
		},
	}

	actual := mapUnitContentsV1ToV0(contents)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", actual, expected)
	}
}
