// Copyright 2016 The fleet Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpc

import (
	"encoding/json"
	"net/http"
)

func dbgHandler(w http.ResponseWriter, r *http.Request) {
	if currentReg == nil {
		return
	}
	e := json.NewEncoder(w)

	data := map[string]interface{}{
		"Units":          currentReg.unitsCache,
		"ScheduledUnits": currentReg.scheduledUnits,
		"Heartbeats":     currentReg.unitHeartbeats,
		"States":         currentReg.unitStates,
	}

	e.Encode(data)
}
