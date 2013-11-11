package registry

import (
	"path"
	"strings"
	"time"
	"net"
	"os"

	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/guelfey/go.dbus"
)

const (
	DefaultServiceTTL = "10s"
	DefaultMachineTTL = "20m"
	DefaultScheduleTTL = "1s"
	refreshInterval = 2 // Refresh TTLs at 1/2 the TTL length
	systemdRuntimePath = "/run/systemd/system/"
)

type Agent struct {
	Registry *Registry
	Systemd *systemdDbus.Conn
	Machine *Machine
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
	for now := range c {
		println(now.String())
		a.SetAllUnits()
	}
}

func (a *Agent) doMachineHeartbeat() {
	interval := intervalFromTTL(DefaultMachineTTL)

	c := time.Tick(interval)
	for now := range c {
		println(now.String())
		a.SetMachine()
	}
}

func (a *Agent) SetMachine() {
	var addrs []Addr
	ifs, err := net.Interfaces()

	if err != nil {
		panic(err)
	}

	shouldAppend := func(i net.Interface) bool {
		if (i.Flags & net.FlagLoopback) == net.FlagLoopback {
			return false
		}

		if (i.Flags & net.FlagUp) != net.FlagUp {
			return false
		}

		return true
	}

	for _, k := range(ifs) {
		if shouldAppend(k) != true {
			continue
		}
		kaddrs, _ := k.Addrs()
		for _, j := range(kaddrs) {
			if strings.HasPrefix(j.String(), "fe80::") == true {
				continue
			}
			addrs = append(addrs, Addr{j.String(), j.Network()})
		}
	}

	a.Registry.SetMachineAddrs(a.Machine, addrs)
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
	for now := range c {
		println(now.String() + " scheduling")
		a.scheduleUnits()
	}
}

func (a *Agent) scheduleUnits() {
	key := path.Join(keyPrefix, schedulePrefix)
	objects, _ := a.Registry.Etcd.Get(key)
	for _, obj := range objects {
		_, unitName := path.Split(obj.Key)
		createUnit(unitName, obj.Value)
		a.startUnit(unitName)
		a.Registry.Etcd.Delete(obj.Key)
	}
}

func NewAgent(registry *Registry, ttl string) (*Agent) {
	mach := NewMachine("")
	systemd := systemdDbus.New()

	if ttl == "" {
		ttl = DefaultServiceTTL
	}

	agent := &Agent{registry, systemd, mach, ttl}

	return agent
}

func createUnit(name string, contents string) {
	path := path.Join(systemdRuntimePath, name)
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	file.WriteString(contents)
	file.Close()
}

func (a *Agent) startUnit(name string) {
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
			println(u, state)
			a.Registry.SetUnitState(a.Machine, u, state, uint64(d.Seconds()))
		}
	}
}


