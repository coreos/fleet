package systemd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-systemd/dbus"
	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/unit"
)

const (
	DefaultUnitsDirectory = "/run/fleet/units/"
)

type SystemdUnitManager struct {
	systemd  *dbus.Conn
	UnitsDir string

	subscriptions *dbus.SubscriptionSet
}

func NewSystemdUnitManager(uDir string) (*SystemdUnitManager, error) {
	systemd, err := dbus.New()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(uDir, os.FileMode(0755)); err != nil {
		return nil, err
	}

	mgr := SystemdUnitManager{systemd, uDir, systemd.NewSubscriptionSet()}
	return &mgr, nil
}

func (m *SystemdUnitManager) MarshalJSON() ([]byte, error) {
	data := struct {
		DBUSSubscriptions []string
	}{
		DBUSSubscriptions: m.subscriptions.Values(),
	}
	return json.Marshal(data)
}

// Load writes the given Unit to disk, subscribing to relevant dbus
// events, and, if necessary, instructing the systemd daemon to reload.
func (m *SystemdUnitManager) Load(name string, u unit.Unit) error {
	err := m.writeUnit(name, u.String())
	if err != nil {
		return err
	}

	m.subscriptions.Add(name)
	return m.daemonReload()
}

// Unload removes the indicated unit from the filesystem, unsubscribing
// from relevant dbus events.
func (m *SystemdUnitManager) Unload(name string) {
	m.subscriptions.Remove(name)
	m.removeUnit(name)
	m.daemonReload()
}

// Start starts the unit identified by the given name
func (m *SystemdUnitManager) Start(name string) {
	m.startUnit(name)
}

// Stop stops the unit identified by the given name
func (m *SystemdUnitManager) Stop(name string) {
	m.stopUnit(name)
}

// GetUnitState generates a UnitState object representing the
// current state of a Unit
func (m *SystemdUnitManager) GetUnitState(name string) (*unit.UnitState, error) {
	loadState, activeState, subState, err := m.getUnitStates(name)
	if err != nil {
		return nil, err
	}
	return unit.NewUnitState(loadState, activeState, subState, ""), nil
}

func (m *SystemdUnitManager) getUnitStates(name string) (string, string, string, error) {
	info, err := m.systemd.GetUnitProperties(name)

	if err != nil {
		return "", "", "", err
	}
	loadState := info["LoadState"].(string)
	activeState := info["ActiveState"].(string)
	subState := info["SubState"].(string)
	return loadState, activeState, subState, nil
}

func (m *SystemdUnitManager) startUnit(name string) {
	if stat, err := m.systemd.StartUnit(name, "replace"); err != nil {
		log.Errorf("Failed to start systemd unit %s: %v", name, err)
	} else {
		log.Infof("Started systemd unit %s(%s)", name, stat)
	}
}

func (m *SystemdUnitManager) stopUnit(name string) {
	if stat, err := m.systemd.StopUnit(name, "replace"); err != nil {
		log.Errorf("Failed to stop systemd unit %s: %v", name, err)
	} else {
		log.Infof("Stopped systemd unit %s(%s)", name, stat)
	}
}

func (m *SystemdUnitManager) readUnit(name string) (string, error) {
	path := m.getUnitFilePath(name)
	contents, err := ioutil.ReadFile(path)
	if err == nil {
		return string(contents), nil
	}
	return "", fmt.Errorf("no unit file at local path %s", path)
}

func (m *SystemdUnitManager) unitRequiresDaemonReload(name string) bool {
	prop, err := m.systemd.GetUnitProperty(name, "NeedDaemonReload")
	if prop == nil || err != nil {
		return false
	}

	return prop.Value.Value().(bool)
}

func (m *SystemdUnitManager) daemonReload() error {
	log.Infof("Instructing systemd to reload units")
	return m.systemd.Reload()
}

// Units enumerates all files recognized as valid systemd units in
// this manager's units directory.
func (m *SystemdUnitManager) Units() (units []string, err error) {
	fis, err := ioutil.ReadDir(m.UnitsDir)
	if err != nil {
		return
	}

	for _, fi := range fis {
		name := fi.Name()
		if !unit.RecognizedUnitType(name) {
			log.Warningf("Found unrecognized file in %s, ignoring", path.Join(m.UnitsDir, name))
			continue
		}
		units = append(units, name)
	}
	return
}

func (m *SystemdUnitManager) writeUnit(name string, contents string) error {
	log.Infof("Writing systemd unit %s", name)

	ufPath := m.getUnitFilePath(name)
	err := ioutil.WriteFile(ufPath, []byte(contents), os.FileMode(0644))
	if err != nil {
		return err
	}

	_, err = m.systemd.LinkUnitFiles([]string{ufPath}, true, true)
	return err
}

func (m *SystemdUnitManager) removeUnit(name string) {
	log.Infof("Removing systemd unit %s", name)

	m.systemd.DisableUnitFiles([]string{name}, true)

	ufPath := m.getUnitFilePath(name)
	os.Remove(ufPath)
}

func (m *SystemdUnitManager) getUnitFilePath(name string) string {
	return path.Join(m.UnitsDir, name)
}
