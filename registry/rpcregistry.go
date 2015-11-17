package registry

import (
	"fmt"
	"log"
	"net"
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
}

func NewRPCRegistry(etcdRegistry *EtcdRegistry, leaderUpdateNotifier chan string, mach *machine.CoreOSMachine) *RPCRegistry {
	return &RPCRegistry{
		etcdRegistry:         etcdRegistry,
		leaderUpdateNotifier: leaderUpdateNotifier,
		localMachine:         mach,
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

func (r *RPCRegistry) ClearUnitHeartbeat(name string) {
	r.etcdRegistry.ClearUnitHeartbeat(name)
}

func (r *RPCRegistry) CreateUnit(j *job.Unit) error {
	return r.etcdRegistry.CreateUnit(j)
}

func (r *RPCRegistry) DestroyUnit(name string) error {
	return r.etcdRegistry.DestroyUnit(name)
}

func (r *RPCRegistry) UnitHeartbeat(name, machID string, ttl time.Duration) error {
	return r.etcdRegistry.UnitHeartbeat(name, machID, ttl)
}

func (r *RPCRegistry) RemoveMachineState(machID string) error {
	return r.etcdRegistry.RemoveMachineState(machID)
}

func (r *RPCRegistry) RemoveUnitState(jobName string) error {
	return r.etcdRegistry.RemoveUnitState(jobName)
}

func (r *RPCRegistry) SaveUnitState(jobName string, unitState *unit.UnitState, ttl time.Duration) {
	r.etcdRegistry.SaveUnitState(jobName, unitState, ttl)
}

func (r *RPCRegistry) ScheduleUnit(name, machID string) error {
	_, err := r.registryClient.ScheduleUnit(context.Background(), &rpc.ScheduledUnit{
		Name:          name,
		TargetMachine: machID,
	})
	return err
	//return r.etcdRegistry.ScheduleUnit(name, machID)
}

func (r *RPCRegistry) SetUnitTargetState(name string, state job.JobState) error {
	return r.etcdRegistry.SetUnitTargetState(name, state)
}

func (r *RPCRegistry) UnscheduleUnit(name, machID string) error {
	return r.etcdRegistry.UnscheduleUnit(name, machID)
}

func (r *RPCRegistry) Machines() ([]machine.MachineState, error) {
	return r.etcdRegistry.Machines()
}

func (r *RPCRegistry) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	return r.etcdRegistry.SetMachineState(ms, ttl)
}

func (r *RPCRegistry) Schedule() ([]job.ScheduledUnit, error) {
	return r.etcdRegistry.Schedule()
}

func (r *RPCRegistry) ScheduledUnit(name string) (*job.ScheduledUnit, error) {
	return r.etcdRegistry.ScheduledUnit(name)
}

func (r *RPCRegistry) Unit(name string) (*job.Unit, error) {
	return r.etcdRegistry.Unit(name)
}

func (r *RPCRegistry) Units() ([]job.Unit, error) {
	return r.etcdRegistry.Units()
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
