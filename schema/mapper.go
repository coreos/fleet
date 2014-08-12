package schema

import (
	gsunit "github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-systemd/unit"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func MapUnitFileToSchema(u *unit.UnitFile) []*UnitOption {
	sopts := make([]*UnitOption, len(u.Options))
	for i, opt := range u.Options {
		sopts[i] = &UnitOption{
			Section: opt.Section,
			Name:    opt.Name,
			Value:   opt.Value,
		}
	}
	return sopts
}

func MapSchemaToUnitFile(sopts []*UnitOption) *unit.UnitFile {
	opts := make([]*gsunit.UnitOption, len(sopts))
	for i, sopt := range sopts {
		opts[i] = &gsunit.UnitOption{
			Section: sopt.Section,
			Name:    sopt.Name,
			Value:   sopt.Value,
		}
	}

	return unit.NewUnitFromOptions(opts)
}

func MapJobToSchema(j *job.Job) (*Unit, error) {
	su := Unit{
		Name:            j.Name,
		Options:         MapUnitFileToSchema(&j.Unit),
		TargetMachineID: j.TargetMachineID,
		DesiredState:    string(j.TargetState),
	}

	if j.State != nil {
		su.CurrentState = string(*(j.State))
	}

	if j.UnitState != nil {
		su.Systemd = &SystemdState{
			LoadState:   j.UnitState.LoadState,
			ActiveState: j.UnitState.ActiveState,
			SubState:    j.UnitState.SubState,
		}
		if j.UnitState.MachineID != "" {
			su.Systemd.MachineID = j.UnitState.MachineID
		}
	}

	return &su, nil
}

func MapSchemaToJob(entity *Unit) (*job.Job, error) {
	opts := make([]*gsunit.UnitOption, len(entity.Options))
	for i, eopt := range entity.Options {
		opts[i] = &gsunit.UnitOption{
			Section: eopt.Section,
			Name:    eopt.Name,
			Value:   eopt.Value,
		}
	}
	u := unit.NewUnitFromOptions(opts)
	js := job.JobState(entity.CurrentState)
	ts := job.JobState(entity.DesiredState)
	j := job.Job{
		Name:  entity.Name,
		TargetState: ts,
		State: &js,
		Unit:  *u,
	}

	// populate a UnitState object only if the entity
	// is actually reporting relevant data
	if entity.Systemd != nil {
		j.UnitState = &unit.UnitState{
			LoadState:   entity.Systemd.LoadState,
			ActiveState: entity.Systemd.ActiveState,
			SubState:    entity.Systemd.SubState,
		}
		if len(entity.Systemd.MachineID) > 0 {
			j.UnitState.MachineID = entity.Systemd.MachineID
		}
	}

	return &j, nil
}

func MapSchemaToJobs(entities []*Unit) ([]job.Job, error) {
	jobs := make([]job.Job, len(entities))
	for i, _ := range entities {
		entity := entities[i]
		j, err := MapSchemaToJob(entity)
		if err != nil {
			return nil, err
		}
		if j != nil {
			jobs[i] = *j
		}
	}

	return jobs, nil
}

func MapMachineStateToSchema(ms *machine.MachineState) *Machine {
	sm := Machine{
		Id:        ms.ID,
		PrimaryIP: ms.PublicIP,
	}

	sm.Metadata = make(map[string]string, len(ms.Metadata))
	for k, v := range ms.Metadata {
		sm.Metadata[k] = v
	}

	return &sm
}

func MapSchemaToMachineStates(entities []*Machine) []machine.MachineState {
	machines := make([]machine.MachineState, len(entities))
	for i, _ := range entities {
		me := entities[i]

		ms := machine.MachineState{
			ID:       me.Id,
			PublicIP: me.PrimaryIP,
		}

		ms.Metadata = make(map[string]string, len(me.Metadata))
		for k, v := range me.Metadata {
			ms.Metadata[k] = v
		}

		machines[i] = ms
	}

	return machines
}
