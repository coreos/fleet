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

	"github.com/coreos/coreinit/event"
	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

const (
	defaultSystemdRuntimePath = "/run/systemd/system/"
)

type SystemdManager struct {
	Systemd    *systemdDbus.Conn
	Target     *SystemdTarget
	Machine    *machine.Machine
	UnitPrefix string
	unitPath   string

	subscriptions  *systemdDbus.SubscriptionSet
	stop chan bool
}

func NewSystemdManager(machine *machine.Machine, unitPrefix string) *SystemdManager {
	//TODO(bcwaldon): Handle error in call to New()
	systemd, _ := systemdDbus.New()
	subscriptions := systemd.NewSubscriptionSet()

	name := "coreinit-" + machine.BootId + ".target"
	target := NewSystemdTarget(name)

	mgr := &SystemdManager{systemd, target, machine, unitPrefix, defaultSystemdRuntimePath, subscriptions, nil}
	mgr.writeUnit(target.Name(), "")

	return mgr
}

func (m *SystemdManager) Publish(bus *event.EventBus) {
	m.stop = make(chan bool)

	m.Systemd.Subscribe()
	changechan, errchan := m.subscriptions.Subscribe()

	stream := NewEventStream()
	stream.Stream(changechan, bus.Channel)

	for true {
		select {
		case <-m.stop:
			break
		case err := <-errchan:
			var errString string
			if err != nil {
				errString = err.Error()
			} else {
				errString = "N/A"
			}
			log.Errorf("Received error from dbus: err=%s", errString)
		}
	}

	stream.Close()
	m.Systemd.Unsubscribe()
}

func (m *SystemdManager) Stop() {
	close(m.stop)
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
	info, err := m.Systemd.GetUnitProperties(target.Name())

	if err != nil {
		panic(err)
	}

	names := info["Wants"].([]string)

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
		j := job.NewJob(name, state, nil)
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
	unitFile := NewSystemdUnitFile(job.Payload.Value)
	unitFile.SetField("Install", "WantedBy", m.Target.Name())

	name := m.addUnitNamePrefix(job.Name)
	m.writeUnit(name, unitFile.String())

	m.subscriptions.Add(name)

	m.startUnit(name)
}

func (m *SystemdManager) StopJob(job *job.Job) {
	name := m.addUnitNamePrefix(job.Name)
	m.stopUnit(name)
	m.removeUnit(name)
}

func (m *SystemdManager) getUnitStates(name string) (string, string, string, error) {
	info, err := m.Systemd.GetUnitProperties(name)

	if err != nil {
		return "", "", "", err
	} else {
		loadState := info["LoadState"].(string)
		activeState := info["ActiveState"].(string)
		subState := info["SubState"].(string)
		return loadState, activeState, subState, nil
	}
}

func (m *SystemdManager) startUnit(name string) {
	log.V(1).Infof("Starting systemd unit %s", name)

	files := []string{name}
	m.Systemd.EnableUnitFiles(files, true, false)
	log.V(1).Infof("Enabled systemd unit %s", name)

	m.Systemd.StartUnit(name, "replace")
	log.Infof("Started systemd unit %s", name)
}

func (m *SystemdManager) stopUnit(name string) {
	log.V(1).Infof("Stopping systemd unit %s", name)

	m.Systemd.StopUnit(name, "replace")
	log.Infof("Stopped systemd unit %s", name)

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

func (m *SystemdManager) getLocalPath(name string) string {
	return path.Join(m.unitPath, name)
}

func (m *SystemdManager) addUnitNamePrefix(name string) string {
	if len(m.UnitPrefix) > 0 {
		return fmt.Sprintf("%s.%s", m.UnitPrefix, name)
	} else {
		return name
	}
}

func (m *SystemdManager) stripUnitNamePrefix(name string) string {
	if len(m.UnitPrefix) > 0 {
		return strings.TrimPrefix(name, fmt.Sprintf("%s.", m.UnitPrefix))
	} else {
		return name
	}
}
