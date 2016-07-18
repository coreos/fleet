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
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
)

type PageToken struct {
	Limit uint16
	Page  uint16
}

func DefaultPageToken(limit uint16) PageToken {
	return PageToken{Limit: limit, Page: 1}
}

func (tok PageToken) Next() PageToken {
	return PageToken{Limit: tok.Limit, Page: tok.Page + 1}
}

func (tok PageToken) Encode() string {
	buf := bytes.Buffer{}
	binary.Write(&buf, binary.LittleEndian, tok)
	return base64.URLEncoding.EncodeToString(buf.Bytes())
}

func decodePageToken(value string) (*PageToken, error) {
	dec, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	db := bytes.NewBuffer(dec)

	var tok PageToken
	err = binary.Read(db, binary.LittleEndian, &tok)
	if err != nil {
		return nil, err
	}

	return &tok, nil
}

func findNextPageToken(u *url.URL, limit uint16) (*PageToken, error) {
	values := u.Query()["nextPageToken"]

	if len(values) > 1 {
		return nil, errors.New("too many values for page token")
	}

	if len(values) == 0 {
		return nil, nil
	}

	val := values[0]
	tok, err := decodePageToken(val)
	if err != nil {
		return nil, err
	}

	err = validatePageToken(tok, limit)
	if err != nil {
		return nil, err
	}

	return tok, nil
}

func validatePageToken(tok *PageToken, limit uint16) error {
	if tok.Limit != limit {
		return fmt.Errorf("token limit must be %d", limit)
	}

	if tok.Page == 0 {
		return errors.New("token page must be greater than zero")
	}

	return nil
}
