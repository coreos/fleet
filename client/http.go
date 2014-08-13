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

func (c *HTTPClient) Units() ([]job.Unit, error) {
	var junits []job.Unit
	call := c.svc.Units.List()
	for call != nil {
		page, err := call.Do()
		if err != nil {
			return nil, err
		}

		junits = append(junits, schema.MapSchemaUnitsToUnits(page.Units)...)

		if len(page.NextPageToken) > 0 {
			call = c.svc.Units.List()
			call.NextPageToken(page.NextPageToken)
		} else {
			call = nil
		}
	}
	return junits, nil
}

func (c *HTTPClient) ScheduledUnit(name string) (*job.ScheduledUnit, error) {
	u, err := c.svc.Units.Get(name).Do()
	if err != nil || u == nil {
		return nil, err
	}
	su := schema.MapSchemaUnitToScheduledUnit(u)
	return su, err
}

func (c *HTTPClient) Schedule() ([]job.ScheduledUnit, error) {
	var sunits []job.ScheduledUnit
	call := c.svc.Units.List()
	for call != nil {
		page, err := call.Do()
		if err != nil {
			return nil, err
		}
		sunits = append(sunits, schema.MapSchemaUnitsToScheduledUnits(page.Units)...)

		if len(page.NextPageToken) > 0 {
			call = c.svc.Units.List()
			call.NextPageToken(page.NextPageToken)
		} else {
			call = nil
		}
	}
	return sunits, nil
}

func (c *HTTPClient) UnitStates() ([]*unit.UnitState, error) {
	var states []*unit.UnitState
	call := c.svc.UnitState.List()
	for call != nil {
		page, err := call.Do()
		if err != nil {
			return nil, err
		}

		states = append(states, schema.MapSchemaUnitStatesToUnitStates(page.States)...)

		if len(page.NextPageToken) > 0 {
			call = c.svc.UnitState.List()
			call.NextPageToken(page.NextPageToken)
		} else {
			call = nil
		}
	}
	return states, nil
}

func (c *HTTPClient) DestroyUnit(name string) error {
	return c.svc.Units.Delete(name).Do()
}

func (c *HTTPClient) CreateUnit(u *job.Unit) error {
	return c.svc.Units.Set(u.Name, schema.MapUnitToSchemaUnit(u, nil)).Do()
}

func (c *HTTPClient) SetUnitTargetState(name string, state job.JobState) error {
	u := schema.Unit{
		Name:         name,
		DesiredState: string(state),
	}
	return c.svc.Units.Set(name, &u).Do()
}

//NOTE(bcwaldon): This is only temporary until a better version negotiation mechanism is in place
func (c *HTTPClient) LatestVersion() (*semver.Version, error) {
	return semver.NewVersion("0.0.0")
}

func is404(err error) bool {
	googerr, ok := err.(*googleapi.Error)
	return ok && googerr.Code == http.StatusNotFound
}
