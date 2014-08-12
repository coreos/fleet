package client

import (
	"net/http"

	"github.com/coreos/fleet/Godeps/_workspace/src/code.google.com/p/google-api-go-client/googleapi"
	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"

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

		machines = append(machines, schema.MapSchemaToMachineStates(page.Machines)...)

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

func (c *HTTPClient) jobs() ([]job.Job, error) {
	var jobs []job.Job
	call := c.svc.Units.List()
	for call != nil {
		page, err := call.Do()
		if err != nil {
			return nil, err
		}

		units, err := schema.MapSchemaToJobs(page.Units)
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
	jobs, err := c.jobs()
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

func (c *HTTPClient) ScheduledUnit(name string) (*job.ScheduledUnit, error) {
	j, err := c.job(name)
	if err != nil || j == nil {
		return nil, err
	}
	su := job.ScheduledUnit{
		Name:            j.Name,
		State:           j.State,
		TargetMachineID: j.TargetMachineID,
	}
	return &su, err
}

func (c *HTTPClient) Schedule() ([]job.ScheduledUnit, error) {
	jobs, err := c.jobs()
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
	jobs, err := c.jobs()
	if err != nil {
		return nil, err
	}
	var states []*unit.UnitState
	for _, j := range jobs {
		states = append(states, j.UnitState)
	}
	return states, nil
}

func (c *HTTPClient) job(name string) (*job.Job, error) {
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

	return schema.MapSchemaToJob(u)
}

func (c *HTTPClient) DestroyUnit(name string) error {
	return c.svc.Units.Delete(name).Do()
}

func (c *HTTPClient) CreateUnit(u *job.Unit) error {
	opts := make([]*schema.UnitOption, len(u.Unit.Options))
	for i, opt := range u.Unit.Options {
		opts[i] = &schema.UnitOption{
			Section: opt.Section,
			Name:    opt.Name,
			Value:   opt.Value,
		}
	}
	req := schema.DesiredUnitState{
		Name:         u.Name,
		DesiredState: string(job.JobStateInactive),
		Options:      opts,
	}
	return c.svc.Units.Set(u.Name, &req).Do()
}

func (c *HTTPClient) SetUnitTargetState(name string, state job.JobState) error {
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
