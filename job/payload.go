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
	var defaultPeers []string
	var pType string
	if strings.HasSuffix(name, ".service") {
		pType = "systemd-service"
	} else if strings.HasSuffix(name, ".socket") {
		pType = "systemd-socket"

		idx := len(name) - 7
		baseName := name[0:idx]
		defaultPeers = []string{fmt.Sprintf("%s.%s", baseName, "service")}
	} else {
		return nil, errors.New(fmt.Sprintf("Unrecognized systemd unit %s", name))
	}

	peers, ok := requirements["Peers"]
	if !ok {
		peers = defaultPeers
	}

	return &JobPayload{name, pType, value, peers, requirements}, nil
}
