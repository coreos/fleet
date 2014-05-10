package systemd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-systemd/dbus"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

const (
	DefaultUnitsDirectory = "/run/fleet/units/"
)

type SystemdManager struct {
	systemd  *dbus.Conn
	Machine  *machine.Machine
	UnitsDir string

	subscriptions *dbus.SubscriptionSet
	stop          chan bool
}

func NewSystemdManager(machine *machine.Machine, uDir string) (*SystemdManager, error) {
	systemd, err := dbus.New()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(uDir, os.FileMode(0755)); err != nil {
		return nil, err
	}

	return &SystemdManager{systemd, machine, uDir, systemd.NewSubscriptionSet(), nil}, nil
}

func (m *SystemdManager) MarshalJSON() ([]byte, error) {
	data := struct {
		DBUSSubscriptions []string
	}{
		DBUSSubscriptions: m.subscriptions.Values(),
	}
	return json.Marshal(data)
}

// Publish is a long-running function that streams dbus events through
// a translation layer and on to the EventBus
func (m *SystemdManager) Publish(bus *event.EventBus, stopchan chan bool) {
	m.systemd.Subscribe()

	changechan, errchan := m.subscriptions.Subscribe()

	stream := NewEventStream()
	stream.Stream(changechan, bus.Channel)

	for true {
		select {
		case <-stopchan:
			break
		case err := <-errchan:
			var errString string
			if err != nil {
				errString = err.Error()
			} else {
				errString = "N/A"
			}
			log.Errorf("Received error from dbus: err=%s", errString)
		}
	}

	stream.Close()
	m.systemd.Unsubscribe()
}

// Load writes the given Unit to disk, subscribing to relevant dbus
// events, and, if necessary, instructing the systemd daemon to reload.
func (m *SystemdManager) Load(name string, u unit.Unit) error {
	err := m.writeUnit(name, u.String())
	if err != nil {
		return err
	}

	m.subscriptions.Add(name)

	if !m.unitRequiresDaemonReload(name) {
		return nil
	}

	return m.daemonReload()
}

// Unload removes the indicated unit from the filesystem, unsubscribing
// from relevant dbus events.
func (m *SystemdManager) Unload(name string) {
	m.subscriptions.Remove(name)
	m.removeUnit(name)
}

// Start starts the unit identified by the given name
func (m *SystemdManager) Start(name string) {
	m.startUnit(name)
}

// Stop stops the unit identified by the given name
func (m *SystemdManager) Stop(name string) {
	m.stopUnit(name)
}

// GetUnitState generates a UnitState object representing the
// current state of a Unit
func (m *SystemdManager) GetUnitState(name string) (*unit.UnitState, error) {
	loadState, activeState, subState, err := m.getUnitStates(name)
	if err != nil {
		return nil, err
	}
	ms := m.Machine.State()
	return unit.NewUnitState(loadState, activeState, subState, &ms), nil
}

func (m *SystemdManager) getUnitStates(name string) (string, string, string, error) {
	info, err := m.systemd.GetUnitProperties(name)

	if err != nil {
		return "", "", "", err
	} else {
		loadState := info["LoadState"].(string)
		activeState := info["ActiveState"].(string)
		subState := info["SubState"].(string)
		return loadState, activeState, subState, nil
	}
}

func (m *SystemdManager) startUnit(name string) {
	if stat, err := m.systemd.StartUnit(name, "replace"); err != nil {
		log.Errorf("Failed to start systemd unit %s: %v", name, err)
	} else {
		log.Infof("Started systemd unit %s(%s)", name, stat)
	}
}

func (m *SystemdManager) stopUnit(name string) {
	if stat, err := m.systemd.StopUnit(name, "replace"); err != nil {
		log.Errorf("Failed to stop systemd unit %s: %v", name, err)
	} else {
		log.Infof("Stopped systemd unit %s(%s)", name, stat)
	}
}

func (m *SystemdManager) readUnit(name string) (string, error) {
	path := m.getUnitFilePath(name)
	contents, err := ioutil.ReadFile(path)
	if err == nil {
		return string(contents), nil
	} else {
		return "", errors.New(fmt.Sprintf("No unit file at local path %s", path))
	}
}

func (m *SystemdManager) unitRequiresDaemonReload(name string) bool {
	prop, err := m.systemd.GetUnitProperty(name, "NeedDaemonReload")
	if prop == nil || err != nil {
		return false
	}

	return prop.Value.Value().(bool)
}

func (m *SystemdManager) daemonReload() error {
	log.Infof("Instructing systemd to reload units")
	return m.systemd.Reload()
}

// Units enumerates all files recognized as valid systemd units in
// this manager's units directory.
func (m *SystemdManager) Units() (units []string, err error) {
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

func (m *SystemdManager) writeUnit(name string, contents string) error {
	log.Infof("Writing systemd unit %s", name)

	ufPath := m.getUnitFilePath(name)
	err := ioutil.WriteFile(ufPath, []byte(contents), os.FileMode(0644))
	if err != nil {
		return err
	}

	_, err = m.systemd.LinkUnitFiles([]string{ufPath}, true, true)
	return err
}

func (m *SystemdManager) removeUnit(name string) {
	log.Infof("Removing systemd unit %s", name)

	m.systemd.DisableUnitFiles([]string{name}, true)

	ufPath := m.getUnitFilePath(name)
	os.Remove(ufPath)
}

func (m *SystemdManager) getUnitFilePath(name string) string {
	return path.Join(m.UnitsDir, name)
}
