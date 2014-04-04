package job

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/fleet/unit"
)

type JobPayload struct {
	Name string
	Unit unit.SystemdUnitFile
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
	peers, ok := jp.Unit.Requirements()[unit.FleetXConditionMachineOf]

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

func (jp *JobPayload) UnmarshalJSON(data []byte) error {
	var jpm jobPayloadModel
	err := json.Unmarshal(data, &jpm)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to JSON-deserialize object: %s", err))
	}

	if len(jpm.Unit.Raw) > 0 {
		jp.Unit = *unit.NewSystemdUnitFile(jpm.Unit.Raw)
	} else {
		jp.Unit = *unit.NewSystemdUnitFileFromLegacyContents(jpm.Unit.Contents)
	}

	jp.Name = jpm.Name
	return nil
}

func (jp *JobPayload) MarshalJSON() ([]byte, error) {
	ufm := unitFileModel{
		Contents: jp.Unit.LegacyContents(),
		Raw:      jp.Unit.String(),
	}
	jpm := jobPayloadModel{Name: jp.Name, Unit: ufm}
	return json.Marshal(jpm)
}

// unitFileModel is just used for serialization
type unitFileModel struct {
	// Contents is now a legacy field, only read by older instances of fleet
	Contents map[string]map[string]string
	Raw      string
}

// jobPayloadModel is just used for serialization
type jobPayloadModel struct {
	Name string
	Unit unitFileModel
}
