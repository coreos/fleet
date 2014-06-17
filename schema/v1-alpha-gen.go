// Package fleet provides access to the Fleet API.
//
// See http://github.com/coreos/fleet
//
// Usage example:
//
//   import "code.google.com/p/google-api-go-client/fleet/v1-alpha"
//   ...
//   fleetService, err := fleet.New(oauthHttpClient)
package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/coreos/fleet/third_party/code.google.com/p/google-api-go-client/googleapi"
)

// Always reference these packages, just in case the auto-generated code
// below doesn't.
var _ = bytes.NewBuffer
var _ = strconv.Itoa
var _ = fmt.Sprintf
var _ = json.NewDecoder
var _ = io.Copy
var _ = url.Parse
var _ = googleapi.Version
var _ = errors.New
var _ = strings.Replace

const apiId = "fleet:v1-alpha"
const apiName = "fleet"
const apiVersion = "v1-alpha"
const basePath = "http://example.com/v1-alpha/"

func New(client *http.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	s := &Service{client: client, BasePath: basePath}
	s.Machines = NewMachinesService(s)
	return s, nil
}

type Service struct {
	client   *http.Client
	BasePath string // API endpoint base URL

	Machines *MachinesService
}

func NewMachinesService(s *Service) *MachinesService {
	rs := &MachinesService{s: s}
	return rs
}

type MachinesService struct {
	s *Service
}

type Machine struct {
	Id string `json:"id,omitempty"`

	Metadata map[string]string `json:"metadata,omitempty"`

	PrimaryIP string `json:"primaryIP,omitempty"`
}

type MachinePage struct {
	Machines []*Machine `json:"machines,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`
}

// method id "fleet.Machine.List":

type MachinesListCall struct {
	s    *Service
	opt_ map[string]interface{}
}

// List: Retrieve a page of Machine objects.
func (r *MachinesService) List() *MachinesListCall {
	c := &MachinesListCall{s: r.s, opt_: make(map[string]interface{})}
	return c
}

// NextPageToken sets the optional parameter "nextPageToken":
func (c *MachinesListCall) NextPageToken(nextPageToken string) *MachinesListCall {
	c.opt_["nextPageToken"] = nextPageToken
	return c
}

func (c *MachinesListCall) Do() (*MachinePage, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["nextPageToken"]; ok {
		params.Set("nextPageToken", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "machines")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *MachinePage
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Retrieve a page of Machine objects.",
	//   "httpMethod": "GET",
	//   "id": "fleet.Machine.List",
	//   "parameters": {
	//     "nextPageToken": {
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "machines",
	//   "response": {
	//     "$ref": "MachinePage"
	//   }
	// }

}
