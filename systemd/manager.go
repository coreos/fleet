package systemd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-systemd/dbus"
	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/unit"
)

const (
	DefaultUnitsDirectory = "/run/fleet/units/"
)

type systemdUnitManager struct {
	systemd  *dbus.Conn
	UnitsDir string

	hashes map[string]unit.Hash
	mutex  sync.RWMutex
}

func NewSystemdUnitManager(uDir string) (*systemdUnitManager, error) {
	systemd, err := dbus.New()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(uDir, os.FileMode(0755)); err != nil {
		return nil, err
	}

	mgr := systemdUnitManager{
		systemd:  systemd,
		UnitsDir: uDir,
		hashes:   make(map[string]unit.Hash),
		mutex:    sync.RWMutex{},
	}
	return &mgr, nil
}

// Load writes the given Unit to disk, subscribing to relevant dbus
// events, caching the Unit's Hash, and, if necessary, instructing the systemd
// daemon to reload.
func (m *systemdUnitManager) Load(name string, u unit.Unit) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	err := m.writeUnit(name, u.String())
	if err != nil {
		return err
	}
	m.hashes[name] = u.Hash()
	return m.daemonReload()
}

// Unload removes the indicated unit from the filesystem, deletes its
// associated Hash from the cache, and unsubscribes from relevant dbus events.
func (m *systemdUnitManager) Unload(name string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.hashes, name)
	m.removeUnit(name)
	m.daemonReload()
}

// Start starts the unit identified by the given name
func (m *systemdUnitManager) Start(name string) {
	m.startUnit(name)
}

// Stop stops the unit identified by the given name
func (m *systemdUnitManager) Stop(name string) {
	m.stopUnit(name)
}

// GetUnitState generates a UnitState object representing the
// current state of a Unit
func (m *systemdUnitManager) GetUnitState(name string) (*unit.UnitState, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	us, err := m.getUnitState(name)
	if err != nil {
		return nil, err
	}
	if h, ok := m.hashes[name]; ok {
		us.UnitHash = h.String()
	}
	return us, nil
}

func (m *systemdUnitManager) getUnitState(name string) (*unit.UnitState, error) {
	info, err := m.systemd.GetUnitProperties(name)
	if err != nil {
		return nil, err
	}
	us := unit.UnitState{
		LoadState:   info["LoadState"].(string),
		ActiveState: info["ActiveState"].(string),
		SubState:    info["SubState"].(string),
	}
	return &us, nil
}

func (m *systemdUnitManager) startUnit(name string) {
	if stat, err := m.systemd.StartUnit(name, "replace"); err != nil {
		log.Errorf("Failed to start systemd unit %s: %v", name, err)
	} else {
		log.Infof("Started systemd unit %s(%s)", name, stat)
	}
}

func (m *systemdUnitManager) stopUnit(name string) {
	if stat, err := m.systemd.StopUnit(name, "replace"); err != nil {
		log.Errorf("Failed to stop systemd unit %s: %v", name, err)
	} else {
		log.Infof("Stopped systemd unit %s(%s)", name, stat)
	}
}

func (m *systemdUnitManager) readUnit(name string) (string, error) {
	path := m.getUnitFilePath(name)
	contents, err := ioutil.ReadFile(path)
	if err == nil {
		return string(contents), nil
	}
	return "", fmt.Errorf("no unit file at local path %s", path)
}

func (m *systemdUnitManager) unitRequiresDaemonReload(name string) bool {
	prop, err := m.systemd.GetUnitProperty(name, "NeedDaemonReload")
	if prop == nil || err != nil {
		return false
	}

	return prop.Value.Value().(bool)
}

func (m *systemdUnitManager) daemonReload() error {
	log.Infof("Instructing systemd to reload units")
	return m.systemd.Reload()
}

// Units enumerates all files recognized as valid systemd units in
// this manager's units directory.
func (m *systemdUnitManager) Units() (units []string, err error) {
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

func (m *systemdUnitManager) GetUnitStates(filter pkg.Set) (map[string]*unit.UnitState, error) {
	// Unfortunately we need to lock for the entire operation to ensure we
	// have a consistent view of the hashes. Otherwise, Load/Unload
	// operations could mutate the hashes before we've retrieved the state
	// for every unit in the filter, since they won't necessarily all be
	// present in the initial ListUnits() call.
	m.mutex.Lock()
	defer m.mutex.Unlock()
	dbusStatuses, err := m.systemd.ListUnits()

	if err != nil {
		return nil, err
	}

	states := make(map[string]*unit.UnitState)
	for _, dus := range dbusStatuses {
		if !filter.Contains(dus.Name) {
			continue
		}

		us := &unit.UnitState{
			LoadState:   dus.LoadState,
			ActiveState: dus.ActiveState,
			SubState:    dus.SubState,
		}
		if h, ok := m.hashes[dus.Name]; ok {
			us.UnitHash = h.String()
		}
		states[dus.Name] = us
	}

	// grab data on subscribed units that didn't show up in ListUnits, most
	// likely due to being inactive
	for _, name := range filter.Values() {
		if _, ok := states[name]; ok {
			continue
		}

		us, err := m.getUnitState(name)
		if err != nil {
			return nil, err
		}
		if h, ok := m.hashes[name]; ok {
			us.UnitHash = h.String()
		}
		states[name] = us
	}

	return states, nil
}

func (m *systemdUnitManager) writeUnit(name string, contents string) error {
	log.Infof("Writing systemd unit %s", name)

	ufPath := m.getUnitFilePath(name)
	err := ioutil.WriteFile(ufPath, []byte(contents), os.FileMode(0644))
	if err != nil {
		return err
	}

	_, err = m.systemd.LinkUnitFiles([]string{ufPath}, true, true)
	return err
}

func (m *systemdUnitManager) removeUnit(name string) {
	log.Infof("Removing systemd unit %s", name)

	m.systemd.DisableUnitFiles([]string{name}, true)

	ufPath := m.getUnitFilePath(name)
	os.Remove(ufPath)
}

func (m *systemdUnitManager) getUnitFilePath(name string) string {
	return path.Join(m.UnitsDir, name)
}
