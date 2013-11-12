package agent

import (
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
	"github.com/guelfey/go.dbus"
	systemdDbus "github.com/coreos/go-systemd/dbus"
)

const (
	DefaultServiceTTL = "10s"
	DefaultMachineTTL = "20m"
	DefaultScheduleTTL = "1s"
	refreshInterval = 2 // Refresh TTLs at 1/2 the TTL length
	systemdRuntimePath = "/run/systemd/system/"
)

// The Agent owns all of the coordination between the Registry and
// local services like systemd. Additionally, it handles the local Machine
// heartbeat and statistics.
type Agent struct {
	Registry *registry.Registry
	Systemd *systemdDbus.Conn
	Machine *machine.Machine
	ServiceTTL string
}

// DoHeartbeat ensures that all of the units are registered at an interval of
// half of the TTL.
func (a *Agent) DoHeartbeat() {
	go a.doServiceHeartbeat()
	go a.doMachineHeartbeat()
	a.doScheduler()
	return
}

func parseDuration(d string) time.Duration {
	duration, err := time.ParseDuration(d)
	if err != nil {
		panic(err)
	}

	return duration
}

func intervalFromTTL(ttl string) time.Duration {
	duration := parseDuration(ttl)
	return duration / refreshInterval
}

func (a *Agent) doServiceHeartbeat() {
	interval := intervalFromTTL(a.ServiceTTL)

	c := time.Tick(interval)
	for _ = range c {
		log.Println("tick service heartbeat")
		a.SetAllUnits()
	}
}

func (a *Agent) doMachineHeartbeat() {
	interval := intervalFromTTL(DefaultMachineTTL)

	c := time.Tick(interval)
	for _ = range c {
		log.Println("tick machine heartbeat")
		a.SetMachine()
	}
}

func (a *Agent) SetMachine() {
	addrs := a.Machine.GetAddresses()
	ttl := parseDuration(DefaultMachineTTL)
	log.Println("Updating machine", a.Machine, "with addrs", addrs)
	a.Registry.SetMachineAddrs(a.Machine, addrs, ttl)
}

func unitPath(unit string) dbus.ObjectPath {
	prefix := "/org/freedesktop/systemd1/unit/"
	split := strings.Split(unit, ".")
	unit = strings.Join(split, "_2e")
	unitPath := path.Join(prefix, unit)
	return dbus.ObjectPath(unitPath)
}

func (a *Agent) doScheduler() {
	interval := intervalFromTTL(DefaultScheduleTTL)

	c := time.Tick(interval)
	for _ = range c {
		log.Println("tick scheduler heartbeat")
		a.scheduleUnits()
	}
}

// This simply pops all of the service files from a known location in etcd and
// starts them on the local machine and deletes the key from the list. This is
// quite dumb and prone to race conditions.
func (a *Agent) scheduleUnits() {
	units := a.Registry.GetMachineUnits(a.Machine)
	for unitName, unitValue := range units {
		createUnit(unitName, unitValue)
		a.startUnit(unitName)
		a.Registry.DeleteMachineUnit(a.Machine, unitName)
	}
}

func NewAgent(registry *registry.Registry, ttl string) (*Agent) {
	mach := machine.NewMachine("")
	systemd := systemdDbus.New()

	if ttl == "" {
		ttl = DefaultServiceTTL
	}

	agent := &Agent{registry, systemd, mach, ttl}

	return agent
}

func createUnit(name string, contents string) {
	log.Println("Creating unit", name)
	path := path.Join(systemdRuntimePath, name)
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	file.WriteString(contents)
	file.Close()
}

func (a *Agent) startUnit(name string) {
	log.Println("Starting unit", name)

	files := []string{name}
	a.Systemd.EnableUnitFiles(files, true, false)

	a.Systemd.StartUnit(name, "replace")
}

func (a *Agent) SetAllUnits() {
	object := unitPath("local.target")
	info, err := a.Systemd.GetUnitInfo(object)
	if err != nil {
		panic(err)
	}

	localUnits := info["Wants"].Value().([]string)

	d := parseDuration(a.ServiceTTL)
	for _, u := range(localUnits) {
		info, err := a.Systemd.GetUnitInfo(unitPath(u))
		if err != nil {
			panic(err)
		}

		state := info["ActiveState"].Value().(string)

		if state == "active" {
			log.Println("Updating unit state:", u, state)
			a.Registry.SetMachineUnitState(a.Machine, u, state, uint64(d.Seconds()))
		}
	}
}


