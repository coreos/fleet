// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package systemd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"github.com/coreos/go-systemd/dbus"

	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/unit"
)

const (
	DefaultUnitsDirectory = "/run/fleet/units/"
)

type systemdUnitManager struct {
	systemd  *dbus.Conn
	unitsDir string

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

	hashes, err := hashUnitFiles(uDir)
	if err != nil {
		return nil, err
	}

	mgr := systemdUnitManager{
		systemd:  systemd,
		unitsDir: uDir,
		hashes:   hashes,
		mutex:    sync.RWMutex{},
	}
	return &mgr, nil
}

func hashUnitFiles(dir string) (map[string]unit.Hash, error) {
	uNames, err := lsUnitsDir(dir)
	if err != nil {
		return nil, err
	}

	hMap := make(map[string]unit.Hash)
	for _, uName := range uNames {
		h, err := hashUnitFile(path.Join(dir, uName))
		if err != nil {
			return nil, err
		}

		hMap[uName] = h
	}

	return hMap, nil
}

func hashUnitFile(loc string) (unit.Hash, error) {
	b, err := ioutil.ReadFile(loc)
	if err != nil {
		return unit.Hash{}, err
	}

	uf, err := unit.NewUnitFile(string(b))
	if err != nil {
		return unit.Hash{}, err
	}

	return uf.Hash(), nil
}

// Load writes the given Unit to disk, subscribing to relevant dbus
// events and caching the Unit's Hash.
func (m *systemdUnitManager) Load(name string, u unit.UnitFile) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	err := m.writeUnit(name, u.String())
	if err != nil {
		return err
	}
	m.hashes[name] = u.Hash()
	return nil
}

// Unload removes the indicated unit from the filesystem, deletes its
// associated Hash from the cache and clears its unit status in systemd
func (m *systemdUnitManager) Unload(name string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.hashes, name)
	m.removeUnit(name)
}

// TriggerStart asynchronously starts the unit identified by the given name.
// This function does not block for the underlying unit to actually start.
func (m *systemdUnitManager) TriggerStart(name string) {
	jobID, err := m.systemd.StartUnit(name, "replace", nil)
	if err == nil {
		log.Infof("Triggered systemd unit %s start: job=%d", name, jobID)
	} else {
		log.Errorf("Failed to trigger systemd unit %s start: %v", name, err)
	}
}

// TriggerStop asynchronously starts the unit identified by the given name.
// This function does not block for the underlying unit to actually stop.
func (m *systemdUnitManager) TriggerStop(name string) {
	jobID, err := m.systemd.StopUnit(name, "replace", nil)
	if err == nil {
		log.Infof("Triggered systemd unit %s stop: job=%d", name, jobID)
	} else {
		log.Errorf("Failed to trigger systemd unit %s stop: %v", name, err)
	}
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

func (m *systemdUnitManager) readUnit(name string) (string, error) {
	path := m.getUnitFilePath(name)
	contents, err := ioutil.ReadFile(path)
	if err == nil {
		return string(contents), nil
	}
	return "", fmt.Errorf("no unit file at local path %s", path)
}

func (m *systemdUnitManager) ReloadUnitFiles() error {
	log.Infof("Instructing systemd to reload units")
	return m.systemd.Reload()
}

// Units enumerates all files recognized as valid systemd units in
// this manager's units directory.
func (m *systemdUnitManager) Units() ([]string, error) {
	return lsUnitsDir(m.unitsDir)

}

func (m *systemdUnitManager) GetUnitStates(filter pkg.Set) (map[string]*unit.UnitState, error) {
	// Unfortunately we need to lock for the entire operation to ensure we
	// have a consistent view of the hashes. Otherwise, Load/Unload
	// operations could mutate the hashes before we've retrieved the state
	// for every unit in the filter, since they won't necessarily all be
	// present in the initial ListUnits() call.
	fallback := false

	m.mutex.Lock()
	defer m.mutex.Unlock()
	dbusStatuses, err := m.systemd.ListUnitsByNames(filter.Values())

	if err != nil {
		fallback = true
		log.Debugf("ListUnitsByNames is not implemented in your systemd version (requires at least systemd 230), fallback to ListUnits: %v", err)
		dbusStatuses, err = m.systemd.ListUnits()
		if err != nil {
			return nil, err
		}
	}

	states := make(map[string]*unit.UnitState)
	for _, dus := range dbusStatuses {
		if fallback && !filter.Contains(dus.Name) {
			// If filter could not be applied on DBus side, we will filter unit files here
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

	// grab data on subscribed units that didn't show up in ListUnits in fallback mode, most
	// likely due to being inactive
	if fallback {
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
	}

	return states, nil
}

func (m *systemdUnitManager) writeUnit(name string, contents string) error {
	bContents := []byte(contents)
	log.Infof("Writing systemd unit %s (%db)", name, len(bContents))

	ufPath := m.getUnitFilePath(name)
	err := ioutil.WriteFile(ufPath, bContents, os.FileMode(0644))
	if err != nil {
		return err
	}

	_, err = m.systemd.LinkUnitFiles([]string{ufPath}, true, true)
	return err
}

func (m *systemdUnitManager) removeUnit(name string) {
	log.Infof("Removing systemd unit %s", name)

	m.systemd.DisableUnitFiles([]string{name}, true)
	m.systemd.ResetFailedUnit(name)

	ufPath := m.getUnitFilePath(name)
	os.Remove(ufPath)
}

func (m *systemdUnitManager) getUnitFilePath(name string) string {
	return path.Join(m.unitsDir, name)
}

func lsUnitsDir(dir string) ([]string, error) {
	filterFunc := func(name string) bool {
		if !unit.RecognizedUnitType(name) {
			log.Warningf("Found unrecognized file in %s, ignoring", path.Join(dir, name))
			return true
		}

		return false
	}

	return pkg.ListDirectory(dir, filterFunc)
}
