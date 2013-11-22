package unit

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"syscall"

	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/guelfey/go.dbus"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

const (
	defaultSystemdRuntimePath = "/run/systemd/system/"
	defaultSystemdDbusPath    = "/org/freedesktop/systemd1/unit/"
)

type SystemdManager struct {
	Systemd  *systemdDbus.Conn
	Target   *SystemdTarget
	Machine  *machine.Machine
	unitPath string
	dbusPath string
}

func NewSystemdManager(machine *machine.Machine) *SystemdManager {
	systemd := systemdDbus.New()

	name := "coreinit-" + machine.BootId + ".target"
	target := NewSystemdTarget(name)

	mgr := &SystemdManager{systemd, target, machine, defaultSystemdRuntimePath, defaultSystemdDbusPath}

	mgr.writeUnit(target.Name(), "")

	return mgr
}

func (m *SystemdManager) getUnitByName(name string) (*SystemdUnit, error) {
	localPayload, err := m.readUnit(name)

	if err != nil {
		return nil, err
	}

	var unit SystemdUnit
	if strings.HasSuffix(name, ".service") {
		unit = NewSystemdService(m, name, localPayload)
	} else if strings.HasSuffix(name, ".socket") {
		unit = NewSystemdSocket(m, name, localPayload)
	} else {
		panic("WAT")
	}

	return &unit, nil
}

func (m *SystemdManager) getUnitsByTarget(target *SystemdTarget) []SystemdUnit {
	object := m.getDbusPath(target.Name())
	info, err := m.Systemd.GetUnitInfo(object)

	if err != nil {
		panic(err)
	}

	names := info["Wants"].Value().([]string)

	var units []SystemdUnit
	for _, name := range names {
		unit, err := m.getUnitByName(name)
		if err == nil {
			units = append(units, *unit)
		} else {
			log.Printf("Unit %s seems to exist, yet unable to get corresponding SystemdUnit object", name)
		}
	}

	return units
}

func (m *SystemdManager) GetJobs() map[string]job.Job {
	units := m.getUnitsByTarget(m.Target)
	jobs := make(map[string]job.Job, len(units))
	for _, u := range units {
		state := m.getJobStateFromUnit(&u)
		j, _ := job.NewJob(u.Name(), state, nil)
		jobs[j.Name] = *j
	}

	return jobs
}

func (m *SystemdManager) getJobStateFromUnit(u *SystemdUnit) *job.JobState {
	state, sockets, err := (*u).State()
	if err != nil {
		log.Printf("Failed to get state for unit %s", (*u).Name())
		return nil
	} else {
		return job.NewJobState(state, sockets, m.Machine)
	}
}

func (m *SystemdManager) GetJobState(j *job.Job) *job.JobState {
	unit, err := m.getUnitByName(j.Name)
	if err != nil {
		log.Printf("No local unit corresponding to job %s", j.Name)
		return nil
	}

	return m.getJobStateFromUnit(unit)
}

func (m *SystemdManager) StartJob(job *job.Job) {
	//This is probably not the right place to force the service to be
	// WantedBy our systemd target
	job.Payload.Value += "\r\n\r\n[Install]\r\nWantedBy=" + m.Target.Name()

	ss := NewSystemdService(m, job.Name, job.Payload.Value)
	m.writeUnit(ss.Name(), job.Payload.Value)
	m.startUnit(ss.Name())
}

func (m *SystemdManager) StopJob(job *job.Job) {
	m.stopUnit(job.Name)
	m.removeUnit(job.Name)
}

func (m *SystemdManager) getUnitState(name string) (string, error) {
	info, err := m.Systemd.GetUnitInfo(m.getDbusPath(name))

	if err != nil {
		return "", err
	} else {
		return info["ActiveState"].Value().(string), nil
	}
}

func (m *SystemdManager) startUnit(name string) {
	log.Println("Starting systemd unit", name)

	files := []string{name}
	m.Systemd.EnableUnitFiles(files, true, false)

	m.Systemd.StartUnit(name, "replace")
}

func (m *SystemdManager) stopUnit(name string) {
	log.Println("Stopping systemd unit", name)

	m.Systemd.StopUnit(name, "replace")

	// go-systemd does not yet have this implemented
	//files := []string{name}
	//Systemd.DisableUnitFiles(files, true, false)
}

func (m *SystemdManager) removeUnit(name string) {
	log.Printf("Unlinking systemd unit %s from target %s", name, m.Target.Name())
	link := m.getLocalPath(path.Join(m.Target.Name()+".wants", name))
	syscall.Unlink(link)
}

func (m *SystemdManager) readUnit(name string) (string, error) {
	path := m.getLocalPath(name)
	contents, err := ioutil.ReadFile(path)
	if err == nil {
		return string(contents), nil
	} else {
		return "", errors.New(fmt.Sprintf("No unit file at local path %s", path))
	}
}

func (m *SystemdManager) writeUnit(name string, contents string) error {
	log.Println("Writing systemd unit file", name)

	path := path.Join(m.unitPath, name)
	file, err := os.Create(path)
	defer file.Close()

	if err != nil {
		return err
	}

	file.Write([]byte(contents))
	return nil
}

func (m *SystemdManager) getDbusPath(name string) dbus.ObjectPath {
	path := path.Join(m.dbusPath, name)
	path = serializeDbusPath(path)
	return dbus.ObjectPath(path)
}

func (m *SystemdManager) getLocalPath(name string) string {
	return path.Join(m.unitPath, name)
}
