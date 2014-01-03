package job

import (
	"errors"
	"fmt"
	"strings"
)

type JobPayload struct {
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	Value        string              `json:"value"`
	Peers        []string            `json:"peers"`
	Requirements map[string][]string `json:"requirements"`
}

func NewJobPayload(name string, value string, requirements map[string][]string) (*JobPayload, error) {
	var peers []string
	var pType string
	if strings.HasSuffix(name, ".service") {
		pType = "systemd-service"
	} else if strings.HasSuffix(name, ".socket") {
		pType = "systemd-socket"

		idx := len(name) - 7
		baseName := name[0:idx]
		svc := fmt.Sprintf("%s.%s", baseName, "service")
		peers = append(peers, svc)
	} else {
		return nil, errors.New(fmt.Sprintf("Unrecognized systemd unit %s", name))
	}

	return &JobPayload{name, pType, value, peers, requirements}, nil
}
