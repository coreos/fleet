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

package debug

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
)

var Enabled bool

func init() {
	Enabled = true
}

func Start(addr string) {
	go http.ListenAndServe(addr, nil)
}

func RegisterHTTPHandler(pattern string, handler http.Handler) {
	if Enabled {
		http.Handle(pattern, handler)
	}
}

func genRandomID() string {
	c := 5
	b := make([]byte, c)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func Enter_(args ...interface{}) string {
	pc, _, _, _ := runtime.Caller(1)
	functionObject := runtime.FuncForPC(pc)
	extractFnName := regexp.MustCompile(`^.*\/(.*)$`)
	fnName := extractFnName.ReplaceAllString(functionObject.Name(), "$1")
	argsStr := make([]string, len(args))
	for idx, arg := range args {
		argsStr[idx] = fmt.Sprintf("%+v", arg)
	}

	fnWithId := genRandomID() + " " + fnName

	fmt.Printf("==> %s(%s)\n", fnWithId, strings.Join(argsStr, ","))
	return fnWithId
}

func Exit_(fnName string) {
	fmt.Printf("<== %s\n", fnName)
}
