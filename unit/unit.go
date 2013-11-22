package unit

import (
	"os"
	"log"
	"path"
	"syscall"
	"io/ioutil"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/guelfey/go.dbus"
)

type SystemdUnit interface {
	Name() string
	State() (string, []string, error)
}

func startUnit(name string, Systemd *systemdDbus.Conn) {
	log.Println("Starting systemd unit", name)

	files := []string{name}
	Systemd.EnableUnitFiles(files, true, false)

	Systemd.StartUnit(name, "replace")
}

func stopUnit(name string, Systemd *systemdDbus.Conn) {
	log.Println("Stopping systemd unit", name)

	Systemd.StopUnit(name, "replace")

	// go-systemd does not yet have this implemented
	//files := []string{name}
	//Systemd.DisableUnitFiles(files, true, false)
}

func removeUnit(name string, targetName string) {
	log.Printf("Unlinking systemd unit %s from target %s", name, targetName)
	link := path.Join(systemdRuntimePath, targetName + ".wants", name)
	syscall.Unlink(link)
}

func readUnit(name string) string {
	path := path.Join(systemdRuntimePath, name)
	contents, _ := ioutil.ReadFile(path)
	return string(contents)
}

func writeUnit(name string, contents string) error {
	log.Println("Writing systemd unit file", name)

	path := path.Join(systemdRuntimePath, name)
	file, err := os.Create(path)
	defer file.Close()

	if err != nil {
		return err
	}

	file.Write([]byte(contents))
	return nil
}

func unitPath(unit string) dbus.ObjectPath {
	prefix := "/org/freedesktop/systemd1/unit/"

	// This encoding should move to go-systemd.
	// See https://github.com/coreos/go-systemd/issues/13
	unit = strings.Replace(unit, ".", "_2e", -1)
	unit = strings.Replace(unit, "-", "_2d", -1)
	unit = strings.Replace(unit, "@", "_40", -1)

	unitPath := path.Join(prefix, unit)
	return dbus.ObjectPath(unitPath)
}

func getUnitState(name string, Systemd *systemdDbus.Conn) (string, error) {
	info, err := Systemd.GetUnitInfo(unitPath(name))

	if err != nil {
		return "", err
	} else {
		return info["ActiveState"].Value().(string), nil
	}
}
