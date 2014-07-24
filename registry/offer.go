package registry

import (
	"path"
	"strings"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/pkg"
)

const (
	offerPrefix = "offer"
)

// jobOfferModel is used for serializing and deserializing JobOffers stored in the Registry
type jobOfferModel struct {
	Job        jobModel
	MachineIDs []string
}

// CreateJobOffer attempts to store a JobOffer and a reference to its associated Job in the repository
func (r *EtcdRegistry) CreateJobOffer(jo *job.JobOffer) error {
	jom := jobOfferModel{
		Job: jobModel{
			Name:     jo.Job.Name,
			UnitHash: jo.Job.Unit.Hash(),
		},
		MachineIDs: jo.MachineIDs,
	}

	json, err := marshal(jom)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	req := etcd.Set{
		Key:   path.Join(r.keyPrefix, offerPrefix, jo.Job.Name, "object"),
		Value: json,
	}
	_, err = r.etcd.Do(&req)
	return err
}

// getJobOfferFromJSON hydrates a JobOffer from a JSON-encoded jobOfferModel
func (r *EtcdRegistry) getJobOfferFromJSON(val string) *job.JobOffer {
	var jom jobOfferModel
	if err := unmarshal(val, &jom); err != nil {
		return nil
	}

	j := r.getJobFromModel(jom.Job)
	if j == nil {
		return nil
	}

	jo := job.JobOffer{
		Job:        *j,
		MachineIDs: jom.MachineIDs,
	}

	return &jo
}

// Bids returns a list of machine IDs that have bid for the referenced Job
func (r *EtcdRegistry) Bids(jName string) (bids pkg.Set, err error) {
	bids = pkg.NewUnsafeSet()

	req := etcd.Get{
		Key:       path.Join(r.keyPrefix, offerPrefix, jName, "bids"),
		Recursive: true,
	}

	var resp *etcd.Result
	resp, err = r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return
	}

	for _, node := range resp.Node.Nodes {
		machID := path.Base(node.Key)
		bids.Add(machID)
	}

	return
}

// UnresolvedJobOffers returns a list of hydrated JobOffers from the Registry
func (r *EtcdRegistry) UnresolvedJobOffers() ([]job.JobOffer, error) {
	req := etcd.Get{
		Key:       path.Join(r.keyPrefix, offerPrefix),
		Sorted:    true,
		Recursive: true,
	}
	resp, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return nil, err
	}

	var offers []job.JobOffer
	for _, node := range resp.Node.Nodes {
		for _, obj := range node.Nodes {
			if !strings.HasSuffix(obj.Key, "/object") {
				continue
			}

			jo := r.getJobOfferFromJSON(obj.Value)
			if jo == nil {
				continue
			}

			offers = append(offers, *jo)
		}
	}

	return offers, nil
}

func (r *EtcdRegistry) ResolveJobOffer(jobName string) error {
	req := etcd.Delete{
		Key: path.Join(r.keyPrefix, offerPrefix, jobName, "object"),
	}
	if _, err := r.etcd.Do(&req); err != nil {
		if !isKeyNotFound(err) {
			return err
		}
	}

	req = etcd.Delete{
		Key:       path.Join(r.keyPrefix, offerPrefix, jobName),
		Recursive: true,
	}

	r.etcd.Do(&req)
	return nil
}

func (r *EtcdRegistry) SubmitJobBid(jName, machID string) {
	req := etcd.Set{
		Key: path.Join(r.keyPrefix, offerPrefix, jName, "bids", machID),
	}
	r.etcd.Do(&req)
}

func (es *EventStream) filterEventJobOffered(resp *etcd.Result) *event.Event {
	if resp.Action != "set" {
		return nil
	}

	dir, base := path.Split(resp.Node.Key)

	if base != "object" {
		return nil
	}

	dir = path.Dir(strings.TrimSuffix(dir, "/"))
	prefix := path.Base(strings.TrimSuffix(dir, "/"))

	if prefix != offerPrefix {
		return nil
	}

	jo := es.registry.getJobOfferFromJSON(resp.Node.Value)
	if jo == nil {
		return nil
	}

	return &event.Event{"EventJobOffered", *jo, nil}
}
