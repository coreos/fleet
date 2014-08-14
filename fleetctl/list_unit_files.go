package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
)

const (
	defaultListUnitFilesFields = "unit,hash,dstate,state,tmachine"
)

var (
	listUnitFilesFieldsFlag string
	cmdListUnitFiles        = &Command{
		Name:        "list-unit-files",
		Summary:     "List the units that exist in the cluster.",
		Usage:       "[--fields]",
		Description: `Lists all unit files that exist in the cluster (whether or not they are loaded onto a machine).`,
		Run:         runListUnitFiles,
	}
	listUnitFilesFields = map[string]unitToField{
		"unit": func(u schema.Unit, full bool) string {
			return u.Name
		},
		"dstate": func(u schema.Unit, full bool) string {
			if u.DesiredState == "" {
				return "-"
			}
			return u.DesiredState
		},
		"tmachine": func(u schema.Unit, full bool) string {
			if u.Machine == "" {
				return "-"
			}
			ms := cachedMachineState(u.Machine)
			if ms == nil {
				ms = &machine.MachineState{ID: u.Machine}
			}

			return machineFullLegend(*ms, full)
		},
		"state": func(u schema.Unit, full bool) string {
			if u.CurrentState == "" {
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

func init() {
	cmdListUnitFiles.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnitFiles.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListUnitFiles.Flags.StringVar(&listUnitFilesFieldsFlag, "fields", defaultListUnitFilesFields, fmt.Sprintf("Columns to print for each Unit file. Valid fields are %q", strings.Join(unitToFieldKeys(listUnitFilesFields), ",")))
}

func runListUnitFiles(args []string) (exit int) {
	if listUnitFilesFieldsFlag == "" {
		fmt.Fprintf(os.Stderr, "Must define output format\n")
		return 1
	}

	cols := strings.Split(listUnitFilesFieldsFlag, ",")
	for _, s := range cols {
		if _, ok := listUnitFilesFields[s]; !ok {
			fmt.Fprintf(os.Stderr, "Invalid key in output format: %q\n", s)
			return 1
		}
	}

	units, err := cAPI.Units()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving list of units from repository: %v\n", err)
		return 1
	}

	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, u := range units {
		var f []string
		for _, c := range cols {
			f = append(f, listUnitFilesFields[c](*u, sharedFlags.Full))
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
