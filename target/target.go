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
)

const (
	systemdRuntimePath = "/run/systemd/system/"
)

type Target struct {
	Systemd *systemdDbus.Conn
}

func New() *Target {
	systemd := systemdDbus.New()
	return &Target{systemd}
}

func (t *Target) GetJobs() map[string]job.Job {
	object := unitPath("local.target")
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
		panic(err)
	}

	stateString := info["ActiveState"].Value().(string)
	return job.NewJobState(stateString)
}

func (t *Target) StartJob(job *job.Job) {
	writeUnit(job.Name, job.Payload.Value)
	t.startUnit(job.Name)
}

func (t *Target) StopJob(job *job.Job) {
	t.stopUnit(job.Name)
	removeUnit(job.Name)
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

func writeUnit(name string, contents string) {
	log.Println("Writing systemd unit", name)
	path := path.Join(systemdRuntimePath, name)
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	file.WriteString(contents)
	file.Close()
}

func removeUnit(name string) {
	log.Println("Removing systemd unit", name)
	link := path.Join(systemdRuntimePath, "local.target.wants", name)
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
	split := strings.Split(unit, ".")
	unit = strings.Join(split, "_2e")

	unitPath := path.Join(prefix, unit)
	return dbus.ObjectPath(unitPath)
}
