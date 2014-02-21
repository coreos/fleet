package job

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/fleet/unit"
)

type JobPayload struct {
	Name         string
	Unit         unit.SystemdUnitFile
}

func NewJobPayload(name string, uFile unit.SystemdUnitFile) *JobPayload {
	return &JobPayload{name, uFile}
}

func (jp *JobPayload) Type() (string, error) {
	if strings.HasSuffix(jp.Name, ".service") {
		return "systemd-service", nil
	} else if strings.HasSuffix(jp.Name, ".socket") {
		return "systemd-socket", nil
	} else {
		return "", errors.New(fmt.Sprintf("Unrecognized systemd unit %s", jp.Name))
	}
}

func (jp *JobPayload) Peers() []string {
	peers, ok := jp.Unit.Requirements()["ConditionMachineOf"]

	if !ok {
		jpType, err := jp.Type()
		if err == nil && jpType == "systemd-socket" {
			idx := len(jp.Name) - 7
			baseName := jp.Name[0:idx]
			peers = []string{fmt.Sprintf("%s.%s", baseName, "service")}
		}
	}

	return peers
}

func (jp *JobPayload) Conflicts() []string {
	conflicts, ok := jp.Unit.Requirements()["Conflicts"]
	if ok {
		return conflicts
	} else {
		return make([]string, 0)
	}
}

func (jp *JobPayload) Description() string {
	return jp.Unit.Description()
}
