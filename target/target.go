package target

import (
	"log"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/dbus"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

const (
	systemdRuntimePath = "/run/systemd/system/"
)

type Target struct {
	Name	string
	Systemd *systemdDbus.Conn
	SystemdTarget *SystemdTarget
	Machine *machine.Machine
}

func New(machine *machine.Machine) *Target {
	systemd := systemdDbus.New()

	name := "coreinit-" + machine.BootId + ".target"
	st := NewSystemdTarget(name)
	target := &Target{name, systemd, st, machine}

	return target
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

func (t *Target) GetJobs() map[string]job.Job {
	object := unitPath(t.Name)
	info, err := t.Systemd.GetUnitInfo(object)

	if err != nil {
		panic(err)
	}

	names := info["Wants"].Value().([]string)
	jobs := make(map[string]job.Job, len(names))

	for _, name := range names {
		state := t.GetJobState(name)
		j, _ := job.NewJob(name, state, nil)
		jobs[name] = *j
	}

	return jobs
}

func (t *Target) GetJobState(name string) *job.JobState {
	unit := *UnitFactory(t.Systemd, name)
	state, sockets, err := unit.State()
	if err != nil {
		log.Printf("Failed to get state for job %s", name)
		return nil
	} else {
		return job.NewJobState(state, sockets, t.Machine)
	}
}

func (t *Target) StartJob(job *job.Job) {
	//This is probably not the right place to force the service to be
	// WantedBy our systemd target
	job.Payload.Value += "\r\n\r\n[Install]\r\nWantedBy=" + t.Name

	ss := NewSystemdService(t.Systemd, job.Name, job.Payload.Value)
	writeUnit(ss.Name(), job.Payload.Value)
	startUnit(ss.Name(), t.Systemd)
}

func (t *Target) StopJob(job *job.Job) {
	stopUnit(job.Name, t.Systemd)
	removeUnit(job.Name, t.Name)
}

type SystemdTarget struct {
	Name string
}

func NewSystemdTarget(name string) *SystemdTarget {
	tgt := SystemdTarget{name}
	tgt.persist()
	return &tgt
}

func (st *SystemdTarget) persist() error {
	return writeUnit(st.Name, "")
}
