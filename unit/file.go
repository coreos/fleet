package unit

import (
	"fmt"
	"strings"
)

type SystemdUnitFile struct {
	Contents map[string]map[string]string
}

func (self *SystemdUnitFile) GetSection(section string) map[string]string {
	result, ok := self.Contents[section]
	if ok {
		return result
	} else {
		return make(map[string]string, 0)
	}
}

func (self *SystemdUnitFile) SetField(section string, key string, value string) {
	_, ok := self.Contents[section]
	if !ok {
		self.Contents[section] = make(map[string]string, 0)
	}

	self.Contents[section][key] = value
}

func (self *SystemdUnitFile) Requirements() map[string][]string {
	requirements := make(map[string][]string, 0)
	for key, value := range self.GetSection("X-Coreinit") {
		if !strings.HasPrefix(key, "X-Condition") {
			continue
		}

		// Strip off leading X-Condition
		key = key[11:]

		if _, ok := requirements[key]; !ok {
			requirements[key] = make([]string, 0)
		}

		requirements[key] = append(requirements[key], value)
	}
	return requirements
}

func (self *SystemdUnitFile) String() string {
	var serialized string
	for section, keyMap := range self.Contents {
		serialized += fmt.Sprintf("[%s]\n", section)
		for key, value := range keyMap {
			serialized += fmt.Sprintf("%s=%s\n", key, value)
		}
		serialized += "\n"
	}
	return serialized
}

func NewSystemdUnitFile(Contents string) *SystemdUnitFile {
	parsed := deserializeUnitFile(Contents)
	return &SystemdUnitFile{parsed}
}

// This is dangerously simple and should be rewritten to match the spec
func deserializeUnitFile(contents string) map[string]map[string]string {
	sections := make(map[string]map[string]string, 0)
	var section string
	for _, line := range strings.Split(contents, "\n") {
		// Ignore commented-out lines
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		line = strings.Trim(line, " ")

		// Ignore blank lines
		if len(line) == 0 {
			continue
		}

		// Check for section
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1:len(line)-1]
			sections[section] = make(map[string]string, 0)
			continue
		}

		// Check for key=value
		if strings.ContainsAny(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.Trim(parts[0], " ")
			value := strings.Trim(parts[1], " ")

			if len(section) > 0 {
				sections[section][key] = value
			}

		}
	}

	return sections
}
