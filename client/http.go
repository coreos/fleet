package client

import (
	"net/http"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
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

func (c *HTTPClient) GetActiveMachines() ([]machine.MachineState, error) {
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

//NOTE(bcwaldon): This is only temporary until a better version negotiation mechanism is in place
func (c *HTTPClient) GetLatestVersion() (*semver.Version, error) {
	return semver.NewVersion("0.0.0")
}
