/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package unit

import (
	"fmt"
)

// NewUnitFromLegacyContents creates a Unit object from an obsolete unit
// file datastructure. This should only be used to remain backwards-compatible where necessary.
func NewUnitFromLegacyContents(contents map[string]map[string]string) (*UnitFile, error) {
	var serialized string
	for section, keyMap := range contents {
		serialized += fmt.Sprintf("[%s]\n", section)
		for key, value := range keyMap {
			serialized += fmt.Sprintf("%s=%s\n", key, value)
		}
		serialized += "\n"
	}
	return NewUnitFile(serialized)
}
