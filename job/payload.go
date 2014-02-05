package job

import (
	"errors"
	"fmt"
	"strings"
	
	"github.com/coreos/coreinit/unit"
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
		if err != nil && jpType == "systemd-socket" {
			idx := len(jp.Name) - 7
			baseName := jp.Name[0:idx]
			peers = []string{fmt.Sprintf("%s.%s", baseName, "service")}
		}
	}

	return peers
}
