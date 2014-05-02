package job

import (
	"fmt"
	"strings"
)

const (
	PathUnit    = "path"
	ServiceUnit = "service"
	SocketUnit  = "socket"
	TimerUnit   = "timer"
)

func SupportedUnitTypes() []string {
	return []string{PathUnit, ServiceUnit, SocketUnit, TimerUnit}
}

func newJobType(jtName string) JobType {
	switch jtName {
	case PathUnit:
		return &PathType{jtName}
	case ServiceUnit:
		return &ServiceType{jtName}
	default:
		return &JobUnitType{jtName}
	}
}

type JobType interface {
	Peers(jp *JobPayload) []string
}

type JobUnitType struct {
	name string
}

// Peers assigns the service with the same name to the unit
// for default unit type, like socket and timer.
func (jt *JobUnitType) Peers(jp *JobPayload) (peers []string) {
	peers = append(peers, servicePeer(jp.Name, jt.name))
	return
}

type PathType JobUnitType

// Peers returns the unit peers for the path job.
// The service with the same name is assigned
// when the configuration doesn't specify a `Unit` to pair with.
func (pt *PathType) Peers(jp *JobPayload) (peers []string) {
	pathSection, ok := jp.Unit.Contents["Path"]
	if !ok {
		//Section `Path` is required, no peers.
		return
	}

	if units, ok := pathSection["Unit"]; ok {
		peers = units
		return
	}

	peers = append(peers, servicePeer(jp.Name, pt.name))
	return
}

type ServiceType JobUnitType

// Peers doesn't asign units to pair a service.
// Services need to use X-ConditionMachineOf to specify
// the machines where they need to be scheduled.
func (st *ServiceType) Peers(jp *JobPayload) (peers []string) {
	return
}

func servicePeer(jpName, jtName string) string {
	baseName := strings.TrimSuffix(jpName, fmt.Sprintf(".%s", jtName))
	return fmt.Sprintf("%s.%s", baseName, "service")
}
