package main

import (
	"testing"

	"github.com/coreos/fleet/machine"
)

func TestBootIDLegend(t *testing.T) {
	ms := machine.MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}}

	l := machineBootIDLegend(ms, true)
	if l != "595989bb-cbb7-49ce-8726-722d6e157b4e" {
		t.Errorf("Expected full bootID, but it was %s\n", l)
	}

	l = machineBootIDLegend(ms, false)
	if l != "595989bb..." {
		t.Errorf("Expected partial bootID, but it was %s\n", l)
	}
}

func TestFullLegendWithPublicIP(t *testing.T) {
	ms := machine.MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}}

	l := machineFullLegend(ms, false)
	if l != "595989bb.../5.6.7.8" {
		t.Errorf("Expected partial bootID with public IP, but it was %s\n", l)
	}

	l = machineFullLegend(ms, true)
	if l != "595989bb-cbb7-49ce-8726-722d6e157b4e/5.6.7.8" {
		t.Errorf("Expected full bootID with public IP, but it was %s\n", l)
	}
}

func TestFullLegendWithoutPublicIP(t *testing.T) {
	ms := machine.MachineState{"520983A8-FB9C-4A68-B49C-CED5BB2E9D08", "", map[string]string{"foo": "bar"}}

	l := machineFullLegend(ms, false)
	if l != "520983A8..." {
		t.Errorf("Expected partial bootID without public IP, but it was %s\n", l)
	}

	l = machineFullLegend(ms, true)
	if l != "520983A8-FB9C-4A68-B49C-CED5BB2E9D08" {
		t.Errorf("Expected full bootID without public IP, but it was %s\n", l)
	}
}
