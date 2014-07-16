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

// Description returns the first Description option found in the [Unit] section.
// If the option is not defined, an empty string is returned.
func (u *Unit) Description() string {
	if values := u.Contents["Unit"]["Description"]; len(values) > 0 {
		return values[0]
	}
	return ""
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
