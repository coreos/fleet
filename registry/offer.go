package registry

import (
	"path"
	"strings"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

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
func (r *FleetRegistry) CreateJobOffer(jo *job.JobOffer) error {
	jom := jobOfferModel{
		Job: jobModel{
			Name:     jo.Job.Name,
			UnitHash: jo.Job.UnitHash,
		},
		MachineIDs: jo.MachineIDs,
	}

	json, err := marshal(jom)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	key := path.Join(r.keyPrefix, offerPrefix, jo.Job.Name, "object")
	_, err = r.storage.Set(key, json, 0)
	return err
}

// getJobOfferFromJSON hydrates a JobOffer from a JSON-encoded jobOfferModel
func (r *FleetRegistry) getJobOfferFromJSON(val string) *job.JobOffer {
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

// UnresolvedJobOffers returns a list of hydrated JobOffers from the Registry
func (r *FleetRegistry) UnresolvedJobOffers() []job.JobOffer {
	var offers []job.JobOffer

	key := path.Join(r.keyPrefix, offerPrefix)
	resp, err := r.storage.Get(key, true, true)

	if err != nil {
		return offers
	}

	for _, node := range resp.Node.Nodes {
		key := path.Join(node.Key, "object")
		resp, err := r.storage.Get(key, true, true)

		// The object was probably handled between when we attempted to
		// start resolving offers and when we actually tried to get it
		if err != nil {
			continue
		}

		jo := r.getJobOfferFromJSON(resp.Node.Value)
		if jo == nil {
			continue
		}

		offers = append(offers, *jo)
	}

	return offers
}

func (r *FleetRegistry) LockJobOffer(jobName, context string) *TimedResourceMutex {
	key := path.Join(r.keyPrefix, offerPrefix, jobName)
	_, err := r.storage.Get(key, false, true)
	if err != nil {
		return nil
	}

	return r.lockResource("offer", jobName, context)
}

func (r *FleetRegistry) ResolveJobOffer(jobName string) error {
	key := path.Join(r.keyPrefix, offerPrefix, jobName, "object")
	if _, err := r.storage.Delete(key, false); err != nil {
		return err
	}

	key = path.Join(r.keyPrefix, offerPrefix, jobName)
	r.storage.Delete(key, true)
	return nil
}

func (r *FleetRegistry) SubmitJobBid(jb *job.JobBid) {
	key := path.Join(r.keyPrefix, offerPrefix, jb.JobName, "bids", jb.MachineID)
	//TODO: Use a TTL
	r.storage.Set(key, "", 0)
}

func (es *EventStream) filterEventJobOffered(resp *etcd.Response) *event.Event {
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

func filterEventJobBidSubmitted(resp *etcd.Response) *event.Event {
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
