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

	var opts map[string]map[string][]string
	if len(jpm.Unit.ContentsV1) > 0 {
		opts = jpm.Unit.ContentsV1
	} else {
		opts = mapUnitContentsV0ToV1(jpm.Unit.ContentsV0)
	}

	jp.Name = jpm.Name
	jp.Unit = unit.SystemdUnitFile{Options: opts}

	return nil
}

func (jp *JobPayload) MarshalJSON() ([]byte, error) {
	ufm := unitFileModel{
		ContentsV0: mapUnitContentsV1ToV0(jp.Unit.Options),
		ContentsV1: jp.Unit.Options,
	}
	jpm := jobPayloadModel{Name: jp.Name, Unit: ufm}
	return json.Marshal(jpm)
}

// unitFileModel is just used for serialization
type unitFileModel struct {
	ContentsV0 map[string]map[string]string   `json:"contents"`
	ContentsV1 map[string]map[string][]string `json:"contentsV1"`
}

// jobPayloadModel is just used for serialization
type jobPayloadModel struct {
	Name string
	Unit unitFileModel
}

// mapUnitContentsV0ToV1 transforms the old unit contents format to the new format
func mapUnitContentsV0ToV1(contents map[string]map[string]string) map[string]map[string][]string {
	coerced := make(map[string]map[string][]string, len(contents))
	for section, options := range contents {
		coerced[section] = make(map[string][]string, len(options))
		for key, value := range options {
			coerced[section][key] = []string{value}
		}
	}
	return coerced
}

// mapUnitContentsV1ToV0 transforms the units from the new format to the old. This
// is a lossy operation.
func mapUnitContentsV1ToV0(contents map[string]map[string][]string) map[string]map[string]string {
	coerced := make(map[string]map[string]string, len(contents))
	for section, options := range contents {
		coerced[section] = make(map[string]string, 0)
		for key, values := range options {
			if len(values) == 0 {
				continue
			}
			coerced[section][key] = values[len(values)-1]
		}
	}
	return coerced
}
