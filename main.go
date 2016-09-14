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

// Package main is a simple wrapper of the real fleet entrypoint package
// (located at github.com/coreos/fleet/fleetd) to ensure that fleet is still
// "go getable"; e.g. `go get github.com/coreos/fleet` works as expected and
// builds a binary in $GOBIN/fleetd
//
// This package should NOT be extended or modified in any way; to modify the
// fleetd binary, work in the `github.com/coreos/fleet/fleetd` package.
//
package main

import "github.com/coreos/fleet/fleetd"

func main() {
	fleetd.Main()
}
