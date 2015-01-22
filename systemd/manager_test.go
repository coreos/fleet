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

package systemd

import (
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"
)

func TestHashUnitFile(t *testing.T) {
	f, err := ioutil.TempFile("", "fleet-testing-")
	if err != nil {
		t.Fatalf(err.Error())
	}

	defer os.Remove(f.Name())

	contents := `
[Service]
ExecStart=/usr/bin/sleep infinity
`

	if _, err := f.Write([]byte(contents)); err != nil {
		t.Fatalf(err.Error())
	}

	if err := f.Close(); err != nil {
		t.Fatalf(err.Error())
	}

	hash, err := hashUnitFile(f.Name())
	if err != nil {
		t.Fatalf(err.Error())
	}

	want := "40ea6646945809f4b420a50475ee68503088f127"
	got := hash.String()
	if want != got {
		t.Fatalf("unit hash incorrect: want=%s, got=%s", want, got)
	}
}

func TestHashUnitFileDirectory(t *testing.T) {
	dir, err := ioutil.TempDir("", "fleet-testing-")
	if err != nil {
		t.Fatal(err.Error())
	}

	defer os.RemoveAll(dir)

	fixtures := []struct {
		name     string
		contents string
		hash     string
	}{
		{
			name:     "foo.service",
			contents: "[Service]\nExecStart=/usr/bin/sleep infinity",
			hash:     "40ea6646945809f4b420a50475ee68503088f127",
		},
		{
			name:     "bar.service",
			contents: "[Service]\nExecStart=/usr/bin/sleep 10",
			hash:     "5bf16b98c62f35fcdc723d32989cdeba7a2dd2a8",
		},
		{
			name:     "baz.service",
			contents: "[Service]\nExecStart=/usr/bin/sleep 2000",
			hash:     "5ba5292ab6a82b623ee6086dc90b3354ba004832",
		},
	}

	for _, f := range fixtures {
		err := ioutil.WriteFile(path.Join(dir, f.name), []byte(f.contents), 0400)
		if err != nil {
			t.Fatal(err.Error())
		}
	}

	hashes, err := hashUnitFiles(dir)
	if err != nil {
		t.Fatal(err.Error())
	}

	got := make(map[string]string, len(hashes))
	for uName, hash := range hashes {
		got[uName] = hash.String()
	}

	want := make(map[string]string, len(fixtures))
	for _, f := range fixtures {
		want[f.name] = f.hash
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("hashUnitFileDirectory returned unexpected values: want=%v, got=%v", want, got)
	}
}
