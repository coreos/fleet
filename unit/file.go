package unit

import (
	"bytes"
	"fmt"
	"strings"
)

// Description returns the first Description option found in the [Unit] section.
// If the option is not defined, an empty string is returned.
func (u *Unit) Description() string {
	if values := u.Contents["Unit"]["Description"]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func NewUnit(raw string) *Unit {
	parsed := deserializeUnitFile(raw)
	return &Unit{parsed, raw}
}

// NewUnitFromLegacyContents creates a Unit object from an obsolete unit
// file datastructure. This should only be used to remain backwards-compatible where necessary.
func NewUnitFromLegacyContents(contents map[string]map[string]string) *Unit {
	var serialized string
	for section, keyMap := range contents {
		serialized += fmt.Sprintf("[%s]\n", section)
		for key, value := range keyMap {
			serialized += fmt.Sprintf("%s=%s\n", key, value)
		}
		serialized += "\n"
	}
	return NewUnit(serialized)
}

// deserializeUnitFile parses a systemd unit file and attempts to map its various sections and values.
// Currently this function is dangerously simple and should be rewritten to match the systemd unit file spec
func deserializeUnitFile(raw string) map[string]map[string][]string {
	sections := make(map[string]map[string][]string)
	var section string
	for _, line := range strings.Split(raw, "\n") {
		// Ignore commented-out lines
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		line = strings.TrimSpace(line)

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

		// ignore any lines that aren't within a section
		if len(section) == 0 {
			continue
		}

		key, values := deserializeUnitLine(line)
		for _, v := range values {
			sections[section][key] = append(sections[section][key], v)
		}
	}

	return sections
}

func deserializeUnitLine(line string) (key string, values []string) {
	parts := strings.SplitN(line, "=", 2)
	key = strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		for _, v := range parseMultivalueLine(value) {
			values = append(values, v)
		}
	} else {
		values = append(values, value)
	}
	return
}

// parseMultivalueLine parses a line that includes several quoted values separated by whitespaces.
// Example: MachineMetadata="foo=bar" "baz=qux"
func parseMultivalueLine(line string) (values []string) {
	var v bytes.Buffer
	w := false // check whether we're within quotes or not

	for _, e := range []byte(line) {
		// ignore quotes
		if e == '"' {
			w = !w
			continue
		}

		if e == ' ' {
			if !w { // between quoted values, keep the previous value and reset.
				values = append(values, v.String())
				v.Reset()
				continue
			}
		}

		v.WriteByte(e)
	}

	values = append(values, v.String())

	return
}
