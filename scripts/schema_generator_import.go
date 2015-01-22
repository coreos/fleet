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

// +build ignore

package scripts

// This file exists to ensure Godep manages a vendored copy of the
// `google-api-go-generator` library, used by scripts/schema-generator.
// Unfortunately since this is a binary package and hence is not importable, we
// need to trick godep into managing it. To update the dependency, do the following steps:
// 1. Use `godep restore` to set up GOPATH with all the right package versions
// 2. Uncomment the import line below
// 3. Update the package in GOPATH as appropriate (e.g. `go get -u google.golang.org/api/google-api-go-generator`)
// 4. Run `godep save` as usual across the entire project (e.g. `godep save -r ./...`)
// 5. Revert this file (i.e. comment the line again, and revert to the original import) as it will not build properly
//
// import _ "google.golang.org/api/google-api-go-generator"
