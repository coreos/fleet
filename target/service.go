package target

import (
	systemdDbus "github.com/coreos/go-systemd/dbus"
)


type SystemdService struct {
	Systemd *systemdDbus.Conn
	name string
}

func NewSystemdService(systemd *systemdDbus.Conn, name string, contents string) *SystemdService {
	return &SystemdService{systemd, name}
}

func (ss *SystemdService) Name() string {
	return ss.name
}

func (ss *SystemdService) State() (string, []string, error) {
	state, err := getUnitState(ss.name, ss.Systemd)
	if err != nil {
		return "", nil, err
	}

	return state, make([]string, 0), nil
}
