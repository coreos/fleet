// Copyright 2014 CoreOS, Inc.
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
	"net/url"
	"reflect"
	"testing"
)

func TestDefaultPageToken(t *testing.T) {
	tok := DefaultPageToken(testTokenLimit)
	expect := PageToken{Limit: 100, Page: 1}
	if !reflect.DeepEqual(expect, tok) {
		t.Errorf("Unexpected default PageToken: expect=%v, got=%v", expect, tok)
	}
}

func TestPageTokenAdvance(t *testing.T) {
	tok := PageToken{Page: 2, Limit: 12}
	next := tok.Next()
	expect := PageToken{Page: 3, Limit: 12}
	if !reflect.DeepEqual(expect, next) {
		t.Errorf("Unexpected PageToken: expect=%v, got=%v", expect, next)
	}
}

func TestPageTokenEncode(t *testing.T) {
	tests := []struct {
		input  PageToken
		expect string
	}{
		{PageToken{Limit: 12482}, "wjAAAA=="},
		{PageToken{Limit: 19, Page: 11852}, "EwBMLg=="},
	}

	for i, tt := range tests {
		enc := tt.input.Encode()
		if enc != tt.expect {
			t.Errorf("case %d: expected %s, got %s", i, tt.expect, enc)
		}
	}
}

func TestPageTokenDecode(t *testing.T) {
	tests := []struct {
		input  string
		expect *PageToken
		pass   bool
	}{
		{"_wMAAA==", &PageToken{Limit: 1023}, true},
		{"LQAJAA==", &PageToken{Limit: 45, Page: 9}, true},

		// incorrectly base64-encoded data
		{"basdfasdf", nil, false},

		// empty string is valid base64, but fails binary decode
		{"", nil, false},
	}

	for i, tt := range tests {
		tok, err := decodePageToken(tt.input)
		if (err == nil) != tt.pass {
			t.Errorf("case %d: expected pass=%t, got err=%v", i, tt.pass, err)
			continue
		}

		if !reflect.DeepEqual(tok, tt.expect) {
			t.Errorf("case %d: expected %v, got %v", i, tt.expect, tok)
		}
	}
}

func TestFindNextPageToken(t *testing.T) {
	tests := []struct {
		input  url.URL
		expect *PageToken
		pass   bool
	}{
		{url.URL{RawQuery: "nextPageToken=ZABMLg=="}, &PageToken{Limit: 100, Page: 11852}, true},

		// Other query params should be ignored
		{url.URL{RawQuery: "filter=foobar&nextPageToken=ZABMLg=="}, &PageToken{Limit: 100, Page: 11852}, true},

		// Lack of a nextPageToken value is ok
		{url.URL{RawQuery: "filter=foobar"}, nil, true},

		// More than one value for nextPageToken is not acceptable
		{url.URL{RawQuery: "nextPageToken=LQAJAA==&nextPageToken=_wMAAA=="}, nil, false},

		// Unparseable values result in an error
		{url.URL{RawQuery: "nextPageToken=bogus"}, nil, false},

		// Ensure validation code is being called
		{url.URL{RawQuery: "nextPageToken="}, nil, false},
	}

	for i, tt := range tests {
		next, err := findNextPageToken(&tt.input, testTokenLimit)

		if tt.pass != (err == nil) {
			t.Errorf("case %d: pass=%t, err=%v", i, tt.pass, err)
		}

		if !reflect.DeepEqual(next, tt.expect) {
			t.Errorf("case %d: expected %v, got %v", i, tt.expect, next)
		}
	}
}

func TestValidatePageToken(t *testing.T) {
	tests := []struct {
		input PageToken
		pass  bool
	}{
		{PageToken{Limit: 100, Page: 9}, true},

		// Limit must be 100
		{PageToken{Limit: 20, Page: 9}, false},

		// Page must be nonzero
		{PageToken{Limit: 100, Page: 0}, false},
	}

	for i, tt := range tests {
		err := validatePageToken(&tt.input, testTokenLimit)

		if tt.pass != (err == nil) {
			t.Errorf("case %d: pass=%t, err=%v", i, tt.pass, err)
		}
	}
}
