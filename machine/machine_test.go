package machine

import (
	"testing"
)

func TestStackState(t *testing.T) {
	top := MachineState{"c31e44e1-f858-436e-933e-59c642517860", "1.2.3.4", map[string]string{"ping": "pong"}}
	bottom := MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}}
	stacked := stackState(top, bottom)

	if stacked.BootId != "c31e44e1-f858-436e-933e-59c642517860" {
		t.Errorf("Unexpected BootId value %s", stacked.BootId)
	}

	if stacked.PublicIP != "1.2.3.4" {
		t.Errorf("Unexpected PublicIp value %s", stacked.PublicIP)
	}

	if len(stacked.Metadata) != 1 || stacked.Metadata["ping"] != "pong" {
		t.Errorf("Unexpected Metadata %v", stacked.Metadata)
	}
}

func TestStackStateEmptyTop(t *testing.T) {
	top := MachineState{}
	bottom := MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}}
	stacked := stackState(top, bottom)

	if stacked.BootId != "595989bb-cbb7-49ce-8726-722d6e157b4e" {
		t.Errorf("Unexpected BootId value %s", stacked.BootId)
	}

	if stacked.PublicIP != "5.6.7.8" {
		t.Errorf("Unexpected PublicIp value %s", stacked.PublicIP)
	}

	if len(stacked.Metadata) != 1 || stacked.Metadata["foo"] != "bar" {
		t.Errorf("Unexpected Metadata %v", stacked.Metadata)
	}
}

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
