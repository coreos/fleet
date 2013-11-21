package job

import (
	"errors"
	"fmt"
	"strings"
)

func NewJobPayloadFromSystemdUnit(name string, contents string) (*JobPayload, error) {
	var payloadType string

	if strings.HasSuffix(name, ".service") {
		payloadType = "systemd-service"
	} else if strings.HasSuffix(name, ".socket") {
		payloadType = "systemd-socket"
	} else {
		return nil, errors.New(fmt.Sprintf("Unrecognized systemd unit %s", name))
	}

	return NewJobPayload(payloadType, contents)
}

