package target

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"syscall"
	"text/template"

	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/guelfey/go.dbus"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

const (
	systemdRuntimePath = "/run/systemd/system/"
)

const unitTemplate = `[Unit]
Description=coreinit job {{ .Name }}

[Service]
ExecStart={{ .Command }}

[Install]
WantedBy={{ .Target }}`

type Target struct {
	Name	string
	Systemd *systemdDbus.Conn
	Machine *machine.Machine
}

func New(machine *machine.Machine) *Target {
	name := "coreinit-" + machine.BootId
	systemd := systemdDbus.New()
	target := &Target{name, systemd, machine}
	target.createSystemdTarget(name)
	return target
}

func (t *Target) GetJobs() map[string]job.Job {
	object := unitPath(t.Name + ".target")
	info, err := t.Systemd.GetUnitInfo(object)

	if err != nil {
		panic(err)
	}

	names := info["Wants"].Value().([]string)
	jobs := make(map[string]job.Job, len(names))

	for _, name := range names {
		payload := job.NewJobPayload(readUnit(name))
		name = strings.TrimSuffix(name, ".service")
		state := t.GetJobState(name)
		jobs[name] = *job.NewJob(name, state, payload)
	}

	return jobs
}

func (t *Target) GetJobState(name string) *job.JobState {
	info, err := t.Systemd.GetUnitInfo(unitPath(name + ".service"))

	if err != nil {
		return nil
	}

	stateString := info["ActiveState"].Value().(string)
	return job.NewJobState(stateString, t.Machine)
}

func (t *Target) StartJob(job *job.Job) {
	targetName := t.Name + ".target"
	writeUnit(job.Name + ".service", job.Payload.Value, targetName)
	t.startUnit(job.Name + ".service")
}

func (t *Target) StopJob(job *job.Job) {
	t.stopUnit(job.Name + ".service")
	t.removeUnit(job.Name + ".service")
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

func writeUnit(name string, command string, target string) {
	log.Println("Writing systemd unit", name)

	path := path.Join(systemdRuntimePath, name)
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	tmpl, _ := template.New("unitTemplate").Parse(unitTemplate)
	type Data struct {
		Name string
		Command string
		Target string
	}
	context := Data{name, command, target}
	tmpl.Execute(file, context)
}

func (t *Target) createSystemdTarget(name string) {
	path := path.Join(systemdRuntimePath, name + ".target")
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	file.Close()

	t.Systemd.EnableUnitFiles([]string{path}, true, false)
}

func (t *Target) removeUnit(name string) {
	log.Println("Removing systemd unit", name)
	link := path.Join(systemdRuntimePath, t.Name + ".target.wants", name)
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
