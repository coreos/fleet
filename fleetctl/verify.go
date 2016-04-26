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

package main

import (
	"github.com/codegangsta/cli"

	"github.com/coreos/fleet/client"
)

func NewVerifyCommand() cli.Command {
	return cli.Command{
		Name:      "verify",
		Usage:     "DEPRECATED - No longer works",
		ArgsUsage: "UNIT",
		Action:    makeActionWrapper(runVerifyUnit),
	}
}

func runVerifyUnit(c *cli.Context, cAPI client.API) (exit int) {
	stderr("WARNING: The signed/verified units feature is DEPRECATED and cannot be used.")
	return 2
}
