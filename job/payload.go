package job

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/fleet/unit"
)

// Fleet specific unit file requirement keys.
// "X-" prefix only appears in unit file and dropped
// in code before value is used.
const (
	// Require the unit be scheduled to a specific machine defined by given boot ID.
	FleetXConditionMachineBootID = "ConditionMachineBootID"
	// Limit eligible machines to the one that hosts a specific unit.
	FleetXConditionMachineOf = "ConditionMachineOf"
	// Prevent a unit from being collocated with other units using glob-matching on the other unit names.
	FleetXConflicts = "Conflicts"
)

func SupportedUnitTypes() []string {
	return []string{"service", "socket"}
}

type JobPayload struct {
	Name string
	Unit unit.SystemdUnitFile
}

func NewJobPayload(name string, uFile unit.SystemdUnitFile) *JobPayload {
	return &JobPayload{name, uFile}
}

func (jp *JobPayload) Type() (string, error) {
	for _, ut := range SupportedUnitTypes() {
		if strings.HasSuffix(jp.Name, fmt.Sprintf(".%s", ut)) {
			return ut, nil
		}
	}

	return "", errors.New(fmt.Sprintf("Unrecognized systemd unit %s", jp.Name))
}

// Peers returns a list of Payload names that must be scheduled to the same
// machine as this Payload. If no peers were explicitly defined for certain unit
// types, a default list of peers will be returned. This behavior only applies
// to the socket unit type. For example, the default peer of foo.socket would
// be foo.service.
func (jp *JobPayload) Peers() []string {
	if peers, ok := jp.Requirements()[FleetXConditionMachineOf]; ok {
		return peers
	}

	peers := make([]string, 0)

	jpType, err := jp.Type()
	if err != nil {
		return peers
	}

	if jpType != "socket" {
		return peers
	}

	baseName := strings.TrimSuffix(jp.Name, fmt.Sprintf(".%s", jpType))
	serviceName := fmt.Sprintf("%s.%s", baseName, "service")
	peers = append(peers, serviceName)

	return peers
}

func (jp *JobPayload) Conflicts() []string {
	conflicts, ok := jp.Requirements()[FleetXConflicts]
	if ok {
		return conflicts
	} else {
		return make([]string, 0)
	}
}

// Requirements returns all relevant options from the [X-Fleet] section
// of a unit file. Relevant options are identified with a `X-` prefix in
// the unit. This prefix is stripped from relevant options before
// being returned.
func (jp *JobPayload) Requirements() map[string][]string {
	requirements := make(map[string][]string)
	for key, value := range jp.Unit.Contents["X-Fleet"] {
		if !strings.HasPrefix(key, "X-") {
			continue
		}

		// Strip off leading X-
		key = key[2:]

		if _, ok := requirements[key]; !ok {
			requirements[key] = make([]string, 0)
		}

		requirements[key] = value
	}

	return requirements
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
