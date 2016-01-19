package registry

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	"github.com/coreos/fleet/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/fleet/Godeps/_workspace/src/google.golang.org/grpc"

	"github.com/coreos/fleet/debug"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	pb "github.com/coreos/fleet/protobuf"
	"github.com/coreos/fleet/unit"
)

const (
	port                  = 50059
	grpcConnectionTimeout = 500 * time.Millisecond
)

var DebugRPCRegistry bool = false

type RPCRegistry struct {
	registryClient pb.RegistryClient
	registryConn   *grpc.ClientConn
	mu             *sync.Mutex
	dialer         func(addr string, timeout time.Duration) (net.Conn, error)
}

func NewRPCRegistry(dialer func(string, time.Duration) (net.Conn, error)) *RPCRegistry {
	return &RPCRegistry{
		mu:     new(sync.Mutex),
		dialer: dialer,
	}
}

func (r *RPCRegistry) ctx() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	return ctx
}

func (r *RPCRegistry) Connect() {
	var err error
	r.registryConn, err = grpc.Dial(":fleet-engine:", grpc.WithInsecure(), grpc.WithDialer(r.dialer), grpc.WithTimeout(grpcConnectionTimeout), grpc.WithBlock())
	if err != nil {
		log.Fatalf("unable to dial to registry: %s", err)
	}

	r.registryClient = pb.NewRegistryClient(r.registryConn)
}

func (r *RPCRegistry) getClient() pb.RegistryClient {
	for ; ; time.Sleep(100 * time.Millisecond) {
		log.Infof("RegistryClient is not initialized, waiting for the connection...")
		if r.registryClient != nil {
			break
		}
	}
	return r.registryClient
}

func (r *RPCRegistry) ClearUnitHeartbeat(unitName string) {
	if DebugRPCRegistry {
		defer debug.Exit_(debug.Enter_(unitName))
	}

	r.getClient().ClearUnitHeartbeat(r.ctx(), &pb.UnitName{unitName})
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

	_, err := r.getClient().DestroyUnit(r.ctx(), &pb.UnitName{unitName})
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
	return errors.New("Remove machine state not implemented")
}

func (r *RPCRegistry) RemoveUnitState(unitName string) error {
	_, err := r.getClient().RemoveUnitState(r.ctx(), &pb.UnitName{unitName})
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

func (r *RPCRegistry) Machines() ([]machine.MachineState, error) {
	panic("not implemented")
}

func (r *RPCRegistry) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	panic("not implemented")
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

	maybeSchedUnit, err := r.getClient().GetScheduledUnit(r.ctx(), &pb.UnitName{unitName})

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

	maybeUnit, err := r.getClient().GetUnit(r.ctx(), &pb.UnitName{unitName})
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
		log.Errorf("rpcregistry failed to get the units %v", err)
		return []job.Unit{}, err
	}

	jobUnits := make([]job.Unit, len(units.Units))
	for i, u := range units.Units {
		jobUnit := rpcUnitToJobUnit(&u)
		jobUnits[i] = *jobUnit
	}
	return jobUnits, nil
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
	return 0, errors.New("Engine version not implemented")
}

func (r *RPCRegistry) UpdateEngineVersion(from, to int) error {
	return errors.New("Update engine version not implemented")
}

func (r *RPCRegistry) LatestDaemonVersion() (*semver.Version, error) {
	return nil, errors.New("Latests daemon version not implemented")
}
