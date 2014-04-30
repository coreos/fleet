package systemd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"syscall"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-systemd/dbus"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

const (
	defaultSystemdRuntimePath = "/run/systemd/system/"
)

type SystemdManager struct {
	Systemd  *dbus.Conn
	Machine  *machine.Machine
	unitPath string

	subscriptions *dbus.SubscriptionSet
	stop          chan bool
}

func NewSystemdManager(machine *machine.Machine) *SystemdManager {
	systemd, err := dbus.New()
	if err != nil {
		panic(err)
	}

	return &SystemdManager{systemd, machine, defaultSystemdRuntimePath, systemd.NewSubscriptionSet(), nil}
}

func (m *SystemdManager) MarshalJSON() ([]byte, error) {
	data := struct {
		DBUSSubscriptions []string
	}{
		DBUSSubscriptions: m.subscriptions.Values(),
	}
	return json.Marshal(data)
}

// Publish is a long-running function that streams dbus events through
// a translation layer and on to the EventBus
func (m *SystemdManager) Publish(bus *event.EventBus, stopchan chan bool) {
	m.Systemd.Subscribe()

	changechan, errchan := m.subscriptions.Subscribe()

	stream := NewEventStream()
	stream.Stream(changechan, bus.Channel)

	for true {
		select {
		case <-stopchan:
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

// LoadJob writes the unit of the given Job to disk, subscribes to
// relevant dbus events, and only if necessary, instructs the systemd
// daemon to reload
func (m *SystemdManager) LoadJob(job *job.Job) {
	m.writeUnit(job.Name, job.Payload.Unit.String())
	m.subscriptions.Add(job.Name)

	if m.unitRequiresDaemonReload(job.Name) {
		m.daemonReload()
	}
}

// UnloadJob removes the unit associated with the indicated Job from
// the filesystem, unsubscribing from relevant dbus events
func (m *SystemdManager) UnloadJob(jobName string) {
	m.subscriptions.Remove(jobName)
	m.removeUnit(jobName)
}

// StartJob starts the unit created for the indicated Job
func (m *SystemdManager) StartJob(jobName string) {
	m.startUnit(jobName)
}

// StopJob stops the unit created for the indicated Job
func (m *SystemdManager) StopJob(jobName string) {
	m.stopUnit(jobName)
}

// GetPayloadState generates a PayloadState object representing the
// current state of a Job's unit
func (m *SystemdManager) GetPayloadState(jobName string) (*job.PayloadState, error) {
	loadState, activeState, subState, err := m.getUnitStates(jobName)
	if err != nil {
		return nil, err
	}
	ms := m.Machine.State()
	return job.NewPayloadState(loadState, activeState, subState, nil, &ms), nil
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
	files := []string{name}
	if _, _, err := m.Systemd.EnableUnitFiles(files, true, false); err != nil {
		log.Errorf("Failed to enable systemd unit %s: %v", name, err)
		return
	} else {
		log.Infof("Enabled systemd unit %s", name)
	}

	if stat, err := m.Systemd.StartUnit(name, "replace"); err != nil {
		log.Errorf("Failed to start systemd unit %s: %v", name, err)
	} else {
		log.Infof("Started systemd unit %s(%s)", name, stat)
	}
}

func (m *SystemdManager) stopUnit(name string) {
	if stat, err := m.Systemd.StopUnit(name, "replace"); err != nil {
		log.Errorf("Failed to stop systemd unit %s: %v", name, err)
	} else {
		log.Infof("Stopped systemd unit %s(%s)", name, stat)
	}

	// go-systemd does not yet have this implemented
	//files := []string{name}
	//Systemd.DisableUnitFiles(files, true, false)
}

func (m *SystemdManager) removeUnit(name string) {
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

func (m *SystemdManager) unitRequiresDaemonReload(name string) bool {
	prop, err := m.Systemd.GetUnitProperty(name, "NeedDaemonReload")
	if prop == nil || err != nil {
		return false
	}

	return prop.Value.Value().(bool)
}

func (m *SystemdManager) daemonReload() error {
	log.Infof("Instructing systemd to reload units")
	return m.Systemd.Reload()
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
