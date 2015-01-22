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

package config

import (
	"testing"
)

func TestConfigMetadata(t *testing.T) {
	raw := "foo=bar, ping=pong"
	cfg := Config{RawMetadata: raw}
	metadata := cfg.Metadata()

	if len(metadata) != 2 {
		t.Errorf("Parsed %d keys, expected 1", len(metadata))
	}

	if metadata["foo"] != "bar" {
		t.Errorf("Incorrect value '%s' of key 'foo', expected 'bar'", metadata["foo"])
	}

	if metadata["ping"] != "pong" {
		t.Errorf("Incorrect value '%s' of key 'ping', expected 'pong'", metadata["ping"])
	}
}

func TestConfigMetadataNotSet(t *testing.T) {
	cfg := Config{}
	metadata := cfg.Metadata()

	if len(metadata) != 0 {
		t.Errorf("Parsed %d keys, expected 0", len(metadata))
	}
}
