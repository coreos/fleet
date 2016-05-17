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
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/codegangsta/cli"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
)

const (
	defaultListUnitFilesFields = "unit,hash,dstate,state,target"
)

func mapTargetField(u schema.Unit, full bool) string {
	if suToGlobal(u) {
		return "global"
	}
	if u.MachineID == "" {
		return "-"
	}
	ms := cachedMachineState(u.MachineID)
	if ms == nil {
		ms = &machine.MachineState{ID: u.MachineID}
	}

	return machineFullLegend(*ms, full)
}

func NewListUnitFilesCommand() cli.Command {
	return cli.Command{
		Name:        "list-unit-files",
		Usage:       "List the units that exist in the cluster.",
		ArgsUsage:   "[--fields]",
		Description: "Lists all unit files that exist in the cluster (whether or not they are loaded onto a machine)",
		Action:      makeActionWrapper(runListUnitFiles),
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "full", Usage: "Do not ellipsize fields on output"},
			cli.BoolFlag{Name: "no-legend", Usage: "Do not print a legend (column headers)"},
			cli.StringFlag{Name: "fields", Value: defaultListUnitFilesFields, Usage: fmt.Sprintf("Columns to print for each Unit file. Valid fields are %q", strings.Join(unitToFieldKeys(listUnitFilesFields), ","))},
		},
	}
}

var (
	listUnitFilesFields = map[string]unitToField{
		"unit": func(u schema.Unit, full bool) string {
			return u.Name
		},
		"global": func(u schema.Unit, full bool) string {
			return strconv.FormatBool(suToGlobal(u))
		},
		"dstate": func(u schema.Unit, full bool) string {
			if u.DesiredState == "" {
				return "-"
			}
			return u.DesiredState
		},
		"target":   mapTargetField,
		"tmachine": mapTargetField,
		"state": func(u schema.Unit, full bool) string {
			if suToGlobal(u) || u.CurrentState == "" {
				return "-"
			}
			return u.CurrentState
		},
		"hash": func(u schema.Unit, full bool) string {
			uf := schema.MapSchemaUnitOptionsToUnitFile(u.Options)
			if !full {
				return uf.Hash().Short()
			}
			return uf.Hash().String()
		},
		"desc": func(u schema.Unit, full bool) string {
			uf := schema.MapSchemaUnitOptionsToUnitFile(u.Options)
			d := uf.Description()
			if d == "" {
				return "-"
			}
			return d
		},
	}
)

type unitToField func(u schema.Unit, full bool) string

func runListUnitFiles(c *cli.Context, cAPI client.API) (exit int) {
	listUnitFilesFieldsFlag := c.String("fields")
	if listUnitFilesFieldsFlag == "" {
		stderr("Must define output format")
		return 1
	}

	cols := strings.Split(listUnitFilesFieldsFlag, ",")
	for _, s := range cols {
		if _, ok := listUnitFilesFields[s]; !ok {
			stderr("Invalid key in output format: %q", s)
			return 1
		}
		if s == "tmachine" {
			stderr("WARNING: The \"tmachine\" field is deprecated. Use \"target\" instead")
		}
	}

	units, err := cAPI.Units()
	if err != nil {
		stderr("Error retrieving list of units from repository: %v", err)
		return 1
	}

	if !c.Bool("no-legend") {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, u := range units {
		var f []string
		for _, col := range cols {
			f = append(f, listUnitFilesFields[col](*u, c.Bool("full")))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()

	return
}

func unitToFieldKeys(m map[string]unitToField) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return
}
