package registry

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	"github.com/coreos/fleet/log"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

type RegistryMux struct {
	etcdRegistry      *EtcdRegistry
	engineChangedChan chan machine.MachineState
	localMachine      machine.Machine
	rpcserver         *rpcserver
	currentRegistry   Registry
	rpcRegistry       *RPCRegistry
	currentEngine     machine.MachineState

	handlingEngineChange *sync.RWMutex
}

const dialRegistryReconnectTimeout = 200 * time.Millisecond

func NewRegistryMux(etcdRegistry *EtcdRegistry, engineChanged chan machine.MachineState, localMachine machine.Machine) *RegistryMux {
	return &RegistryMux{
		etcdRegistry:         etcdRegistry,
		engineChangedChan:    engineChanged,
		localMachine:         localMachine,
		handlingEngineChange: new(sync.RWMutex),
	}
}

func (r *RegistryMux) StartMux() {
	go func() {
		for newEngine := range r.engineChangedChan {
			r.EngineChanged(newEngine)
		}
	}()
}

func (r *RegistryMux) rpcDialer(_ string, timeout time.Duration) (net.Conn, error) {
	for {
		addr := fmt.Sprintf("%s:%d", r.currentEngine.PublicIP, rpcServerPort)
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			log.Infof("connected to engine on %s\n", r.currentEngine.PublicIP)
			return conn, nil
		}
		log.Errorf("unable to connect to new engine: %+v", err)
		time.Sleep(dialRegistryReconnectTimeout)
	}
}

func (r *RegistryMux) EngineChanged(newEngine machine.MachineState) {
	r.handlingEngineChange.Lock()
	defer r.handlingEngineChange.Unlock()
	r.currentEngine = newEngine
	log.Infof("engine changed, checking capabilities %+v", newEngine)
	if r.localMachine.State().Capabilities.Has(machine.CapGRPC) {
		if r.rpcserver != nil {
			r.rpcserver.Stop()
			r.rpcserver = nil
		}
		if newEngine.ID == r.localMachine.State().ID {
			log.Infof("starting rpc server\n")
			// start rpc server
			rpcserver, err := newRPCServer(r.etcdRegistry, newEngine.PublicIP)
			if err != nil {
				log.Errorf("unable to create rpc server %+v", err)
			}
			r.rpcserver = rpcserver
			r.rpcserver.Start()
		}
		if newEngine.Capabilities.Has(machine.CapGRPC) {
			log.Infof("new engine supports GRPC, connecting\n")
			if r.rpcRegistry == nil {
				r.rpcRegistry = NewRPCRegistry(r.rpcDialer)
				r.rpcRegistry.Connect()
			}
			r.currentRegistry = r.rpcRegistry
			// connect to rpc registry
		} else {
			log.Infof("falling back to etcd registry\n")
			r.currentRegistry = r.etcdRegistry
		}

	} else {
		log.Infof("falling back to etcd registry\n")
		r.currentRegistry = r.etcdRegistry
	}
}

func (r *RegistryMux) getRegistry() Registry {
	r.handlingEngineChange.RLock()
	defer r.handlingEngineChange.RUnlock()
	if r.currentRegistry == nil {
		return r.etcdRegistry
	}
	return r.currentRegistry
}

func (r *RegistryMux) ClearUnitHeartbeat(name string) {
	r.getRegistry().ClearUnitHeartbeat(name)
}

func (r *RegistryMux) CreateUnit(unit *job.Unit) error {
	return r.getRegistry().CreateUnit(unit)
}

func (r *RegistryMux) DestroyUnit(unit string) error {
	return r.getRegistry().DestroyUnit(unit)
}

func (r *RegistryMux) UnitHeartbeat(name string, machID string, ttl time.Duration) error {
	return r.getRegistry().UnitHeartbeat(name, machID, ttl)
}

func (r *RegistryMux) Machines() ([]machine.MachineState, error) {
	return r.etcdRegistry.Machines()
}

func (r *RegistryMux) RemoveMachineState(machID string) error {
	return r.etcdRegistry.RemoveMachineState(machID)
}

func (r *RegistryMux) RemoveUnitState(jobName string) error {
	return r.getRegistry().RemoveUnitState(jobName)
}

func (r *RegistryMux) SaveUnitState(jobName string, unitState *unit.UnitState, ttl time.Duration) {
	r.getRegistry().SaveUnitState(jobName, unitState, ttl)
}

func (r *RegistryMux) ScheduleUnit(name string, machID string) error {
	return r.getRegistry().ScheduleUnit(name, machID)
}

func (r *RegistryMux) SetUnitTargetState(name string, state job.JobState) error {
	return r.getRegistry().SetUnitTargetState(name, state)
}

func (r *RegistryMux) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	return r.etcdRegistry.SetMachineState(ms, ttl)
}

func (r *RegistryMux) UnscheduleUnit(name string, machID string) error {
	return r.getRegistry().UnscheduleUnit(name, machID)
}

func (r *RegistryMux) Schedule() ([]job.ScheduledUnit, error) {
	return r.getRegistry().Schedule()
}

func (r *RegistryMux) ScheduledUnit(name string) (*job.ScheduledUnit, error) {
	return r.getRegistry().ScheduledUnit(name)
}

func (r *RegistryMux) Unit(name string) (*job.Unit, error) {
	return r.getRegistry().Unit(name)
}

func (r *RegistryMux) Units() ([]job.Unit, error) {
	return r.getRegistry().Units()
}

func (r *RegistryMux) UnitStates() ([]*unit.UnitState, error) {
	return r.getRegistry().UnitStates()
}

func (r *RegistryMux) LatestDaemonVersion() (*semver.Version, error) {
	return r.etcdRegistry.LatestDaemonVersion()
}

func (r *RegistryMux) EngineVersion() (int, error) {
	return r.etcdRegistry.EngineVersion()
}

func (r *RegistryMux) UpdateEngineVersion(from int, to int) error {
	return r.etcdRegistry.UpdateEngineVersion(from, to)
}
