package unit

import (
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

func UnitFactory(manager *SystemdManager, name string) *SystemdUnit {
	var unit SystemdUnit
	contents := manager.readUnit(name)
	if strings.HasSuffix(name, ".service") {
		unit = NewSystemdService(manager, name, contents)
	} else if strings.HasSuffix(name, ".socket") {
		unit = NewSystemdSocket(manager, name, contents)
	} else {
		panic("WAT")
	}
	return &unit
}

func (m *SystemdManager) GetJobs() map[string]job.Job {
	object := m.getDbusPath(m.Target.Name())
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
	unit := *UnitFactory(m, name)
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
	link := m.getLocalPath(path.Join(m.Target.Name() + ".wants", name))
	syscall.Unlink(link)
}

func (m *SystemdManager) readUnit(name string) string {
	path := m.getLocalPath(name)
	contents, _ := ioutil.ReadFile(path)
	return string(contents)
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

	// This encoding should move to go-systemd.
	// See https://github.com/coreos/go-systemd/issues/13
	path = strings.Replace(path, ".", "_2e", -1)
	path = strings.Replace(path, "-", "_2d", -1)
	path = strings.Replace(path, "@", "_40", -1)

	return dbus.ObjectPath(path)
}

func (m *SystemdManager) getLocalPath(name string) string {
	return path.Join(m.unitPath, name)
}
