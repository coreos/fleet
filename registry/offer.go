package registry

import (
	"path"
	"strings"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
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

// Bids returns a list of JobBids that have been submitted for the given JobOffer
func (r *EtcdRegistry) Bids(jo *job.JobOffer) ([]job.JobBid, error) {
	var bids []job.JobBid

	req := etcd.Get{
		Key:       path.Join(r.keyPrefix, offerPrefix, jo.Job.Name, "bids"),
		Recursive: true,
	}
	resp, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			return bids, nil
		}
		return nil, err
	}

	for _, node := range resp.Node.Nodes {
		machID := path.Base(node.Key)
		jb := job.NewBid(jo.Job.Name, machID)
		bids = append(bids, *jb)
	}

	return bids, nil
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

func (r *EtcdRegistry) SubmitJobBid(jb *job.JobBid) {
	req := etcd.Set{
		Key: path.Join(r.keyPrefix, offerPrefix, jb.JobName, "bids", jb.MachineID),
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

func filterEventJobBidSubmitted(resp *etcd.Result) *event.Event {
	if resp.Action != "set" {
		return nil
	}

	dir, machID := path.Split(resp.Node.Key)
	dir, prefix := path.Split(strings.TrimSuffix(dir, "/"))

	if prefix != "bids" {
		return nil
	}

	dir, jobName := path.Split(strings.TrimSuffix(dir, "/"))
	prefix = path.Base(strings.TrimSuffix(dir, "/"))

	if prefix != offerPrefix {
		return nil
	}

	jb := job.NewBid(jobName, machID)
	return &event.Event{"EventJobBidSubmitted", *jb, nil}
}
