package rpc

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/fleet/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/fleet/Godeps/_workspace/src/google.golang.org/grpc/codes"
	"github.com/coreos/fleet/debug"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	pb "github.com/coreos/fleet/protobuf"
	"github.com/coreos/fleet/registry"
)

var debugRPCServer bool = false

const (
	rpcServerPort    = 50059
	bindAddrMaxRetry = 5
	bindRetryTimeout = 500 * time.Millisecond

	registryServiceName = "rpc.Registry"
)

type rpcserver struct {
	etcdRegistry registry.Registry
	mu           *sync.Mutex
	listener     net.Listener
	grpcserver   *grpc.Server

	stop          chan struct{}
	localRegistry *inmemoryRegistry

	// serverStatus stores the serving status of this service.
	serverStatus pb.HealthCheckResponse_ServingStatus

	hasNonGRPCAgents bool
}

func NewRPCServer(reg registry.Registry, addr string) (*rpcserver, error) {
	s := &rpcserver{
		etcdRegistry:  reg,
		mu:            new(sync.Mutex),
		localRegistry: newInmemoryRegistry(),
		stop:          make(chan struct{}),
	}
	var err error
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, rpcServerPort))
	if err != nil {
		return nil, err
	}
	for it := 0; it < bindAddrMaxRetry; it++ {
		s.listener, err = net.ListenTCP("tcp", tcpAddr)
		if err == nil {
			break
		}
		log.Infof("Retrying %d to bind %s address... %v", it, tcpAddr, err)
		time.Sleep(bindRetryTimeout)
	}
	if err != nil {
		return nil, err
	}

	s.grpcserver = grpc.NewServer()
	s.localRegistry.LoadFrom(s.etcdRegistry)
	pb.RegisterRegistryServer(s.grpcserver, s)

	s.SetServingStatus(pb.HealthCheckResponse_NOT_SERVING)

	machineStates, err := s.etcdRegistry.Machines()
	if err != nil {
		return nil, err
	}
	s.hasNonGRPCAgents = false
	for _, state := range machineStates {
		if !state.Capabilities.Has(machine.CapGRPC) {
			log.Info("Fleet cluster has non gRPC agents!. Enabled unit state storage into etcd!")
			s.hasNonGRPCAgents = true
			break
		}
	}
	return s, nil
}

func (s *rpcserver) Status(ctx context.Context, in *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.Service != registryServiceName {
		return nil, grpc.Errorf(codes.NotFound, "unknown rpc service to collect its status")
	}
	return &pb.HealthCheckResponse{
		Status: s.serverStatus,
	}, nil
}

// SetServingStatus is called when need to reset the serving status of the service
func (s *rpcserver) SetServingStatus(status pb.HealthCheckResponse_ServingStatus) {
	s.mu.Lock()
	s.serverStatus = status
	s.mu.Unlock()
}

func (s *rpcserver) Start() error {
	s.SetServingStatus(pb.HealthCheckResponse_SERVING)
	return s.grpcserver.Serve(s.listener)
}

func (s *rpcserver) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	s.SetServingStatus(pb.HealthCheckResponse_NOT_SERVING)
	s.grpcserver.Stop()
}

func (s *rpcserver) GetScheduledUnits(ctx context.Context, unitFilter *pb.UnitFilter) (*pb.ScheduledUnits, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_())
	}

	units, err := s.localRegistry.Schedule()

	return &pb.ScheduledUnits{Units: units}, err
}

func (s *rpcserver) GetScheduledUnit(ctx context.Context, name *pb.UnitName) (*pb.MaybeScheduledUnit, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	su := s.localRegistry.ScheduledUnit(name.Name)

	return &pb.MaybeScheduledUnit{IsScheduled: &pb.MaybeScheduledUnit_Unit{Unit: su}}, nil
}

func (s *rpcserver) GetUnit(ctx context.Context, name *pb.UnitName) (*pb.MaybeUnit, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	unit, exists := s.localRegistry.Unit(name.Name)
	if exists {
		return &pb.MaybeUnit{HasUnit: &pb.MaybeUnit_Unit{Unit: &unit}}, nil
	}
	return &pb.MaybeUnit{HasUnit: &pb.MaybeUnit_Notfound{Notfound: &pb.NotFound{}}}, nil

}

func (s *rpcserver) GetUnits(ctx context.Context, filter *pb.UnitFilter) (*pb.Units, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_())
	}
	units := make([]pb.Unit, 0)
	units = append(units, s.localRegistry.Units()...)

	// Check if there are etcd fleet-based agents in the cluster to share the state
	if s.hasNonGRPCAgents {
		log.Debug("Merging etcd with inmemory units in GetUnits()")
		etcdUnits, err := s.etcdRegistry.Units()
		if err != nil {
			return nil, err
		}

		unitNames := make(map[string]struct{}, len(units))
		for _, unit := range units {
			unitNames[unit.Name] = struct{}{}
		}
		for _, unit := range etcdUnits {
			if _, ok := unitNames[unit.Name]; !ok {
				units = append(units, unit.ToPB())
			}
		}
	}

	return &pb.Units{Units: units}, nil
}

func (s *rpcserver) GetUnitStates(ctx context.Context, filter *pb.UnitStateFilter) (*pb.UnitStates, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_())
	}
	states := make([]*pb.UnitState, 0)
	states = append(states, s.localRegistry.UnitStates()...)

	if s.hasNonGRPCAgents {
		log.Debug("Merging etcd with inmemory unit states in GetUnitStates()")
		etcdUnitStates, err := s.etcdRegistry.UnitStates()
		if err != nil {
			return nil, err
		}

		unitStateNames := make(map[string]string, len(states))
		for _, state := range states {
			unitStateNames[state.Name] = state.MachineID
		}
		for _, state := range etcdUnitStates {
			machId, ok := unitStateNames[state.UnitName]
			if !ok || (ok && machId != state.MachineID) {
				states = append(states, state.ToPB())
			}
		}
	}

	return &pb.UnitStates{states}, nil
}

func (s *rpcserver) ClearUnitHeartbeat(ctx context.Context, name *pb.UnitName) (*pb.GenericReply, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	s.localRegistry.ClearUnitHeartbeat(name.Name)
	return &pb.GenericReply{}, nil
}

func (s *rpcserver) CreateUnit(ctx context.Context, u *pb.Unit) (*pb.GenericReply, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(u.Name))
	}

	err := s.etcdRegistry.CreateUnit(rpcUnitToJobUnit(u))
	if err == nil {
		s.localRegistry.CreateUnit(u)
	}
	return &pb.GenericReply{}, err
}

func (s *rpcserver) DestroyUnit(ctx context.Context, name *pb.UnitName) (*pb.GenericReply, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	err := s.etcdRegistry.DestroyUnit(name.Name)
	if err == nil {
		s.localRegistry.DestroyUnit(name.Name)
	}
	return &pb.GenericReply{}, err
}

func (s *rpcserver) UnitHeartbeat(ctx context.Context, heartbeat *pb.Heartbeat) (*pb.GenericReply, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(heartbeat))
	}

	s.localRegistry.UnitHeartbeat(heartbeat.Name, heartbeat.MachineID, time.Duration(heartbeat.TTL)*time.Second)
	return &pb.GenericReply{}, nil
}

func (s *rpcserver) RemoveUnitState(ctx context.Context, name *pb.UnitName) (*pb.GenericReply, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	// Check if there are etcd fleet-based agents in the cluster to share the state
	if s.hasNonGRPCAgents {
		s.etcdRegistry.RemoveUnitState(name.Name)
	}

	s.localRegistry.RemoveUnitState(name.Name)
	return &pb.GenericReply{}, nil
}

func (s *rpcserver) SaveUnitState(ctx context.Context, req *pb.SaveUnitStateRequest) (*pb.GenericReply, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(req))
	}

	// Check if there are etcd fleet-based agents in the cluster to share the state
	if s.hasNonGRPCAgents {
		unitState := rpcUnitStateToExtUnitState(req.State)
		s.etcdRegistry.SaveUnitState(req.Name, unitState, time.Duration(req.TTL)*time.Second)
	}

	s.localRegistry.SaveUnitState(req.Name, req.State, time.Duration(req.TTL)*time.Second)
	return &pb.GenericReply{}, nil
}

func (s *rpcserver) ScheduleUnit(ctx context.Context, unit *pb.ScheduleUnitRequest) (*pb.GenericReply, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(unit.Name, unit.MachineID))
	}

	err := s.etcdRegistry.ScheduleUnit(unit.Name, unit.MachineID)
	if err == nil {
		s.localRegistry.ScheduleUnit(unit.Name, unit.MachineID)
	}
	return &pb.GenericReply{}, err
}

func (s *rpcserver) SetUnitTargetState(ctx context.Context, unit *pb.ScheduledUnit) (*pb.GenericReply, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(unit.Name, unit.CurrentState))
	}

	err := s.etcdRegistry.SetUnitTargetState(unit.Name, rpcUnitStateToJobState(unit.CurrentState))
	if err == nil {
		if s.localRegistry.SetUnitTargetState(unit.Name, unit.CurrentState) {
		}
	}
	return &pb.GenericReply{}, err
}

func (s *rpcserver) UnscheduleUnit(ctx context.Context, unit *pb.UnscheduleUnitRequest) (*pb.GenericReply, error) {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(unit.Name, unit.MachineID))
	}

	err := s.etcdRegistry.UnscheduleUnit(unit.Name, unit.MachineID)
	if err == nil {
		s.localRegistry.UnscheduleUnit(unit.Name, unit.MachineID)
	}
	return &pb.GenericReply{}, err
}

func (s *rpcserver) AgentEvents(props *pb.MachineProperties, stream pb.Registry_AgentEventsServer) error {
	if debugRPCServer {
		defer debug.Exit_(debug.Enter_(props.Id))
	}
	return errors.New("Agent events function not implemented")
}
