package registry

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/fleet/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/fleet/debug"
	pb "github.com/coreos/fleet/protobuf"
)

var DebugRPCServer bool = false

type rpcserver struct {
	etcdRegistry Registry
	mu           *sync.Mutex
	listener     net.Listener
	grpcserver   *grpc.Server

	stop          chan struct{}
	localRegistry *inmemoryRegistry
}

func NewRPCServer(reg Registry, addr string) (*rpcserver, error) {
	s := &rpcserver{
		etcdRegistry:  reg,
		mu:            new(sync.Mutex),
		localRegistry: NewInmemoryRegistry(),
		stop:          make(chan struct{}),
	}
	var err error
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}
	s.listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
	}

	s.grpcserver = grpc.NewServer()
	s.localRegistry.LoadFrom(s.etcdRegistry)
	pb.RegisterRegistryServer(s.grpcserver, s)

	return s, nil
}

func (s *rpcserver) Start() {
	go s.grpcserver.Serve(s.listener)
}

func (s *rpcserver) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}

	s.grpcserver.Stop()
}

func (s *rpcserver) GetScheduledUnits(ctx context.Context, unitFilter *pb.UnitFilter) (*pb.ScheduledUnits, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_())
	}
	units, err := s.localRegistry.Schedule()

	return &pb.ScheduledUnits{Units: units}, err
}

func (s *rpcserver) GetScheduledUnit(ctx context.Context, name *pb.UnitName) (*pb.MaybeScheduledUnit, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	su, exists := s.localRegistry.ScheduledUnit(name.Name)
	if exists {
		return &pb.MaybeScheduledUnit{IsScheduled: &pb.MaybeScheduledUnit_Unit{Unit: su}}, nil
	}
	return &pb.MaybeScheduledUnit{IsScheduled: &pb.MaybeScheduledUnit_Notfound{Notfound: &pb.NotFound{}}}, nil
}

func (s *rpcserver) GetUnit(ctx context.Context, name *pb.UnitName) (*pb.MaybeUnit, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	unit, exists := s.localRegistry.Unit(name.Name)
	if exists {
		return &pb.MaybeUnit{HasUnit: &pb.MaybeUnit_Unit{Unit: &unit}}, nil
	}
	return &pb.MaybeUnit{HasUnit: &pb.MaybeUnit_Notfound{Notfound: &pb.NotFound{}}}, nil

}

func (s *rpcserver) GetUnits(ctx context.Context, filter *pb.UnitFilter) (*pb.Units, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_())
	}

	units := s.localRegistry.Units()
	return &pb.Units{Units: units}, nil
}

func (s *rpcserver) GetUnitStates(ctx context.Context, filter *pb.UnitStateFilter) (*pb.UnitStates, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_())
	}

	states := s.localRegistry.UnitStates()

	return &pb.UnitStates{states}, nil
}

func (s *rpcserver) ClearUnitHeartbeat(ctx context.Context, name *pb.UnitName) (*pb.GenericReply, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	s.localRegistry.ClearUnitHeartbeat(name.Name)
	return &pb.GenericReply{}, nil
}

func (s *rpcserver) CreateUnit(ctx context.Context, u *pb.Unit) (*pb.GenericReply, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(u.Name))
	}

	err := s.etcdRegistry.CreateUnit(rpcUnitToJobUnit(u))
	if err == nil {
		s.localRegistry.CreateUnit(u)
	}
	return &pb.GenericReply{}, err
}

func (s *rpcserver) DestroyUnit(ctx context.Context, name *pb.UnitName) (*pb.GenericReply, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	err := s.etcdRegistry.DestroyUnit(name.Name)
	if err == nil {
		s.localRegistry.DestroyUnit(name.Name)
	}
	return &pb.GenericReply{}, err
}

func (s *rpcserver) UnitHeartbeat(ctx context.Context, heartbeat *pb.Heartbeat) (*pb.GenericReply, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(heartbeat))
	}

	s.localRegistry.UnitHeartbeat(heartbeat.Name, heartbeat.MachineID, time.Duration(heartbeat.TTL)*time.Second)
	return &pb.GenericReply{}, nil
}

func (s *rpcserver) RemoveUnitState(ctx context.Context, name *pb.UnitName) (*pb.GenericReply, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(name.Name))
	}

	s.localRegistry.RemoveUnitState(name.Name)
	return &pb.GenericReply{}, nil
}

func (s *rpcserver) SaveUnitState(ctx context.Context, req *pb.SaveUnitStateRequest) (*pb.GenericReply, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(req))
	}

	s.localRegistry.SaveUnitState(req.Name, req.State, time.Duration(req.TTL)*time.Second)
	return &pb.GenericReply{}, nil
}

func (s *rpcserver) ScheduleUnit(ctx context.Context, unit *pb.ScheduleUnitRequest) (*pb.GenericReply, error) {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(unit.Name, unit.MachineID))
	}

	err := s.etcdRegistry.ScheduleUnit(unit.Name, unit.MachineID)
	if err == nil {
		s.localRegistry.ScheduleUnit(unit.Name, unit.MachineID)
	}
	return &pb.GenericReply{}, err
}

func (s *rpcserver) SetUnitTargetState(ctx context.Context, unit *pb.ScheduledUnit) (*pb.GenericReply, error) {
	if DebugRPCServer {
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
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(unit.Name, unit.MachineID))
	}

	err := s.etcdRegistry.UnscheduleUnit(unit.Name, unit.MachineID)
	if err == nil {
		s.localRegistry.UnscheduleUnit(unit.Name, unit.MachineID)
	}
	return &pb.GenericReply{}, err
}

func (s *rpcserver) AgentEvents(props *pb.MachineProperties, stream pb.Registry_AgentEventsServer) error {
	if DebugRPCServer {
		defer debug.Exit_(debug.Enter_(props.Id))
	}
	return errors.New("AgentEvents function not implemented")
}
