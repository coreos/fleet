package machine

import (
	"io/ioutil"
	"strings"

	log "github.com/golang/glog"
)

const boot_id_path = "/proc/sys/kernel/random/boot_id"

type Machine struct {
	BootId   string
	PublicIP string
	Metadata map[string]string
}

func New(bootId string, publicIP string, metadata map[string]string) (m *Machine) {
	if bootId == "" {
		bootId = ReadLocalBootId()
	}
	return &Machine{bootId, publicIP, metadata}
}

func (m *Machine) String() string {
	return m.BootId
}

func (m *Machine) HasMetadata(metadata map[string][]string) bool {
	for key, values := range metadata {
		local, ok := m.Metadata[key]
		if !ok {
			log.V(1).Infof("No local values found for Metadata(%s)", key)
			return false
		}

		log.V(2).Infof("Asserting local Metadata(%s) meets requirements", key)

		var localMatch bool
		for _, val := range values {
			if local == val {
				log.V(1).Infof("Local Metadata(%s) meets requirement", key)
				localMatch = true
			}
		}

		if !localMatch {
			log.V(1).Infof("Local Metadata(%s) does not match requirement", key)
			return false
		}
	}

	return true
}

func ReadLocalBootId() string {
	id, err := ioutil.ReadFile(boot_id_path)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(id))
}
