// +build ignore

package scripts

// This file exists to ensure Godep manages a vendored copy of the
// `google-api-go-generator` library, used by scripts/schema-generator.
// Unfortunately since this is a binary package and hence is not importable, we
// need to trick godep into managing it. To update the dependency, do the following steps:
// 1. Use `godep restore` to set up GOPATH with all the right package versions
// 2. Uncomment the import line below
// 3. Update the package in GOPATH as appropriate (e.g. `go get -u code.google.com/p/google-api-go-client/google-api-go-generator`)
// 4. Run `godep save` as usual across the entire project (e.g. `godep save -r ./...`)
// 5. Revert this file (i.e. comment the line again, and revert to the original import) as it will not build properly
//
// import _ "code.google.com/p/google-api-go-client/google-api-go-generator"
