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

var cmdSubmit = &cobra.Command{
	Use:   "submit UNIT...",
	Short: "Upload one or more units to the cluster without starting them",
	Long: `Upload one or more units to the cluster without starting them. Useful
for validating units before they are started.

This operation is idempotent; if a named unit already exists in the cluster, it will not be resubmitted.

Submit a single unit:
fleetctl submit foo.service

Submit a directory of units with glob matching:
fleetctl submit myservice/*`,
	Run: runWrapper(runSubmitUnit),
}

func init() {
	cmdFleet.AddCommand(cmdSubmit)

	cmdSubmit.Flags().BoolVar(&sharedFlags.Sign, "sign", false, "DEPRECATED - this option cannot be used")
	cmdSubmit.Flags().BoolVar(&sharedFlags.Replace, "replace", false, "Replace the old submitted units in the cluster with new versions.")
}

func runSubmitUnit(cCmd *cobra.Command, args []string) (exit int) {
	if len(args) == 0 {
		stderr("No units given")
		return 0
	}

	if err := lazyCreateUnits(cCmd, args); err != nil {
		stderr("Error creating units: %v", err)
		return 1
	}
	return 0
}
