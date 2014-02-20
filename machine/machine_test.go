package machine

import (
	"testing"
)

func TestHasMetadataSimpleMatch(t *testing.T) {
	metadata := map[string]string{
		"region": "us-east-1",
	}
	mach := New("", "", metadata)

	match := map[string][]string{
		"region": []string{"us-east-1"},
	}
	if !mach.HasMetadata(match) {
		t.Errorf("Machine reported it did not have expected state")
	}
}

func TestHasMetadataMultiMatch(t *testing.T) {
	metadata := map[string]string{
		"groups": "ping",
	}
	mach := New("", "", metadata)

	match := map[string][]string{
		"groups": []string{"ping", "pong"},
	}
	if !mach.HasMetadata(match) {
		t.Errorf("Machine reported it did not have expected state")
	}
}

func TestHasMetadataSingleMatchFail(t *testing.T) {
	metadata := map[string]string{
		"groups": "ping",
	}
	mach := New("", "", metadata)

	match := map[string][]string{
		"groups": []string{"pong"},
	}
	if mach.HasMetadata(match) {
		t.Errorf("Machine reported a successful match for metadata which it does not have")
	}
}

func TestHasMetadataPartialMatchFail(t *testing.T) {
	metadata := map[string]string{
		"region": "us-east-1",
		"groups": "ping",
	}
	mach := New("", "", metadata)

	match := map[string][]string{
		"region": []string{"us-east-1"},
		"groups": []string{"pong"},
	}
	if mach.HasMetadata(match) {
		t.Errorf("Machine reported a successful match for metadata which it does not have")
	}
}
