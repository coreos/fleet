package unit

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-systemd/unit"
)

func NewUnit(raw string) (*Unit, error) {
	reader := strings.NewReader(raw)
	opts, err := unit.Deserialize(reader)
	if err != nil {
		return nil, err
	}

	contents := mapOptions(opts)
	return &Unit{contents, raw}, nil
}

func mapOptions(opts []*unit.UnitOption) map[string]map[string][]string {
	contents := make(map[string]map[string][]string)
	for _, opt := range opts {
		if _, ok := contents[opt.Section]; !ok {
			contents[opt.Section] = make(map[string][]string)
		}

		if _, ok := contents[opt.Section][opt.Name]; !ok {
			contents[opt.Section][opt.Name] = make([]string, 0)
		}

		var vals []string
		if opt.Section == "X-Fleet" {
			// The go-systemd parser does not know that our X-Fleet options support
			// multivalue options, so we have to manually parse them here
			vals = parseMultivalueLine(opt.Value)
		} else {
			vals = []string{opt.Value}
		}

		contents[opt.Section][opt.Name] = append(contents[opt.Section][opt.Name], vals...)
	}

	return contents
}

// NewUnitFromLegacyContents creates a Unit object from an obsolete unit
// file datastructure. This should only be used to remain backwards-compatible where necessary.
func NewUnitFromLegacyContents(contents map[string]map[string]string) (*Unit, error) {
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
func deserializeUnitFile(raw string) (map[string]map[string][]string, error) {
	sections := make(map[string]map[string][]string)
	var section string
	var prev string
	for i, line := range strings.Split(raw, "\n") {

		// Join lines ending in backslash
		if strings.HasSuffix(line, "\\") {
			// Replace trailing slash with space
			prev = prev + line[:len(line)-1] + " "
			continue
		}

		// Concatenate any previous conjoined lines
		if prev != "" {
			line = prev + line
			prev = ""
		} else if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			// Ignore commented-out lines that are not part of a continuation
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

		key, values, err := deserializeUnitLine(line)
		if err != nil {
			return nil, fmt.Errorf("error parsing line %d: %v", i+1, err)
		}
		for _, v := range values {
			sections[section][key] = append(sections[section][key], v)
		}
	}

	return sections, nil
}

func deserializeUnitLine(line string) (key string, values []string, err error) {
	idx := strings.Index(line, "=")
	if idx == -1 {
		err = errors.New("missing '='")
		return
	}

	key = strings.TrimSpace(line[:idx])
	values = parseMultivalueLine(strings.TrimSpace(line[idx+1:]))

	return
}

// parseMultivalueLine parses a line that includes several quoted values separated by whitespaces.
// Example: MachineMetadata="foo=bar" "baz=qux"
// If the provided line is not a multivalue string, the line is returned as the sole value.
func parseMultivalueLine(line string) (values []string) {
	if !strings.HasPrefix(line, `"`) || !strings.HasSuffix(line, `"`) {
		return []string{line}
	}

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
