package unit

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"

	systemdDbus "github.com/coreos/go-systemd/dbus"
	log "github.com/golang/glog"
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
	var unit SystemdUnit
	if strings.HasSuffix(name, ".service") {
		unit = NewSystemdService(m, name)
	} else if strings.HasSuffix(name, ".socket") {
		unit = NewSystemdSocket(m, name)
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
			log.V(1).Infof("Unit %s seems to exist, yet unable to get corresponding SystemdUnit object", name)
		}
	}

	return units
}

func (m *SystemdManager) GetJobs() map[string]job.Job {
	units := m.getUnitsByTarget(m.Target)
	jobs := make(map[string]job.Job, len(units))
	for _, u := range units {
		state := m.getJobStateFromUnit(&u)
		name := m.stripUnitNamePrefix(u.Name())
		j, _ := job.NewJob(name, state, nil)
		jobs[j.Name] = *j
	}

	return jobs
}

func (m *SystemdManager) getJobStateFromUnit(u *SystemdUnit) *job.JobState {
	loadState, activeState, subState, sockets, err := (*u).State()
	if err != nil {
		log.V(1).Infof("Failed to get state for unit %s", (*u).Name())
		return nil
	} else {
		return job.NewJobState(loadState, activeState, subState, sockets, m.Machine)
	}
}

func (m *SystemdManager) GetJobState(j *job.Job) *job.JobState {
	name := m.addUnitNamePrefix(j.Name)
	unit, err := m.getUnitByName(name)
	if err != nil {
		log.V(1).Infof("No local unit corresponding to job %s", j.Name)
		return nil
	}

	return m.getJobStateFromUnit(unit)
}

func (m *SystemdManager) StartJob(job *job.Job) {
	//This is probably not the right place to force the service to be
	// WantedBy our systemd target
	job.Payload.Value += "\r\n\r\n[Install]\r\nWantedBy=" + m.Target.Name()

	name := m.addUnitNamePrefix(job.Name)
	m.writeUnit(name, job.Payload.Value)
	m.startUnit(name)
}

func (m *SystemdManager) StopJob(job *job.Job) {
	name := m.addUnitNamePrefix(job.Name)
	m.stopUnit(name)
	m.removeUnit(name)
}

func (m *SystemdManager) getUnitStates(name string) (string, string, string, error) {
	info, err := m.Systemd.GetUnitInfo(m.getDbusPath(name))

	if err != nil {
		return "", "", "", err
	} else {
		loadState := info["LoadState"].Value().(string)
		activeState := info["ActiveState"].Value().(string)
		subState := info["SubState"].Value().(string)
		return loadState, activeState, subState, nil
	}
}

func (m *SystemdManager) startUnit(name string) {
	log.Infof("Starting systemd unit %s", name)

	files := []string{name}
	m.Systemd.EnableUnitFiles(files, true, false)

	m.Systemd.StartUnit(name, "replace")
}

func (m *SystemdManager) stopUnit(name string) {
	log.Infof("Stopping systemd unit %s", name)

	m.Systemd.StopUnit(name, "replace")

	// go-systemd does not yet have this implemented
	//files := []string{name}
	//Systemd.DisableUnitFiles(files, true, false)
}

func (m *SystemdManager) removeUnit(name string) {
	log.Infof("Unlinking systemd unit %s from target %s", name, m.Target.Name())
	link := m.getLocalPath(path.Join(m.Target.Name()+".wants", name))
	syscall.Unlink(link)

	file := m.getLocalPath(name)
	log.Infof("Removing systemd unit file %s", file)
	syscall.Unlink(file)
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
	log.Infof("Writing systemd unit file %s", name)

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

func (m *SystemdManager) addUnitNamePrefix(name string) string {
	return fmt.Sprintf("%s.%s", m.Machine.BootId, name)
}

func (m *SystemdManager) stripUnitNamePrefix(name string) string {
	return strings.TrimPrefix(name, fmt.Sprintf("%s.", m.Machine.BootId))
}
