package rpc

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"

	"github.com/coreos/fleet/engine"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg/lease"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/unit"
)

type RegistryMux struct {
	etcdRegistry    *registry.EtcdRegistry
	localMachine    machine.Machine
	rpcserver       *rpcserver
	currentRegistry registry.Registry
	rpcRegistry     *RPCRegistry
	currentEngine   machine.MachineState
	leaseManager    lease.Manager

	handlingEngineChange *sync.RWMutex
}

const (
	dialRegistryReconnectTimeout = 200 * time.Millisecond

	engineLeaderKeyPath = "engine-leader"
)

func NewRegistryMux(etcdRegistry *registry.EtcdRegistry, localMachine machine.Machine, leaseManager lease.Manager) *RegistryMux {
	return &RegistryMux{
		etcdRegistry:         etcdRegistry,
		localMachine:         localMachine,
		handlingEngineChange: new(sync.RWMutex),
		leaseManager:         leaseManager,
	}
}

// ConnectToRegistry allows to disable_engine fleet agents to adapt its Registry
// to fleet leader changes regardless of whether is etcd or gRPC based.
func (r *RegistryMux) ConnectToRegistry(e *engine.Engine) {
	for {
		// We have to check if the leader has changed to etcd otherwise keep grpc connection
		isGrpc, err := e.IsGrpcLeader()
		// If there is not error then we are able to get the leader state and continue
		// otherwise we have to wait
		if err == nil {
			if isGrpc {
				if r.rpcRegistry != nil && r.rpcRegistry.IsRegistryReady() {
					log.Infof("Reusing gRPC engine, connection is READY\n")
					r.currentRegistry = r.rpcRegistry
				} else {
					if r.rpcRegistry != nil {
						r.rpcRegistry.Close()
					}
					log.Infof("New engine supports gRPC, connecting\n")
					r.rpcRegistry = NewRPCRegistry(r.rpcDialerNoEngine)
					// connect to rpc registry
					r.rpcRegistry.Connect()
					r.currentRegistry = r.rpcRegistry
				}
			} else {
				if r.rpcRegistry != nil {
					r.rpcRegistry.Close()
				}
				// new leader is etcd-based
				r.currentRegistry = r.etcdRegistry
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func (r *RegistryMux) rpcDialerNoEngine(_ string, timeout time.Duration) (net.Conn, error) {
	ticker := time.Tick(dialRegistryReconnectTimeout)
	// Timeout re-defined to call etcd every 5secs to get the leader
	timeout = 5 * time.Second
	check := time.After(timeout)

	for {
		select {
		case <-check:
			log.Errorf("Unable to connect to engine %s\n", r.currentEngine.PublicIP)
			// Get the new engine leader of the cluster out of etcd
			lease, err := r.leaseManager.GetLease(engineLeaderKeyPath)
			// Key found
			if err == nil && lease != nil {
				var err error
				machines, err := r.etcdRegistry.Machines()
				if err != nil {
					log.Errorf("Unable to get the machines of the cluster %v\n", err)
					return nil, errors.New("Unable to get the machines of the cluster")
				}
				for _, s := range machines {
					// Update the currentEngine with the new one... otherwise wait until
					// there is one
					if s.ID == lease.MachineID() {
						// New leader has not gRPC capabilities enabled.
						if !s.Capabilities.Has(machine.CapGRPC) {
							log.Error("New leader engine has not gRPC enabled!")
							return nil, errors.New("New leader engine has not gRPC enabled!")
						}
						r.currentEngine = s
						log.Infof("Found a new engine to connect to: %s\n", r.currentEngine.PublicIP)
						// Restore initial check configuration
						timeout = 5 * time.Second
						check = time.After(timeout)
					}
				}
			} else {
				timeout = 2 * time.Second
				log.Errorf("Unable to get the leader engine, retrying in %v...", timeout)
				check = time.After(timeout)
			}
		case <-ticker:
			addr := fmt.Sprintf("%s:%d", r.currentEngine.PublicIP, rpcServerPort)
			conn, err := net.Dial("tcp", addr)
			if err == nil {
				log.Infof("Connected to engine on %s\n", r.currentEngine.PublicIP)
				return conn, nil
			}
			log.Errorf("Retry to connect to new engine: %+v", err)
		}
	}
}

func (r *RegistryMux) rpcDialer(_ string, timeout time.Duration) (net.Conn, error) {
	ticker := time.Tick(dialRegistryReconnectTimeout)
	alert := time.After(timeout)

	for {
		select {
		case <-alert:
			log.Errorf("Unable to connect to engine %s\n", r.currentEngine.PublicIP)
			return nil, errors.New("Unable to connect to new engine, the client connection is closing")
		case <-ticker:
			addr := fmt.Sprintf("%s:%d", r.currentEngine.PublicIP, rpcServerPort)
			conn, err := net.Dial("tcp", addr)
			if err == nil {
				log.Infof("Connected to engine on %s\n", r.currentEngine.PublicIP)
				return conn, nil
			}
			log.Errorf("Retry to connect to new engine: %+v", err)
		}
	}
}

func (r *RegistryMux) EngineChanged(newEngine machine.MachineState) {
	r.handlingEngineChange.Lock()
	defer r.handlingEngineChange.Unlock()

	stopServer := false
	if r.currentEngine.ID != newEngine.ID {
		stopServer = true
	}
	r.currentEngine = newEngine
	log.Infof("Engine changed, checking capabilities %+v", newEngine)
	if r.localMachine.State().Capabilities.Has(machine.CapGRPC) {
		if r.rpcserver != nil && ((r.rpcRegistry != nil && !r.rpcRegistry.IsRegistryReady()) || stopServer) {
			// If the engine changed, we need to stop the rpc server
			r.rpcserver.Stop()
			r.rpcserver = nil
		}
		if newEngine.ID == r.localMachine.State().ID {
			if r.rpcserver == nil {
				// start rpc server
				log.Infof("Starting rpc server...\n")
				var err error
				r.rpcserver, err = NewRPCServer(r.etcdRegistry, newEngine.PublicIP)
				if err != nil {
					log.Fatalf("Unable to create rpc server %+v", err)
				}

				go func() {
					errc := make(chan error, 1)
					if errc <- r.rpcserver.Start(); <-errc != nil {
						log.Fatalf("Failed to serve gRPC requests on listener: %v", <-errc)
					}
				}()
			}

		}
		if newEngine.Capabilities.Has(machine.CapGRPC) {
			if r.rpcRegistry != nil && r.rpcRegistry.IsRegistryReady() {
				log.Infof("Reusing gRPC engine, connection is READY\n")
				r.currentRegistry = r.rpcRegistry
			} else {
				log.Infof("New engine supports gRPC, connecting\n")
				r.rpcRegistry = NewRPCRegistry(r.rpcDialer)
				// connect to rpc registry
				r.rpcRegistry.Connect()
				r.currentRegistry = r.rpcRegistry
			}
		} else {
			log.Infof("Falling back to etcd registry\n")
			if r.rpcserver != nil {
				// If the engine changed to a non gRPC leader, we need to stop the server
				r.rpcserver.Stop()
			}
			r.currentRegistry = r.etcdRegistry
		}

	} else {
		log.Infof("Falling back to etcd registry\n")
		r.currentRegistry = r.etcdRegistry
	}
}

func (r *RegistryMux) getRegistry() registry.Registry {
	r.handlingEngineChange.RLock()
	defer r.handlingEngineChange.RUnlock()
	if r.currentRegistry == nil {
		return r.etcdRegistry
	}
	return r.currentRegistry
}

func (r *RegistryMux) IsRegistryReady() bool {
	return r.getRegistry().IsRegistryReady()
}

func (r *RegistryMux) UseEtcdRegistry() bool {
	return r.getRegistry().UseEtcdRegistry()
}

func (r *RegistryMux) ClearUnitHeartbeat(name string) {
	r.getRegistry().ClearUnitHeartbeat(name)
}

func (r *RegistryMux) CreateUnit(unit *job.Unit) error {
	return r.getRegistry().CreateUnit(unit)
}

func (r *RegistryMux) CreateMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	return r.getRegistry().CreateMachineState(ms, ttl)
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
