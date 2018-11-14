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

package rpc

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/coreos/fleet/debug"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	pb "github.com/coreos/fleet/protobuf"
	"github.com/coreos/fleet/unit"
)

var DebugRPCRegistry bool = false

type RPCRegistry struct {
	dialer         func(addr string, timeout time.Duration) (net.Conn, error)
	mu             *sync.Mutex
	registryClient pb.RegistryClient
	registryConn   *grpc.ClientConn
	balancer       *simpleBalancer
}

func NewRPCRegistry(dialer func(string, time.Duration) (net.Conn, error)) *RPCRegistry {
	return &RPCRegistry{
		mu:     new(sync.Mutex),
		dialer: dialer,
	}
}

func (r *RPCRegistry) ctx() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx
}

func (r *RPCRegistry) getClient() pb.RegistryClient {
	return r.registryClient
}

func (r *RPCRegistry) Connect() {
	// We want the connection operation to block and constantly reconnect using grpc backoff
	log.Info("Starting gRPC connection to fleet-engine...")
	ep_engines := []string{":fleet-engine:"}
	r.balancer = newSimpleBalancer(ep_engines)
	connection, err := grpc.Dial(ep_engines[0],
		grpc.WithTimeout(12*time.Second), grpc.WithInsecure(),
		grpc.WithDialer(r.dialer), grpc.WithBlock(), grpc.WithBalancer(r.balancer))
	if err != nil {
		log.Fatalf("Unable to dial to registry: %s", err)
	}
	r.registryConn = connection
	r.registryClient = pb.NewRegistryClient(r.registryConn)
	log.Info("Connected succesfully to fleet-engine via grpc!")
}

func (r *RPCRegistry) Close() {
	r.registryConn.Close()
}

func (r *RPCRegistry) IsRegistryReady() bool {
	if r.registryConn != nil {
		hasConn := false
		if r.balancer != nil {
			select {
			case <-r.balancer.readyc:
				hasConn = true
			}
		}

		status, err := r.Status()
		if err != nil {
			log.Errorf("unable to get the status of the registry service %v", err)
			return false
		}
		log.Infof("Status of rpc service: %d, balancer has a connection: %t", status, hasConn)

		return hasConn && status == pb.HealthCheckResponse_SERVING && err == nil
	}
	return false
}

func (r *RPCRegistry) UseEtcdRegistry() bool {
	return false
}

func (r *RPCRegistry) Status() (pb.HealthCheckResponse_ServingStatus, error) {
	req := &pb.HealthCheckRequest{
		Service: registryServiceName,
	}
	resp, err := r.getClient().Status(r.ctx(), req)
	if err != nil {
		return -1, err
	}
	return resp.Status, err
}

func (r *RPCRegistry) ClearUnitHeartbeat(unitName string) {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName))
	}

	r.getClient().ClearUnitHeartbeat(r.ctx(), &pb.UnitName{Name: unitName})
}

func (r *RPCRegistry) CreateUnit(j *job.Unit) error {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(j.Name))
	}

	un := j.ToPB()
	_, err := r.getClient().CreateUnit(r.ctx(), &un)
	return err
}

func (r *RPCRegistry) DestroyUnit(unitName string) error {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName))
	}

	_, err := r.getClient().DestroyUnit(r.ctx(), &pb.UnitName{Name: unitName})
	return err
}

func (r *RPCRegistry) UnitHeartbeat(unitName, machID string, ttl time.Duration) error {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName, machID))
	}

	_, err := r.getClient().UnitHeartbeat(r.ctx(), &pb.Heartbeat{
		Name:      unitName,
		MachineID: machID,
		TTL:       int32(ttl.Seconds()),
	})
	return err
}

func (r *RPCRegistry) RemoveMachineState(machID string) error {
	return errors.New("Remove machine state function not implemented")
}

func (r *RPCRegistry) RemoveUnitState(unitName string) error {
	_, err := r.getClient().RemoveUnitState(r.ctx(), &pb.UnitName{Name: unitName})
	return err
}

func (r *RPCRegistry) SaveUnitState(unitName string, unitState *unit.UnitState, ttl time.Duration) {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName, unitState))
	}

	if unitState.UnitName == "" {
		unitState.UnitName = unitName
	}

	r.getClient().SaveUnitState(r.ctx(), &pb.SaveUnitStateRequest{
		Name:  unitName,
		State: unitState.ToPB(),
		TTL:   int32(ttl.Seconds()),
	})
}

func (r *RPCRegistry) ScheduleUnit(unitName, machID string) error {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName, machID))
	}

	_, err := r.getClient().ScheduleUnit(r.ctx(), &pb.ScheduleUnitRequest{
		Name:      unitName,
		MachineID: machID,
	})
	return err
}

func (r *RPCRegistry) SetUnitTargetState(unitName string, state job.JobState) error {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName, state))
	}

	_, err := r.getClient().SetUnitTargetState(r.ctx(), &pb.ScheduledUnit{
		Name:         unitName,
		CurrentState: state.ToPB(),
	})
	return err
}

func (r *RPCRegistry) UnscheduleUnit(unitName, machID string) error {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName, machID))
	}

	_, err := r.getClient().UnscheduleUnit(r.ctx(), &pb.UnscheduleUnitRequest{
		Name:      unitName,
		MachineID: machID,
	})
	return err
}

func (r *RPCRegistry) SetMachineMetadata(machID string, key string, value string) error {
	panic("Set machine metadata function not implemented")
}

func (r *RPCRegistry) DeleteMachineMetadata(machID string, key string) error {
	panic("Delete machine metadata function not implemented")
}

func (r *RPCRegistry) Machines() ([]machine.MachineState, error) {
	panic("Machines function not implemented")
}

func (r *RPCRegistry) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	panic("Set machine state function not implemented")
}

func (r *RPCRegistry) MachineState(machID string) (machine.MachineState, error) {
	panic("Machine state function not implemented")
}

func (r *RPCRegistry) CreateMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	panic("Set machine state function not implemented")
}

func (r *RPCRegistry) Schedule() ([]job.ScheduledUnit, error) {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_())
	}

	scheduledUnits, err := r.getClient().GetScheduledUnits(r.ctx(), &pb.UnitFilter{})
	if err != nil {
		return []job.ScheduledUnit{}, err
	}
	units := make([]job.ScheduledUnit, len(scheduledUnits.Units))

	for i, unit := range scheduledUnits.Units {
		state := rpcUnitStateToJobState(unit.CurrentState)
		units[i] = job.ScheduledUnit{
			Name:            unit.Name,
			TargetMachineID: unit.MachineID,
			State:           &state,
		}
	}
	return units, err
}

func (r *RPCRegistry) ScheduledUnit(unitName string) (*job.ScheduledUnit, error) {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName))
	}

	maybeSchedUnit, err := r.getClient().GetScheduledUnit(r.ctx(), &pb.UnitName{Name: unitName})

	if err != nil {
		return nil, err
	}

	if scheduledUnit := maybeSchedUnit.GetUnit(); scheduledUnit != nil {
		state := rpcUnitStateToJobState(scheduledUnit.CurrentState)
		scheduledJob := &job.ScheduledUnit{
			Name:            scheduledUnit.Name,
			TargetMachineID: scheduledUnit.MachineID,
			State:           &state,
		}
		return scheduledJob, err
	}
	return nil, nil

}

func (r *RPCRegistry) Unit(unitName string) (*job.Unit, error) {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName))
	}

	maybeUnit, err := r.getClient().GetUnit(r.ctx(), &pb.UnitName{Name: unitName})
	if err != nil {
		return nil, err
	}

	if unit := maybeUnit.GetUnit(); unit != nil {
		ur := rpcUnitToJobUnit(unit)
		return ur, nil
	}
	return nil, nil
}

func (r *RPCRegistry) Units() ([]job.Unit, error) {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_())
	}

	units, err := r.getClient().GetUnits(r.ctx(), &pb.UnitFilter{})
	if err != nil {
		log.Errorf("RPC registry failed to get the units %v", err)
		return []job.Unit{}, err
	}

	jobUnits := make([]job.Unit, len(units.Units))
	for i, u := range units.Units {
		jobUnit := rpcUnitToJobUnit(&u)
		jobUnits[i] = *jobUnit
	}
	return jobUnits, nil
}

func (r *RPCRegistry) UnitState(unitName string) (*unit.UnitState, error) {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName))
	}

	state, err := r.getClient().GetUnitState(r.ctx(), &pb.UnitName{Name: unitName})
	if err != nil {
		return nil, err
	}

	return &unit.UnitState{
		UnitName:    state.Name,
		MachineID:   state.MachineID,
		UnitHash:    state.Hash,
		LoadState:   state.LoadState,
		ActiveState: state.ActiveState,
		SubState:    state.SubState,
	}, nil
}

func (r *RPCRegistry) UnitStates() ([]*unit.UnitState, error) {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_())
	}

	unitStates, err := r.getClient().GetUnitStates(r.ctx(), &pb.UnitStateFilter{})
	if err != nil {
		return nil, err
	}

	nUnitStates := make([]*unit.UnitState, len(unitStates.UnitStates))

	for i, state := range unitStates.UnitStates {
		nUnitStates[i] = &unit.UnitState{
			UnitName:    state.Name,
			MachineID:   state.MachineID,
			UnitHash:    state.Hash,
			LoadState:   state.LoadState,
			ActiveState: state.ActiveState,
			SubState:    state.SubState,
		}
	}
	return nUnitStates, nil
}

func (r *RPCRegistry) EngineVersion() (int, error) {
	return 0, errors.New("Engine version function not implemented")
}

func (r *RPCRegistry) UpdateEngineVersion(from, to int) error {
	return errors.New("Update engine version function not implemented")
}

func (r *RPCRegistry) LatestDaemonVersion() (*semver.Version, error) {
	return nil, errors.New("Latest daemon version function not implemented")
}
