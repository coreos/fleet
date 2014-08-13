package client

import (
	"net/http"

	"github.com/coreos/fleet/Godeps/_workspace/src/code.google.com/p/google-api-go-client/googleapi"
	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	gsunit "github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-systemd/unit"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
)

func NewHTTPClient(c *http.Client) (API, error) {
	svc, err := schema.New(c)
	if err != nil {
		return nil, err
	}
	return &HTTPClient{svc: svc}, nil
}

type HTTPClient struct {
	svc *schema.Service

	//NOTE(bcwaldon): This is only necessary until the API interface
	// is fully implemented by HTTPClient
	API
}

func (c *HTTPClient) Machines() ([]machine.MachineState, error) {
	machines := make([]machine.MachineState, 0)
	call := c.svc.Machines.List()
	for call != nil {
		page, err := call.Do()
		if err != nil {
			return nil, err
		}

		machines = append(machines, mapMachinePageToMachineStates(page.Machines)...)

		if len(page.NextPageToken) > 0 {
			call = c.svc.Machines.List()
			call.NextPageToken(page.NextPageToken)
		} else {
			call = nil
		}
	}
	return machines, nil
}

func mapMachinePageToMachineStates(entities []*schema.Machine) []machine.MachineState {
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

func (c *HTTPClient) Jobs() ([]job.Job, error) {
	machines, err := c.Machines()
	if err != nil {
		return nil, err
	}

	mm := make(map[string]*machine.MachineState, len(machines))
	for i, _ := range machines {
		m := machines[i]
		mm[m.ID] = &m
	}

	var jobs []job.Job
	call := c.svc.Units.List()
	for call != nil {
		page, err := call.Do()
		if err != nil {
			return nil, err
		}

		units, err := mapUnitPageToJobs(page.Units, mm)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, units...)

		if len(page.NextPageToken) > 0 {
			call = c.svc.Units.List()
			call.NextPageToken(page.NextPageToken)
		} else {
			call = nil
		}
	}
	return jobs, nil
}

func (c *HTTPClient) Units() ([]job.Unit, error) {
	jobs, err := c.Jobs()
	if err != nil {
		return nil, err
	}
	var jus []job.Unit
	for _, j := range jobs {
		ju := job.Unit{
			Name: j.Name,
			Unit: j.Unit,
		}
		jus = append(jus, ju)
	}
	return jus, nil
}

func (c *HTTPClient) Schedule() ([]job.ScheduledUnit, error) {
	jobs, err := c.Jobs()
	if err != nil {
		return nil, err
	}
	var sched []job.ScheduledUnit
	for _, j := range jobs {
		su := job.ScheduledUnit{
			Name:            j.Name,
			State:           j.State,
			TargetMachineID: j.TargetMachineID,
		}
		sched = append(sched, su)
	}
	return sched, nil
}

func (c *HTTPClient) UnitStates() ([]*unit.UnitState, error) {
	jobs, err := c.Jobs()
	if err != nil {
		return nil, err
	}
	var states []*unit.UnitState
	for _, j := range jobs {
		states = append(states, j.UnitState)
	}
	return states, nil
}

func (c *HTTPClient) Job(name string) (*job.Job, error) {
	u, err := c.svc.Units.Get(name).Do()
	if err != nil {
		if is404(err) {
			err = nil
		}
		return nil, err
	}

	if u == nil {
		return nil, nil
	}

	machines, err := c.Machines()
	if err != nil {
		return nil, err
	}

	mm := make(map[string]*machine.MachineState, len(machines))
	for i, _ := range machines {
		m := machines[i]
		mm[m.ID] = &m
	}

	return mapUnitToJob(u, mm)
}

func mapUnitPageToJobs(entities []*schema.Unit, mm map[string]*machine.MachineState) ([]job.Job, error) {
	jobs := make([]job.Job, len(entities))
	for i, _ := range entities {
		entity := entities[i]
		j, err := mapUnitToJob(entity, mm)
		if err != nil {
			return nil, err
		}
		if j != nil {
			jobs[i] = *j
		}
	}

	return jobs, nil
}

func mapUnitToJob(entity *schema.Unit, mm map[string]*machine.MachineState) (*job.Job, error) {
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
		Name:        entity.Name,
		State:       &js,
		TargetState: ts,
		Unit:        *u,
	}

	// populate a UnitState object only if the entity
	// is actually reporting relevant data
	if entity.Systemd != nil {
		j.UnitState = &unit.UnitState{
			LoadState:   entity.Systemd.LoadState,
			ActiveState: entity.Systemd.ActiveState,
			SubState:    entity.Systemd.SubState,
			UnitName:    j.Name,
		}
		if len(entity.Systemd.MachineID) > 0 {
			j.UnitState.MachineID = entity.Systemd.MachineID
		}
	}

	return &j, nil
}

func (c *HTTPClient) DestroyJob(name string) error {
	return c.svc.Units.Delete(name).Do()
}

func (c *HTTPClient) CreateJob(j *job.Job) error {
	opts := make([]*schema.UnitOption, len(j.Unit.Options))
	for i, opt := range j.Unit.Options {
		opts[i] = &schema.UnitOption{
			Section: opt.Section,
			Name:    opt.Name,
			Value:   opt.Value,
		}
	}
	req := schema.DesiredUnitState{
		Name:         j.Name,
		DesiredState: string(job.JobStateInactive),
		Options:      opts,
	}
	return c.svc.Units.Set(j.Name, &req).Do()
}

func (c *HTTPClient) SetJobTargetState(name string, state job.JobState) error {
	req := schema.DesiredUnitState{
		Name:         name,
		DesiredState: string(state),
	}
	return c.svc.Units.Set(name, &req).Do()
}

//NOTE(bcwaldon): This is only temporary until a better version negotiation mechanism is in place
func (c *HTTPClient) LatestVersion() (*semver.Version, error) {
	return semver.NewVersion("0.0.0")
}

func is404(err error) bool {
	googerr, ok := err.(*googleapi.Error)
	return ok && googerr.Code == http.StatusNotFound
}
