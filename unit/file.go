package unit

import (
	"fmt"
	"strings"
)

type SystemdUnitFile struct {
	Options map[string]map[string][]string
}

func (self *SystemdUnitFile) GetSection(name string) map[string][]string {
	if options, ok := self.Options[name]; ok {
		return options
	} else {
		return make(map[string][]string, 0)
	}
}

func (self *SystemdUnitFile) ReplaceField(section string, key string, value string) {
	if _, ok := self.Options[section]; !ok {
		self.Options[section] = make(map[string][]string, 0)
	}

	self.Options[section][key] = []string{value}
}

func (self *SystemdUnitFile) Requirements() map[string][]string {
	requirements := make(map[string][]string, 0)
	for key, value := range self.GetSection("X-Fleet") {
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

func (self *SystemdUnitFile) Description() string {
	values, ok := self.GetSection("Unit")["Description"]
	if ok && len(values) > 0 {
		return values[0]
	} else {
		return ""
	}
}

func (self *SystemdUnitFile) String() (serialized string) {
	for section, options := range self.Options {
		serialized += fmt.Sprintf("[%s]\n", section)
		for key, values := range options {
			for _, val := range values {
				serialized += fmt.Sprintf("%s=%s\n", key, val)
			}
		}
		serialized += "\n"
	}
	return
}

func NewSystemdUnitFile(contents string) *SystemdUnitFile {
	parsed := deserializeUnitFile(contents)
	return &SystemdUnitFile{Options: parsed}
}

// deserializeUnitFile is dangerously simple and should be rewritten to match the systemd unit file spec
func deserializeUnitFile(contents string) map[string]map[string][]string {
	sections := make(map[string]map[string][]string, 0)
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
			sections[section] = make(map[string][]string, 0)
			continue
		}

		// Check for key=value
		if strings.ContainsAny(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.Trim(parts[0], " ")
			value := strings.Trim(parts[1], " ")

			if len(section) > 0 {
				if _, ok := sections[section][key]; !ok {
					sections[section][key] = make([]string, 0)
				}

				sections[section][key] = append(sections[section][key], value)
			}

		}
	}

	return sections
}
