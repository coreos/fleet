package main

import (
	"testing"

	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/version"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"
)

func newTestRegistryForCheckVersion(v string) registry.Registry {
	version, err := semver.NewVersion(v)
	if err != nil {
		panic(err)
	}
	return &TestRegistry{version: version}
}

func TestCheckVersion(t *testing.T) {
	registryCtl = newTestRegistryForCheckVersion(version.Version)
	_, ok := checkVersion()
	if !ok {
		t.Errorf("checkVersion failed but should have succeeded")
	}
	registryCtl = newTestRegistryForCheckVersion("0.4.0")
	msg, ok := checkVersion()
	if ok || msg == "" {
		t.Errorf("checkVersion succeeded but should have failed")
	}
}
