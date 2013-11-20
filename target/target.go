package target

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
	systemdRuntimePath = "/run/systemd/system/"
)

type Target struct {
	Name	string
	Systemd *systemdDbus.Conn
	Machine *machine.Machine
}

func New(machine *machine.Machine) *Target {
	systemd := systemdDbus.New()

	name := "coreinit-" + machine.BootId + ".target"
	target := &Target{name, systemd, machine}
	createSystemdTarget(name)

	return target
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
		payload := job.NewJobPayload(readUnit(name))
		state := t.GetJobState(name)
		jobs[name] = *job.NewJob(name, state, payload)
	}

	return jobs
}

func (t *Target) GetJobState(name string) *job.JobState {
	info, err := t.Systemd.GetUnitInfo(unitPath(name))

	if err != nil {
		return nil
	}

	stateString := info["ActiveState"].Value().(string)
	return job.NewJobState(stateString, t.Machine)
}

func (t *Target) StartJob(job *job.Job) {
	createSystemdService(job.Name, job.Payload.Value, t.Name)
	t.startUnit(job.Name)
}

func (t *Target) StopJob(job *job.Job) {
	t.stopUnit(job.Name)
	t.removeUnit(job.Name)
}

func (t *Target) startUnit(name string) {
	log.Println("Starting systemd unit", name)

	files := []string{name}
	t.Systemd.EnableUnitFiles(files, true, false)

	t.Systemd.StartUnit(name, "replace")
}

func (t *Target) stopUnit(name string) {
	log.Println("Stopping systemd unit", name)

	t.Systemd.StopUnit(name, "replace")

	// go-systemd does not yet have this implemented
	//files := []string{name}
	//t.Systemd.DisableUnitFiles(files, true, false)
}

func createSystemdService(name string, contents string, target string) {
	log.Println("Writing systemd service file", name)

	path := path.Join(systemdRuntimePath, name)
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	contents += "\r\n\r\n[Install]\r\nWantedBy=" + target

	file.Write([]byte(contents))
}

// Ensure a local systemd target file exists. The name
// argument must end with '.target'
func createSystemdTarget(name string) {
	path := path.Join(systemdRuntimePath, name)
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	file.Close()
}

func (t *Target) removeUnit(name string) {
	log.Printf("Unlinking systemd unit %s from target %s", name, t.Name)
	link := path.Join(systemdRuntimePath, t.Name + ".wants", name)
	syscall.Unlink(link)
}

func readUnit(name string) string {
	path := path.Join(systemdRuntimePath, name)
	contents, _ := ioutil.ReadFile(path)
	return string(contents)
}

func unitPath(unit string) dbus.ObjectPath {
	prefix := "/org/freedesktop/systemd1/unit/"

	// This encoding should move to go-systemd.
	// See https://github.com/coreos/go-systemd/issues/13
	unit = strings.Replace(unit, ".", "_2e", -1)
	unit = strings.Replace(unit, "-", "_2d", -1)

	unitPath := path.Join(prefix, unit)
	return dbus.ObjectPath(unitPath)
}
