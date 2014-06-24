package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
)

func assertErrorResponse(rr *httptest.ResponseRecorder, code int) error {
	if rr.Code != code {
		return fmt.Errorf("expected HTTP code %d, got %d", code, rr.Code)
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
