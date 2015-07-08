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

package registry

import (
	"path"
	"sort"
	"time"

	etcd "github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/etcd/client"

	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

const (
	// Legacy namespace for unit states
	statePrefix = "/state/"
	// Namespace for unit states stored per-machine
	statesPrefix = "/states/"
)

// legacyUnitStatePath returns the path where UnitState objects were formerly
// reported before being moved to a machine-specific namespace
// https://github.com/coreos/fleet/issues/638
func (r *EtcdRegistry) legacyUnitStatePath(jobName string) string {
	return r.prefixed(statePrefix, jobName)
}

// unitStatesNamespace generates a keypath of a namespace containing all
// UnitState objects for a particular job
func (r *EtcdRegistry) unitStatesNamespace(jobName string) string {
	return r.prefixed(statesPrefix, jobName)
}

// unitStatePath generates a keypath where the UnitState object for a given
// machine ID + jobName combination is stored
func (r *EtcdRegistry) unitStatePath(machID, jobName string) string {
	return path.Join(r.unitStatesNamespace(jobName), machID)
}

// UnitStates returns a list of all UnitStates stored in the registry, sorted
// by unit name and then machine ID.
func (r *EtcdRegistry) UnitStates() (states []*unit.UnitState, err error) {
	var mus map[MUSKey]*unit.UnitState
	mus, err = r.statesByMUSKey()
	if err != nil {
		return
	}

	var sorted MUSKeys
	for key, _ := range mus {
		sorted = append(sorted, key)
	}
	sort.Sort(sorted)

	for _, key := range sorted {
		states = append(states, mus[key])
	}

	return
}

// MUSKey is used to index UnitStates by name + machineID
type MUSKey struct {
	name   string
	machID string
}

// MUSKeys provides for sorting of UnitStates by their MUSKey
type MUSKeys []MUSKey

func (mk MUSKeys) Len() int { return len(mk) }
func (mk MUSKeys) Less(i, j int) bool {
	mi := mk[i]
	mj := mk[j]
	return mi.name < mj.name || (mi.name == mj.name && mi.machID < mj.machID)
}
func (mk MUSKeys) Swap(i, j int) { mk[i], mk[j] = mk[j], mk[i] }

// statesByMUSKey returns a map of all UnitStates stored in the registry indexed by MUSKey
func (r *EtcdRegistry) statesByMUSKey() (map[MUSKey]*unit.UnitState, error) {
	mus := make(map[MUSKey]*unit.UnitState)
	key := r.prefixed(statesPrefix)
	opts := &etcd.GetOptions{
		Recursive: true,
	}
	res, err := r.kAPI.Get(r.ctx(), key, opts)
	if err != nil && !isEtcdError(err, etcd.ErrorCodeKeyNotFound) {
		return nil, err
	}
	if res != nil {
		for _, dir := range res.Node.Nodes {
			_, name := path.Split(dir.Key)
			for _, node := range dir.Nodes {
				_, machID := path.Split(node.Key)
				var usm unitStateModel
				if err := unmarshal(node.Value, &usm); err != nil {
					log.Errorf("Error unmarshalling UnitState(%s) from Machine(%s): %v", name, machID, err)
					continue
				}
				us := modelToUnitState(&usm, name)
				if us != nil {
					key := MUSKey{name, machID}
					mus[key] = us
				}
			}
		}
	}
	return mus, nil
}

// getUnitState retrieves the current UnitState, if any exists, for the
// given unit that originates from the indicated machine
func (r *EtcdRegistry) getUnitState(uName, machID string) (*unit.UnitState, error) {
	key := r.unitStatePath(machID, uName)
	res, err := r.kAPI.Get(r.ctx(), key, nil)
	if err != nil {
		if isEtcdError(err, etcd.ErrorCodeKeyNotFound) {
			err = nil
		}
		return nil, err
	}

	var usm unitStateModel
	if err := unmarshal(res.Node.Value, &usm); err != nil {
		return nil, err
	}

	return modelToUnitState(&usm, uName), nil
}

// SaveUnitState persists the given UnitState to the Registry
func (r *EtcdRegistry) SaveUnitState(jobName string, unitState *unit.UnitState, ttl time.Duration) {
	usm := unitStateToModel(unitState)
	if usm == nil {
		log.Errorf("Unable to save nil UnitState model")
		return
	}

	val, err := marshal(usm)
	if err != nil {
		log.Errorf("Error marshalling UnitState: %v", err)
		return
	}

	opts := &etcd.SetOptions{
		TTL: ttl,
	}

	legacyKey := r.legacyUnitStatePath(jobName)
	r.kAPI.Set(r.ctx(), legacyKey, val, opts)

	newKey := r.unitStatePath(unitState.MachineID, jobName)
	r.kAPI.Set(r.ctx(), newKey, val, opts)
}

// Delete the state from the Registry for the given Job's Unit
func (r *EtcdRegistry) RemoveUnitState(jobName string) error {
	// TODO(jonboulle): consider https://github.com/coreos/fleet/issues/465
	legacyKey := r.legacyUnitStatePath(jobName)
	_, err := r.kAPI.Delete(r.ctx(), legacyKey, nil)
	if err != nil && !isEtcdError(err, etcd.ErrorCodeKeyNotFound) {
		return err
	}

	// TODO(jonboulle): deal properly with multiple states
	newKey := r.unitStatesNamespace(jobName)
	opts := &etcd.DeleteOptions{
		Recursive: true,
	}
	_, err = r.kAPI.Delete(r.ctx(), newKey, opts)
	if err != nil && !isEtcdError(err, etcd.ErrorCodeKeyNotFound) {
		return err
	}
	return nil
}

type unitStateModel struct {
	LoadState            string                `json:"loadState"`
	ActiveState          string                `json:"activeState"`
	SubState             string                `json:"subState"`
	MachineState         *machine.MachineState `json:"machineState"`
	UnitHash             string                `json:"unitHash"`
	ActiveEnterTimestamp uint64                `json:"ActiveEnterTimestamp"`
}

func modelToUnitState(usm *unitStateModel, name string) *unit.UnitState {
	if usm == nil {
		return nil
	}

	us := unit.UnitState{
		LoadState:            usm.LoadState,
		ActiveState:          usm.ActiveState,
		SubState:             usm.SubState,
		UnitHash:             usm.UnitHash,
		UnitName:             name,
		ActiveEnterTimestamp: usm.ActiveEnterTimestamp,
	}

	if usm.MachineState != nil {
		us.MachineID = usm.MachineState.ID
	}

	return &us
}

func unitStateToModel(us *unit.UnitState) *unitStateModel {
	if us == nil {
		return nil
	}

	// Refuse to create a UnitState without a Hash
	// See https://github.com/coreos/fleet/issues/720
	//if len(us.UnitHash) == 0 {
	//	return nil
	//}

	usm := unitStateModel{
		LoadState:            us.LoadState,
		ActiveState:          us.ActiveState,
		SubState:             us.SubState,
		UnitHash:             us.UnitHash,
		ActiveEnterTimestamp: us.ActiveEnterTimestamp,
	}

	if us.MachineID != "" {
		usm.MachineState = &machine.MachineState{ID: us.MachineID}
	}

	return &usm
}
