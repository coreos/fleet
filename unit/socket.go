package unit

import (
	"fmt"
	"errors"
	"strings"
	"log"
)

type SystemdSocket struct {
	manager *SystemdManager
	name string
	Sockets []ListenSocket
}

func NewSystemdSocket(manager *SystemdManager, name string, contents string) *SystemdSocket {
	lines := strings.Split(contents, "\n")
	listenLines := filterListenLines(lines)
	sockets := make([]ListenSocket, 0)
	for _, line := range listenLines {
		socket, err := NewListenSocketFromListenConfig(line)
		//TODO: do something more with this err
		if err == nil {
			sockets = append(sockets, *socket)
		} else {
			log.Printf("Unable to parse Listen line in socket file: %s", err)
		}
	}
	return &SystemdSocket{manager, name, sockets}
}

func (ss *SystemdSocket) Name() string {
	return ss.name
}

func (ss *SystemdSocket) State() (string, []string, error) {
	state, err := ss.manager.getUnitState(ss.name)
	if err != nil {
		return "", nil, err
	}

	sockStrings := []string{}
	for _, sock := range ss.Sockets {
		sockStrings = append(sockStrings, sock.String())
	}

	return state, sockStrings, nil
}

type ListenSocket struct {
	Type string
	Address string
}

func (ls *ListenSocket) String() string {
	return fmt.Sprintf("%s://%s", ls.Type, ls.Address)
}

func NewListenSocketFromListenConfig(cfg string) (*ListenSocket, error) {
	sockType, address, err := parseListenLine(cfg)
	if err == nil {
		return &ListenSocket{sockType, address}, nil
	} else {
		return nil, err
	}
}

func filterListenLines(lines []string) []string {
	var filtered []string
	for _, line := range lines {
		if strings.HasPrefix(line, "Listen") {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func parseListenLine(line string) (string, string, error) {
	keyMap := map[string]string{
		"ListenSequentialPacket": "unix",
		"ListenDatagram": "udp",
		"ListenStream": "tcp",
	}

	parts := strings.SplitN(line, "=", 2)
	key, ok := keyMap[parts[0]]
	if !ok {
		return "", "", errors.New(fmt.Sprintf("unrecognized key %s", parts[0]))
	}

	return key, parts[1], nil
}
