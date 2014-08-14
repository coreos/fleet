package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
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
	listUnitFilesFields = map[string]jobUnitToField{
		"unit": func(u job.Unit, su *job.ScheduledUnit, full bool) string {
			return u.Name
		},
		"dstate": func(u job.Unit, su *job.ScheduledUnit, full bool) string {
			if u.TargetState == "" {
				return "-"
			}
			return string(u.TargetState)
		},
		"tmachine": func(u job.Unit, su *job.ScheduledUnit, full bool) string {
			if su == nil || su.TargetMachineID == "" {
				return "-"
			}
			ms := cachedMachineState(su.TargetMachineID)
			if ms == nil {
				ms = &machine.MachineState{ID: su.TargetMachineID}
			}

			return machineFullLegend(*ms, full)
		},
		"state": func(u job.Unit, su *job.ScheduledUnit, full bool) string {
			if su == nil || su.State == nil {
				return "-"
			}
			return string(*su.State)
		},
		"hash": func(u job.Unit, su *job.ScheduledUnit, full bool) string {
			if !full {
				return u.Unit.Hash().Short()
			}
			return u.Unit.Hash().String()
		},
		"desc": func(u job.Unit, su *job.ScheduledUnit, full bool) string {
			d := u.Unit.Description()
			if d == "" {
				return "-"
			}
			return d
		},
	}
)

type jobUnitToField func(j job.Unit, su *job.ScheduledUnit, full bool) string

func init() {
	cmdListUnitFiles.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnitFiles.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListUnitFiles.Flags.StringVar(&listUnitFilesFieldsFlag, "fields", defaultListUnitFilesFields, fmt.Sprintf("Columns to print for each Unit file. Valid fields are %q", strings.Join(jobUnitToFieldKeys(listUnitFilesFields), ",")))
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

	sched, err := cAPI.Schedule()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving unit schedule from repository: %v\n", err)
		return 1
	}

	sMap := make(map[string]*job.ScheduledUnit)
	for _, s := range sched {
		s := s
		sMap[s.Name] = &s
	}

	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, u := range units {
		var f []string
		for _, c := range cols {
			f = append(f, listUnitFilesFields[c](u, sMap[u.Name], sharedFlags.Full))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()
	return
}

func jobUnitToFieldKeys(m map[string]jobUnitToField) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return
}
