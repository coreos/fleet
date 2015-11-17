package registry

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/fleet/job"
	pb "github.com/coreos/fleet/rpc"
	"github.com/coreos/fleet/unit"
	"google.golang.org/grpc"

	sdunit "github.com/coreos/go-systemd/unit"
)

type rpcserver struct {
	etcdRegistry *EtcdRegistry
}

func (r *RPCRegistry) startServer() {
	if r.listener != nil {
		r.listener.Close()
	}
	var err error
	r.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	s := grpc.NewServer()
	pb.RegisterRegistryServer(s, &rpcserver{r.etcdRegistry})
	go s.Serve(r.listener)
}

func (s *rpcserver) GetScheduledUnits(ctx context.Context, unitFilter *pb.UnitFilter) (*pb.ScheduledUnits, error) {
	schedule, err := s.etcdRegistry.Schedule()
	if err != nil {
		return nil, err
	}

	units := make([]*pb.ScheduledUnit, len(schedule))
	for i, u := range schedule {
		units[i] = jobScheduledUnitToRPC(&u)
	}

	return &pb.ScheduledUnits{
		Units: units,
	}, nil
}

func (s *rpcserver) GetScheduledUnit(ctx context.Context, name *pb.UnitName) (*pb.ScheduledUnit, error) {
	scheduledUnit, err := s.etcdRegistry.ScheduledUnit(name.Name)
	if err != nil {
		return nil, err
	}
	return jobScheduledUnitToRPC(scheduledUnit), nil
}

func (s *rpcserver) GetUnit(ctx context.Context, name *pb.UnitName) (*pb.Unit, error) {
	u, err := s.etcdRegistry.Unit(name.Name)
	if err != nil {
		return nil, err
	}

	return unitToRPC(u), nil
}

func unitToRPC(u *job.Unit) *pb.Unit {
	return &pb.Unit{
		Name:        u.Name,
		Unit:        unitFileToRPC(u.Unit),
		TargetState: unitStateToRPC(u.TargetState),
	}
}

func unitFileToRPC(unitFile unit.UnitFile) *pb.UnitFile {
	sections := map[string]*pb.UnitSection{}
	for _, unitOption := range unitFile.Options {
		unitSectionOption := &pb.UnitSectionOption{
			Name:  unitOption.Name,
			Value: unitOption.Value,
		}

		if _, exists := sections[unitOption.Section]; !exists {
			sections[unitOption.Section] = &pb.UnitSection{
				Name: unitOption.Section,
				Options: []*pb.UnitSectionOption{
					unitSectionOption,
				},
			}
		} else {
			sections[unitOption.Section].Options = append(sections[unitOption.Section].Options, unitSectionOption)
		}
	}

	unitFileSections := []*pb.UnitSection{}
	for _, section := range sections {
		unitFileSections = append(unitFileSections, section)
	}

	return &pb.UnitFile{Sections: unitFileSections}
}

func (s *rpcserver) GetUnits(context.Context, *pb.UnitFilter) (*pb.Units, error) {
	units, err := s.etcdRegistry.Units()
	if err != nil {
		return nil, err
	}

	rpcUnits := make([]*pb.Unit, len(units))
	for idx, unit := range units {
		rpcUnits[idx] = unitToRPC(&unit)
	}

	return &pb.Units{Units: rpcUnits}, nil
}

func (s *rpcserver) GetUnitStates(context.Context, *pb.UnitStateFilter) (*pb.UnitStates, error) {
	unitStates, err := s.etcdRegistry.UnitStates()
	if err != nil {
		return nil, err
	}

	rpcUnitStates := make([]*pb.UnitState, len(unitStates))
	for idx, unitState := range unitStates {
		rpcUnitStates[idx] = unitExtStateToRPC(unitState)
	}

	return &pb.UnitStates{rpcUnitStates}, nil
}

func unitExtStateToRPC(s *unit.UnitState) *pb.UnitState {
	return &pb.UnitState{
		Name:        s.UnitName,
		Hash:        s.UnitHash,
		LoadState:   s.LoadState,
		ActiveState: s.ActiveState,
		SubState:    s.SubState,
		Machineid:   s.MachineID,
	}
}

func (s *rpcserver) ClearUnitHeartbeat(context.Context, *pb.UnitName) (*pb.GenericReply, error) {
	return &pb.GenericReply{}, nil
}

func rpcUnitToJobUnit(u *pb.Unit) *job.Unit {
	unitOptions := []*sdunit.UnitOption{}

	for _, section := range u.Unit.Sections {
		for _, sectionOption := range section.Options {
			unitOptions = append(unitOptions, &sdunit.UnitOption{
				Section: section.Name,
				Name:    sectionOption.Name,
				Value:   sectionOption.Value,
			})
		}
	}

	return &job.Unit{
		Name:        u.Name,
		Unit:        *unit.NewUnitFromOptions(unitOptions),
		TargetState: rpcUnitStateToJobState(u.TargetState),
	}
}

func (s *rpcserver) CreateUnit(ctx context.Context, u *pb.Unit) (*pb.GenericReply, error) {
	err := s.etcdRegistry.CreateUnit(rpcUnitToJobUnit(u))

	return &pb.GenericReply{}, err
}

func (s *rpcserver) DestroyUnit(ctx context.Context, name *pb.UnitName) (*pb.GenericReply, error) {
	err := s.etcdRegistry.DestroyUnit(name.Name)
	return &pb.GenericReply{}, err
}

func (s *rpcserver) UnitHeartbeat(ctx context.Context, heartbeat *pb.Heartbeat) (*pb.GenericReply, error) {
	err := s.etcdRegistry.UnitHeartbeat(heartbeat.Name, heartbeat.Machineid, time.Duration(heartbeat.Ttl)*time.Second)
	return &pb.GenericReply{}, err
}

func (s *rpcserver) RemoveUnitState(ctx context.Context, name *pb.UnitName) (*pb.GenericReply, error) {
	err := s.etcdRegistry.RemoveUnitState(name.Name)
	return &pb.GenericReply{}, err
}

func rpcUnitStateToExtUnitState(state *pb.UnitState) *unit.UnitState {
	return &unit.UnitState{
		UnitName:    state.Name,
		UnitHash:    state.Hash,
		LoadState:   state.LoadState,
		ActiveState: state.ActiveState,
		SubState:    state.SubState,
		MachineID:   state.Machineid,
	}
}

func (s *rpcserver) SaveUnitState(ctx context.Context, req *pb.SaveUnitStateReq) (*pb.GenericReply, error) {
	s.etcdRegistry.SaveUnitState(req.Name, rpcUnitStateToExtUnitState(req.State), time.Duration(req.Ttl)*time.Second)
	return &pb.GenericReply{}, nil
}

func (s *rpcserver) ScheduleUnit(ctx context.Context, unit *pb.ScheduledUnit) (*pb.GenericReply, error) {
	err := s.etcdRegistry.ScheduleUnit(unit.Name, unit.TargetMachine)
	return &pb.GenericReply{}, err
}

func rpcUnitStateToJobState(state pb.TargetState) job.JobState {
	switch state {
	case pb.TargetState_INACTIVE:
		return job.JobStateInactive
	case pb.TargetState_LOADED:
		return job.JobStateLoaded
	case pb.TargetState_LAUNCHED:
		return job.JobStateLaunched
	}
	return job.JobStateInactive
}

func (s *rpcserver) SetUnitTargetState(ctx context.Context, unit *pb.ScheduledUnit) (*pb.GenericReply, error) {
	err := s.etcdRegistry.SetUnitTargetState(unit.Name, rpcUnitStateToJobState(unit.CurrentState))
	return &pb.GenericReply{}, err
}

func (s *rpcserver) UnscheduleUnit(ctx context.Context, unit *pb.ScheduledUnit) (*pb.GenericReply, error) {
	err := s.etcdRegistry.UnscheduleUnit(unit.Name, unit.TargetMachine)
	return &pb.GenericReply{}, err
}

func jobScheduledUnitToRPC(u *job.ScheduledUnit) *pb.ScheduledUnit {
	return &pb.ScheduledUnit{
		Name:          u.Name,
		CurrentState:  unitStateToRPC(*u.State),
		TargetMachine: u.TargetMachineID,
	}
}

func unitStateToRPC(state job.JobState) pb.TargetState {
	switch state {
	case job.JobStateInactive:
		return pb.TargetState_INACTIVE
	case job.JobStateLoaded:
		return pb.TargetState_LOADED
	case job.JobStateLaunched:
		return pb.TargetState_LAUNCHED
	}
	return pb.TargetState_LOADED
}
