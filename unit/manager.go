package unit

import (
	"log"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/dbus"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

type SystemdManager struct {
	Systemd *systemdDbus.Conn
	SystemdTarget *SystemdTarget
	Machine *machine.Machine
}

func NewSystemdManager(machine *machine.Machine) *SystemdManager {
	systemd := systemdDbus.New()

	name := "coreinit-" + machine.BootId + ".target"
	target := NewSystemdTarget(name)

	mgr := &SystemdManager{systemd, target, machine}

	return mgr
}

func UnitFactory(systemd *systemdDbus.Conn, name string) *SystemdUnit {
	var unit SystemdUnit
	contents := readUnit(name)
	if strings.HasSuffix(name, ".service") {
		unit = NewSystemdService(systemd, name, contents)
	} else if strings.HasSuffix(name, ".socket") {
		unit = NewSystemdSocket(systemd, name, contents)
	} else {
		panic("WAT")
	}
	return &unit
}

func (m *SystemdManager) GetJobs() map[string]job.Job {
	object := unitPath(m.SystemdTarget.Name)
	info, err := m.Systemd.GetUnitInfo(object)

	if err != nil {
		panic(err)
	}

	names := info["Wants"].Value().([]string)
	jobs := make(map[string]job.Job, len(names))

	for _, name := range names {
		state := m.GetJobState(name)
		j, _ := job.NewJob(name, state, nil)
		jobs[name] = *j
	}

	return jobs
}

func (m *SystemdManager) GetJobState(name string) *job.JobState {
	unit := *UnitFactory(m.Systemd, name)
	state, sockets, err := unit.State()
	if err != nil {
		log.Printf("Failed to get state for job %s", name)
		return nil
	} else {
		return job.NewJobState(state, sockets, m.Machine)
	}
}

func (m *SystemdManager) StartJob(job *job.Job) {
	//This is probably not the right place to force the service to be
	// WantedBy our systemd target
	job.Payload.Value += "\r\n\r\n[Install]\r\nWantedBy=" + m.SystemdTarget.Name

	ss := NewSystemdService(m.Systemd, job.Name, job.Payload.Value)
	writeUnit(ss.Name(), job.Payload.Value)
	startUnit(ss.Name(), m.Systemd)
}

func (m *SystemdManager) StopJob(job *job.Job) {
	stopUnit(job.Name, m.Systemd)
	removeUnit(job.Name, m.SystemdTarget.Name)
}
