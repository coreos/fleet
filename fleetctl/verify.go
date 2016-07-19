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

package main

import (
	"github.com/spf13/cobra"
)

var cmdVerifyUnit = &cobra.Command{
	Use:        "verify UNIT",
	Deprecated: "DEPRECATED - No longer works",
	Run:        runWrapper(runVerifyUnit),
}

func init() {
	cmdFleet.AddCommand(cmdVerifyUnit)
}

func runVerifyUnit(cCmd *cobra.Command, args []string) (exit int) {
	stderr("WARNING: The signed/verified units feature is DEPRECATED and cannot be used.")
	return 2
}
