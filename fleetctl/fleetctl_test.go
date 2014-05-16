package main

import (
	"testing"

	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/version"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"
)

func newFakeRegistryForCheckVersion(v string) registry.Registry {
	sv, err := semver.NewVersion(v)
	if err != nil {
		panic(err)
	}

	fr := registry.NewFakeRegistry()
	fr.SetLatestVersion(*sv)

	return fr
}

func TestCheckVersion(t *testing.T) {
	registryCtl = newFakeRegistryForCheckVersion(version.Version)
	_, ok := checkVersion()
	if !ok {
		t.Errorf("checkVersion failed but should have succeeded")
	}
	registryCtl = newFakeRegistryForCheckVersion("0.4.0")
	msg, ok := checkVersion()
	if ok || msg == "" {
		t.Errorf("checkVersion succeeded but should have failed")
	}
}
