package registry

import (
	"path"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
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
	return path.Join(r.keyPrefix, statePrefix, jobName)
}

// unitStatesNamespace generates a keypath of a namespace containing all
// UnitState objects for a particular job
func (r *EtcdRegistry) unitStatesNamespace(jobName string) string {
	return path.Join(r.keyPrefix, statesPrefix, jobName)
}

// unitStatePath generates a keypath where the UnitState object for a given
// machine ID + jobName combination is stored
func (r *EtcdRegistry) unitStatePath(machID, jobName string) string {
	return path.Join(r.unitStatesNamespace(jobName), machID)
}

// getUnitState retrieves the current UnitState of the provided Job's Unit
func (r *EtcdRegistry) getUnitState(jobName string) *unit.UnitState {
	// TODO(jonboulle): deal with multiple UnitStates
	legacyKey := r.legacyUnitStatePath(jobName)
	req := etcd.Get{
		Key:       legacyKey,
		Recursive: true,
	}
	resp, err := r.etcd.Do(&req)

	if err != nil {
		if !isKeyNotFound(err) {
			log.Errorf("Error retrieving UnitState(%s): %v", jobName, err)
		}
		return nil
	}

	var usm unitStateModel
	if err := unmarshal(resp.Node.Value, &usm); err != nil {
		log.Errorf("Error unmarshalling UnitState(%s): %v", jobName, err)
		return nil
	}

	return modelToUnitState(&usm)
}

// SaveUnitState persists the given UnitState to the Registry
func (r *EtcdRegistry) SaveUnitState(jobName string, unitState *unit.UnitState) {
	usm := unitStateToModel(unitState)
	if usm == nil {
		log.Errorf("Unable to save nil UnitState model")
		return
	}

	json, err := marshal(usm)
	if err != nil {
		log.Errorf("Error marshalling UnitState: %v", err)
		return
	}

	legacyKey := r.legacyUnitStatePath(jobName)
	req := etcd.Set{
		Key:   legacyKey,
		Value: json,
	}
	r.etcd.Do(&req)

	newKey := r.unitStatePath(unitState.MachineID, jobName)
	req = etcd.Set{
		Key:   newKey,
		Value: json,
	}
	r.etcd.Do(&req)
}

// Delete the state from the Registry for the given Job's Unit
func (r *EtcdRegistry) RemoveUnitState(jobName string) error {
	// TODO(jonboulle): consider https://github.com/coreos/fleet/issues/465
	legacyKey := r.legacyUnitStatePath(jobName)
	req := etcd.Delete{
		Key: legacyKey,
	}
	_, err := r.etcd.Do(&req)
	if err != nil && !isKeyNotFound(err) {
		return err
	}

	// TODO(jonboulle): deal properly with multiple states
	newKey := r.unitStatesNamespace(jobName)
	req = etcd.Delete{
		Key:       newKey,
		Recursive: true,
	}
	_, err = r.etcd.Do(&req)
	if err != nil && !isKeyNotFound(err) {
		return err
	}
	return nil
}

type unitStateModel struct {
	LoadState    string                `json:"loadState"`
	ActiveState  string                `json:"activeState"`
	SubState     string                `json:"subState"`
	MachineState *machine.MachineState `json:"machineState"`
	UnitHash     string                `json:"unitHash"`
}

func modelToUnitState(usm *unitStateModel) *unit.UnitState {
	if usm == nil {
		return nil
	}

	us := unit.UnitState{
		LoadState:   usm.LoadState,
		ActiveState: usm.ActiveState,
		SubState:    usm.SubState,
		UnitHash:    usm.UnitHash,
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

	usm := unitStateModel{
		LoadState:   us.LoadState,
		ActiveState: us.ActiveState,
		SubState:    us.SubState,
	}

	if us.MachineID != "" {
		usm.MachineState = &machine.MachineState{ID: us.MachineID}
	}

	// Refuse to create a UnitState without a Hash
	if len(us.UnitHash) == 0 {
		return nil
	}
	usm.UnitHash = us.UnitHash

	return &usm
}
