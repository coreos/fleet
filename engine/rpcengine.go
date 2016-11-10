// Copyright 2016 The fleet Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine

import (
	"errors"
	"time"

	etcdErr "github.com/coreos/etcd/error"

	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg/lease"
	"github.com/coreos/fleet/registry"
)

// IsGrpcLeader checks if the current leader has gRPC capabilities enabled or error
// if there is not a elected leader yet.
func (e *Engine) IsGrpcLeader() (bool, error) {
	leader, err := e.lManager.GetLease(engineLeaseName)
	if err != nil {
		log.Errorf("Unable to determine current lease: %v", err)
		return false, err
	}
	// It can happen that the leader is not yet stored in etcd and nor error (line 122 pkg/lease/etcd.go)
	if leader == nil {
		return false, errors.New("Unable to get the current leader")
	}

	leaderState, err := e.getMachineState(leader.MachineID())
	if err != nil {
		log.Errorf("Unable to determine current lease: %v", err)
		return false, err
	}

	if leaderState.Capabilities != nil && leaderState.Capabilities.Has(machine.CapGRPC) {
		return true, nil
	}

	log.Info("Engine leader has no gRPC capabilities enabled!")

	return false, nil
}

func (e *Engine) rpcLeadership(leaseTTL time.Duration, machID string) lease.Lease {
	var previousEngine string
	if e.lease != nil {
		previousEngine = e.lease.MachineID()
	}

	var l lease.Lease
	if isLeader(e.lease, machID) {
		l = rpcRenewLeadership(e.lManager, e.lease, engineVersion, leaseTTL)
	} else {
		l = rpcAcquireLeadership(e.registry, e.lManager, machID, engineVersion, leaseTTL)
	}

	// log all leadership changes
	if l != nil && e.lease == nil && l.MachineID() != machID {
		log.Infof("Engine leader is %s", l.MachineID())
	} else if l != nil && e.lease != nil && l.MachineID() != e.lease.MachineID() {
		log.Infof("Engine leadership changed from %s to %s", e.lease.MachineID(), l.MachineID())
	}

	e.lease = l
	if e.lease != nil && previousEngine != e.lease.MachineID() {
		engineState, err := e.getMachineState(e.lease.MachineID())
		if err != nil {
			log.Errorf("Failed to get machine state for machine %s %v", e.lease.MachineID(), err)
		}
		if engineState != nil {
			log.Infof("Updating engine state... engineState: %v previous: %s lease: %v", engineState, previousEngine, e.lease)
			go e.updateEngineState(*engineState)
		}
	}

	return e.lease
}

func rpcAcquireLeadership(reg registry.Registry, lManager lease.Manager, machID string, ver int, ttl time.Duration) lease.Lease {
	existing, err := lManager.GetLease(engineLeaseName)
	if err != nil {
		log.Errorf("Unable to determine current lease: %v", err)
		return nil
	}

	var l lease.Lease
	if (existing == nil && reg.UseEtcdRegistry()) || (existing == nil && !reg.IsRegistryReady()) {
		l, err = lManager.AcquireLease(engineLeaseName, machID, ver, ttl)
		if err != nil {
			log.Errorf("Engine leadership acquisition failed: %v", err)
			return nil
		} else if l == nil {
			log.Infof("Unable to acquire engine leadership")
			return nil
		}
		log.Infof("Engine leadership acquired")
		return l
	}

	// If reg is not ready, we have to give it an opportunity to steal lease
	// below. Otherwise it could be blocked forever by the existing engine leader,
	// which could cause gRPC registry to always fail when a leader already exists.
	// Thus we return the existing leader, only if reg.IsRegistryReady() == true.
	// TODO(dpark): refactor the entire function for better readability. - 20160908
	if (existing != nil && existing.Version() >= ver) && reg.IsRegistryReady() {
		log.Debugf("Lease already held by Machine(%s) operating at acceptable version %d", existing.MachineID(), existing.Version())
		return existing
	}

	// TODO(hector): Here we could add a possible SLA to determine when the leader
	// is too busy. In such a case, we can trigger a new leader election
	if (existing != nil && reg.UseEtcdRegistry()) || (existing != nil && !reg.IsRegistryReady()) {
		rem := existing.TimeRemaining()
		l, err = lManager.StealLease(engineLeaseName, machID, ver, ttl+rem, existing.Index())
		if err != nil {
			log.Errorf("Engine leadership steal failed: %v", err)
			return nil
		} else if l == nil {
			log.Infof("Unable to steal engine leadership")
			return nil
		}

		log.Infof("Stole engine leadership from Machine(%s)", existing.MachineID())

		if rem > 0 {
			log.Infof("Waiting %v for previous lease to expire before continuing reconciliation", rem)
			<-time.After(rem)
		}

		return l
	}

	log.Infof("Engine leader is BUSY!")

	return existing

}

func rpcRenewLeadership(lManager lease.Manager, l lease.Lease, ver int, ttl time.Duration) lease.Lease {
	err := l.Renew(ttl)
	if err != nil {
		if eerr, ok := err.(*etcdErr.Error); ok && eerr.ErrorCode == etcdErr.EcodeKeyNotFound {
			log.Errorf("Retry renew etcd operation that failed due to %v", err)
			l, err = lManager.AcquireLease(engineLeaseName, l.MachineID(), ver, ttl)
			if err != nil {
				log.Errorf("Engine leadership re-acquisition failed: %v", err)
				return nil
			} else if l == nil {
				log.Infof("Unable to re-acquire engine leadership")
				return nil
			}
			log.Infof("Engine leadership re-acquired")
			return l
		} else {
			log.Errorf("Engine leadership lost, renewal failed: %v", err)
			return nil
		}
	}

	log.Debugf("Engine leadership renewed")
	return l
}

func (e *Engine) getMachineState(machID string) (*machine.MachineState, error) {
	machines, err := e.registry.Machines()
	if err != nil {
		log.Errorf("Unable to get the list of machines from the registry: %v", err)
		return nil, err
	}

	for _, s := range machines {
		if s.ID == machID {
			return &s, nil
		}
	}
	return nil, nil
}
