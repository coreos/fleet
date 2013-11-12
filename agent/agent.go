package agent

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
	"github.com/guelfey/go.dbus"
	systemdDbus "github.com/coreos/go-systemd/dbus"
)

const (
	DefaultServiceTTL = "2s"
	DefaultMachineTTL = "10s"
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

func (a *Agent) DoHeartbeat() {
	go a.doServiceHeartbeat()
	a.doMachineHeartbeat()
	return
}

// Keep the local statistics in the Registry up to date
func (a *Agent) doMachineHeartbeat() {
	interval := intervalFromTTL(DefaultMachineTTL)

	c := time.Tick(interval)
	for _ = range c {
		log.Println("tick machine heartbeat")
		a.UpdateMachine()
	}
}

func (a *Agent) UpdateMachine() {
	addrs := a.Machine.GetAddresses()
	ttl := parseDuration(DefaultMachineTTL)
	log.Println("Updating machine", a.Machine, "with addrs", addrs)
	a.Registry.SetMachineAddrs(a.Machine, addrs, ttl)
}

// Keep the state of local units in the Registry up to date
func (a *Agent) doServiceHeartbeat() {
	interval := intervalFromTTL(a.ServiceTTL)

	c := time.Tick(interval)
	for _ = range c {
		log.Println("tick service heartbeat")
		a.UpdateUnits()
	}
}

func (a *Agent) UpdateUnits() {
	registeredUnits := a.Registry.GetScheduledUnits(a.Machine)
	localUnits := a.getLocalUnits()

	for unitName, unitValue := range registeredUnits {
		writeLocalUnit(unitName, unitValue)
		a.startUnit(unitName)
	}

	d := parseDuration(a.ServiceTTL)
	for u, _ := range(localUnits) {
		_, ok := registeredUnits[u]

		if ok {
			state := a.getLocalUnitState(u)

			if state == "active" {
				log.Println("Updating unit state:", u, state)
				a.Registry.SetUnitState(a.Machine, u, state, uint64(d.Seconds()))
			}
		} else {
			a.stopUnit(u)
		}
	}
}

func (a *Agent) getLocalUnits() map[string]string {
	object := unitPath("local.target")
	info, err := a.Systemd.GetUnitInfo(object)

	if err != nil {
		panic(err)
	}

	names := info["Wants"].Value().([]string)
	units := make(map[string]string, len(names))

	for _, name := range names {
		units[name] = readLocalUnit(name)
	}

	return units
}

func (a *Agent) getLocalUnitState(name string) string {
	info, err := a.Systemd.GetUnitInfo(unitPath(name))
	if err != nil {
		panic(err)
	}

	return info["ActiveState"].Value().(string)
}

func (a *Agent) startUnit(name string) {
	log.Println("Starting unit", name)

	files := []string{name}
	a.Systemd.EnableUnitFiles(files, true, false)

	a.Systemd.StartUnit(name, "replace")
}

func (a *Agent) stopUnit(name string) {
	log.Println("Stopping unit", name)

	a.Systemd.StopUnit(name, "replace")

	link := path.Join(systemdRuntimePath, "local.target.wants", name)
	syscall.Unlink(link)

	// This is probably the better way to remove a unit file from the
	// system, but go-systemd does not yet have this implemented.
	//files := []string{name}
	//a.Systemd.DisableUnitFiles(files, true, false)
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

func writeLocalUnit(name string, contents string) {
	log.Println("Creating unit", name)
	path := path.Join(systemdRuntimePath, name)
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	file.WriteString(contents)
	file.Close()
}

func readLocalUnit(name string) string {
	path := path.Join(systemdRuntimePath, name)
	contents, _ := ioutil.ReadFile(path)
	return string(contents)
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

func unitPath(unit string) dbus.ObjectPath {
	prefix := "/org/freedesktop/systemd1/unit/"
	split := strings.Split(unit, ".")
	unit = strings.Join(split, "_2e")
	unitPath := path.Join(prefix, unit)
	return dbus.ObjectPath(unitPath)
}
