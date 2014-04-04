package unit

import (
	"fmt"
	"strings"
)

type SystemdUnitFile struct {
	// Contents represents the parsed unit file.
	// This field must be considered readonly.
	Contents map[string]map[string][]string

	raw string
}

func (self *SystemdUnitFile) String() string {
	return self.raw
}

// LegacyContents serializes the contents of a unit file into an obsolete datastructure. This
// datastructure is lossy and should only be used to remain backwards-compatible where necessary.
func (self *SystemdUnitFile) LegacyContents() map[string]map[string]string {
	coerced := make(map[string]map[string]string, len(self.Contents))
	for section, options := range self.Contents {
		coerced[section] = make(map[string]string)
		for key, values := range options {
			if len(values) == 0 {
				continue
			}
			coerced[section][key] = values[len(values)-1]
		}
	}
	return coerced
}

// Requirements returns all relevant options from the [X-Fleet] section
// of a unit file. Relevant options are identified with a `X-` prefix.
func (self *SystemdUnitFile) Requirements() map[string][]string {
	requirements := make(map[string][]string)
	for key, value := range self.Contents["X-Fleet"] {
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

// Description returns the first Description option found in the [Unit] section.
// If the option is not defined, an empty string is returned.
func (self *SystemdUnitFile) Description() string {
	if values := self.Contents["Unit"]["Description"]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func NewSystemdUnitFile(raw string) *SystemdUnitFile {
	parsed := deserializeUnitFile(raw)
	return &SystemdUnitFile{parsed, raw}
}

// NewSystemdUnitFileFromLegacyContents creates a SystemdUnitFile object from an obsolete unit
// file datastructure. This should only be used to remain backwards-compatible where necessary.
func NewSystemdUnitFileFromLegacyContents(contents map[string]map[string]string) *SystemdUnitFile {
	var serialized string
	for section, keyMap := range contents {
		serialized += fmt.Sprintf("[%s]\n", section)
		for key, value := range keyMap {
			serialized += fmt.Sprintf("%s=%s\n", key, value)
		}
		serialized += "\n"
	}
	return NewSystemdUnitFile(serialized)
}

// deserializeUnitFile is dangerously simple and should be rewritten to match the systemd unit file spec
func deserializeUnitFile(raw string) map[string]map[string][]string {
	sections := make(map[string]map[string][]string)
	var section string
	for _, line := range strings.Split(raw, "\n") {
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
			section = line[1 : len(line)-1]
			sections[section] = make(map[string][]string)
			continue
		}

		// Check for key=value
		if strings.ContainsAny(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.Trim(parts[0], " ")
			value := strings.Trim(parts[1], " ")

			if len(section) > 0 {
				sections[section][key] = append(sections[section][key], value)
			}

		}
	}

	return sections
}
