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

package api

import (
	"path"
	"strings"

	"github.com/coreos/fleet/log"
)

func isCollectionPath(base, p string) bool {
	return p == base
}

func isItemPath(base, p string) (item string, matched bool) {
	if strings.HasSuffix(p, "/") {
		return
	}

	var err error
	matched, err = path.Match(path.Join(base, "*"), p)
	// err will only be non-nil in the event that our pattern is bad, not due
	// to user-provided data
	if err != nil {
		log.Errorf("Failed to determine if %q is an item path: %v", p, err)
		matched = false
	} else if matched {
		item = path.Base(p)
	}

	return
}
