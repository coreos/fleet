package main

import (
	"testing"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/resource"
	"github.com/coreos/fleet/version"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"
)

func newFakeRegistryForCheckVersion(v string) registry.Registry {
	sv, err := semver.NewVersion(v)
	if err != nil {
		panic(err)
	}

	reg := registry.NewFakeRegistry()
	reg.SetLatestVersion(*sv)

	return reg
}

func TestCheckVersion(t *testing.T) {
	cAPI = newFakeRegistryForCheckVersion(version.Version)
	_, ok := checkVersion()
	if !ok {
		t.Errorf("checkVersion failed but should have succeeded")
	}
	cAPI = newFakeRegistryForCheckVersion("9.0.0")
	msg, ok := checkVersion()
	if ok || msg == "" {
		t.Errorf("checkVersion succeeded but should have failed")
	}
}

func TestMachineIDLegend(t *testing.T) {
	ms := machine.MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, "", resource.ResourceTuple{}}

	l := machineIDLegend(ms, true)
	if l != "595989bb-cbb7-49ce-8726-722d6e157b4e" {
		t.Errorf("Expected full machine ID, but it was %s\n", l)
	}

	l = machineIDLegend(ms, false)
	if l != "595989bb..." {
		t.Errorf("Expected partial machine ID, but it was %s\n", l)
	}
}

func TestFullLegendWithPublicIP(t *testing.T) {
	ms := machine.MachineState{"595989bb-cbb7-49ce-8726-722d6e157b4e", "5.6.7.8", map[string]string{"foo": "bar"}, "", resource.ResourceTuple{}}

	l := machineFullLegend(ms, false)
	if l != "595989bb.../5.6.7.8" {
		t.Errorf("Expected partial machine ID with public IP, but it was %s\n", l)
	}

	l = machineFullLegend(ms, true)
	if l != "595989bb-cbb7-49ce-8726-722d6e157b4e/5.6.7.8" {
		t.Errorf("Expected full machine ID with public IP, but it was %s\n", l)
	}
}

func TestFullLegendWithoutPublicIP(t *testing.T) {
	ms := machine.MachineState{"520983A8-FB9C-4A68-B49C-CED5BB2E9D08", "", map[string]string{"foo": "bar"}, "", resource.ResourceTuple{}}

	l := machineFullLegend(ms, false)
	if l != "520983A8..." {
		t.Errorf("Expected partial machine ID without public IP, but it was %s\n", l)
	}

	l = machineFullLegend(ms, true)
	if l != "520983A8-FB9C-4A68-B49C-CED5BB2E9D08" {
		t.Errorf("Expected full machine ID without public IP, but it was %s\n", l)
	}
}

var unitNameMangleTests = map[string]string{
	"foo":            "foo.service",
	"foo.1":          "foo.1.service",
	"foo/bar.socket": "bar.socket",
	"foo.socket":     "foo.socket",
	"foo.service":    "foo.service",
}

func TestUnitNameMangle(t *testing.T) {
	for n, w := range unitNameMangleTests {
		if g := unitNameMangle(n); g != w {
			t.Errorf("got %q, want %q", g, w)
		}
	}
}
