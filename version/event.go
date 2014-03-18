package version

import (
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
)

type eventHandler struct {
	negotiator *Negotiator
	cluster    ClusterState
}

func (eh *eventHandler) HandleEventNegotiatorPublished(ev event.Event) {
	other := ev.Payload.(Negotiator)
	log.V(1).Infof("EventNegotiatorPublished(%s): negotiator published to cluster", other.Name)

	doit, ver := eh.shouldUpgrade()
	if !doit {
		log.V(1).Infof("EventNegotiatorPublished(%s): cluster upgrade currently not necessary", other.Name)
		return
	}

	mutex := eh.cluster.AcquireMutex(eh.negotiator)
	if mutex == nil {
		log.V(1).Infof("EventNegotiatorPublished(%s): cluster ready to upgrade, but unable to lock mutex", other.Name)
		return
	}

	defer mutex.Unlock()

	log.V(1).Infof("EventNegotiatorPublished(%s): attempting cluster upgrade to version %d", other.Name, ver)
	if err := eh.cluster.Upgrade(ver); err != nil {
		log.Errorf("Failed upgrading cluster to version %d: %v", err)
	} else {
		log.Infof("EventNegotiatorPublished(%s): upgraded cluster to version %d", other.Name, ver)
	}
}

func (eh *eventHandler) HandleEventNegotiatorRemoved(ev event.Event) {
	other := ev.Payload.(Negotiator)
	log.V(1).Infof("EventNegotiatorRemoved(%s): negotiator removed from cluster", other.Name)

	doit, ver := eh.shouldUpgrade()
	if !doit {
		log.V(1).Infof("EventNegotiatorRemoved(%s): cluster upgrade currently not necessary", other.Name)
		return
	}

	mutex := eh.cluster.AcquireMutex(eh.negotiator)
	if mutex == nil {
		log.V(1).Infof("EventNegotiatorRemoved(%s): cluster ready to upgrade, but unable to lock mutex", other.Name)
		return
	}

	defer mutex.Unlock()

	log.V(1).Infof("EventNegotiatorRemoved(%s): attempting cluster upgrade to v%d", other.Name, ver)
	if err := eh.cluster.Upgrade(ver); err != nil {
		log.Errorf("Failed upgrading cluster to v%d: %v", err)
	} else {
		log.Infof("EventNegotiatorRemoved(%s): upgraded cluster to v%d", other.Name, ver)
	}
}

func (eh *eventHandler) shouldUpgrade() (bool, int) {
	negotiators, err := eh.cluster.Negotiators()
	if err != nil {
		log.Errorf("Failed fetching negotiator list: %v", err)
		return false, -1
	}

	maxVersion, err := MaxVersionPossible(negotiators)
	if err != nil {
		log.Errorf("Failed determining max possible version: %v", err)
		return false, -1
	}

	curVersion, _, err := eh.cluster.Version()
	if err != nil {
		log.Errorf("Failed determining current cluster version: %v", err)
		return false, -1
	}

	if maxVersion < curVersion {
		log.Errorf("Calculated max version is less than current cluster version.")
		return false, -1
	} else if maxVersion == curVersion {
		return false, -1
	}

	return true, maxVersion
}

func (eh *eventHandler) HandleEventClusterUpgraded(ev event.Event) {
	newVersion := ev.Payload.(int)
	log.V(1).Infof("EventClusterUpgraded(v%d): cluster upgraded to v%d", newVersion, newVersion)
	eh.negotiator.SetCurrentVersion(newVersion)
	log.Infof("EventClusterUpgraded(v%d): cluster upgraded to v%d - informed local components", newVersion, newVersion)
}
