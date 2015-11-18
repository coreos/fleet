package registry

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/rpc"
	"github.com/coreos/fleet/unit"
	"github.com/coreos/go-semver/semver"
)

const (
	port = 5059
)

type RPCRegistry struct {
	etcdRegistry         *EtcdRegistry
	leaderUpdateNotifier chan string
	localMachine         *machine.CoreOSMachine
	currentLeader        string
	listener             net.Listener
	registryClient       rpc.RegistryClient
	registryConn         *grpc.ClientConn

	eventListeners map[chan agentEvent]struct{}
	mu             *sync.Mutex
}

type agentEvent struct {
	units []string
}

func NewRPCRegistry(etcdRegistry *EtcdRegistry, leaderUpdateNotifier chan string, mach *machine.CoreOSMachine) *RPCRegistry {
	return &RPCRegistry{
		etcdRegistry:         etcdRegistry,
		leaderUpdateNotifier: leaderUpdateNotifier,
		localMachine:         mach,
		eventListeners:       map[chan agentEvent]struct{}{},
		mu:                   new(sync.Mutex),
	}
}

func (r *RPCRegistry) EventStreamer(stop chan struct{}) chan agentEvent {
	ch := make(chan agentEvent)
	r.mu.Lock()
	r.eventListeners[ch] = struct{}{}
	r.mu.Unlock()
	go func() {
		<-stop
		r.mu.Lock()
		delete(r.eventListeners, ch)
		r.mu.Unlock()
	}()
	return ch
}

func (r *RPCRegistry) broadcastEvent(ev agentEvent) {
	for ch, _ := range r.eventListeners {
		ch <- ev
	}
}

func (r *RPCRegistry) Start() {
	for leaderUpdate := range r.leaderUpdateNotifier {
		if r.currentLeader != leaderUpdate {
			r.currentLeader = leaderUpdate
			fmt.Println("XXX got new leader", leaderUpdate)
			if r.currentLeader == r.localMachine.String() {
				fmt.Println("XXX local machine is leader, doing things ")
				r.startServer()
			} else {
				if r.listener != nil {
					r.listener.Close()
				}
			}
			if r.registryConn != nil {
				r.registryConn.Close()
			}
			addr := fmt.Sprintf("%s:%d", r.findMachineAddr(r.currentLeader), port)
			var err error
			r.registryConn, err = grpc.Dial(addr, grpc.WithInsecure())
			if err != nil {
				log.Fatalf("nope: %s", err)
			}
			r.registryClient = rpc.NewRegistryClient(r.registryConn)
			go func() {
				for {
					eventsStream, err := r.registryClient.AgentEvents(context.Background(), &rpc.MachineProperties{r.localMachine.String()})
					if err != nil {
						continue
					}
					for {
						events, err := eventsStream.Recv()
						if err == io.EOF {
							break
						}
						if err != nil {
							break
						}
						r.broadcastEvent(agentEvent{events.UnitIds})
					}
				}
			}()
		}
	}
}

func (r *RPCRegistry) findMachineAddr(machineID string) string {
	machines, err := r.etcdRegistry.Machines()
	if err != nil {
		log.Println("err: unable to get machines:", err)
		return ""
	}
	for _, machine := range machines {
		if machine.ID == machineID {
			return machine.PublicIP
		}
	}
	log.Println("err: unable to find the right machine ")
	return ""
}

func (r *RPCRegistry) getClient() rpc.RegistryClient {
	for ; ; time.Sleep(50 * time.Millisecond) {
		if r.registryClient != nil {
			break
		}
	}
	return r.registryClient
}

func (r *RPCRegistry) ClearUnitHeartbeat(name string) {
	r.getClient().ClearUnitHeartbeat(context.Background(), &rpc.UnitName{name})
}

func (r *RPCRegistry) CreateUnit(j *job.Unit) error {
	_, err := r.getClient().CreateUnit(context.Background(), j.ToPB())
	return err
}

func (r *RPCRegistry) DestroyUnit(name string) error {
	_, err := r.getClient().DestroyUnit(context.Background(), &rpc.UnitName{name})
	return err
}

func (r *RPCRegistry) UnitHeartbeat(name, machID string, ttl time.Duration) error {
	_, err := r.getClient().UnitHeartbeat(context.Background(), &rpc.Heartbeat{
		Name:    name,
		Machine: machID,
		Ttl:     int32(ttl.Seconds()),
	})
	return err
}

func (r *RPCRegistry) RemoveMachineState(machID string) error {
	return r.etcdRegistry.RemoveMachineState(machID)
}

func (r *RPCRegistry) RemoveUnitState(name string) error {
	_, err := r.getClient().RemoveUnitState(context.Background(), &rpc.UnitName{name})
	return err
}

func (r *RPCRegistry) SaveUnitState(name string, unitState *unit.UnitState, ttl time.Duration) {
	r.getClient().SaveUnitState(context.Background(), &rpc.SaveUnitStateRequest{
		Name:  name,
		State: unitState.ToPB(),
		Ttl:   int32(ttl.Seconds()),
	})
}

func (r *RPCRegistry) ScheduleUnit(name, machID string) error {
	_, err := r.getClient().ScheduleUnit(context.Background(), &rpc.ScheduleUnitRequest{
		Name:    name,
		Machine: machID,
	})
	return err
}

func (r *RPCRegistry) SetUnitTargetState(name string, state job.JobState) error {
	_, err := r.getClient().SetUnitTargetState(context.Background(), &rpc.ScheduledUnit{
		Name:  name,
		State: state.ToPB(),
	})
	return err
}

func (r *RPCRegistry) UnscheduleUnit(name, machID string) error {
	_, err := r.getClient().UnscheduleUnit(context.Background(), &rpc.UnscheduleUnitRequest{
		Name:    name,
		Machine: machID,
	})
	return err
}

func (r *RPCRegistry) Machines() ([]machine.MachineState, error) {
	return r.etcdRegistry.Machines()
}

func (r *RPCRegistry) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	return r.etcdRegistry.SetMachineState(ms, ttl)
}

func (r *RPCRegistry) Schedule() ([]job.ScheduledUnit, error) {
	//if 1 == 1 {
	//return r.etcdRegistry.Schedule()
	//}
	scheduledUnits, err := r.getClient().GetScheduledUnits(context.Background(), &rpc.UnitFilter{})
	units := make([]job.ScheduledUnit, len(scheduledUnits.Units))

	for i, unit := range scheduledUnits.Units {
		state := rpcUnitStateToJobState(unit.State)
		units[i] = job.ScheduledUnit{
			Name:            unit.Name,
			TargetMachineID: unit.Machine,
			State:           &state,
		}
	}
	return units, err
}

func (r *RPCRegistry) ScheduledUnit(name string) (*job.ScheduledUnit, error) {
	scheduledUnit, err := r.getClient().GetScheduledUnit(context.Background(), &rpc.UnitName{name})

	state := rpcUnitStateToJobState(scheduledUnit.State)
	return &job.ScheduledUnit{
		Name:            scheduledUnit.Name,
		TargetMachineID: scheduledUnit.Machine,
		State:           &state,
	}, err
}

func (r *RPCRegistry) Unit(name string) (*job.Unit, error) {
	maybeUnit, err := r.getClient().GetUnit(context.Background(), &rpc.UnitName{name})
	if err != nil {
		return nil, err
	}

	if unit := maybeUnit.GetUnit(); unit != nil {
		return rpcUnitToJobUnit(unit), err
	}
	return nil, nil
}

func (r *RPCRegistry) Units() ([]job.Unit, error) {
	units, err := r.getClient().GetUnits(context.Background(), &rpc.UnitFilter{})

	jobUnits := make([]job.Unit, len(units.Units))
	for i, u := range units.Units {
		jobUnit := rpcUnitToJobUnit(u)
		jobUnits[i] = *jobUnit
	}
	return jobUnits, err
}

func (r *RPCRegistry) UnitStates() ([]*unit.UnitState, error) {
	return r.etcdRegistry.UnitStates()
}

func (r *RPCRegistry) EngineVersion() (int, error) {
	return r.etcdRegistry.EngineVersion()
}

func (r *RPCRegistry) UpdateEngineVersion(from, to int) error {
	return r.etcdRegistry.UpdateEngineVersion(from, to)
}

func (r *RPCRegistry) LatestDaemonVersion() (*semver.Version, error) {
	return r.etcdRegistry.LatestDaemonVersion()
}
