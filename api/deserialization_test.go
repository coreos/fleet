// Copyright 2014 The fleet Authors
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

package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
)

func assertErrorResponse(rr *httptest.ResponseRecorder, code int) error {
	if rr.Code != code {
		return fmt.Errorf("expected HTTP code %d, got %d", code, rr.Code)
	}

	ctypes := rr.HeaderMap["Content-Type"]
	expect := []string{"application/json"}
	if !reflect.DeepEqual(expect, ctypes) {
		return fmt.Errorf("expected Content-Type %v, got %v", expect, ctypes)
	}

	var eresp errorResponse
	dec := json.NewDecoder(rr.Body)
	err := dec.Decode(&eresp)
	if err != nil {
		return fmt.Errorf("unable to decode error entity in body: %v", err)
	}

	if eresp.Error.Code != code {
		return fmt.Errorf("expected error entity code %d, got %d", code, eresp.Error.Code)
	}

	return nil
}
