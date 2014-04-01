package main

import (
	"fmt"
	"sort"
	"crypto/md5"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/job"
)

func newListUnitsCommand() cli.Command {
	return cli.Command{
		Name:  "list-units",
		Usage: "Enumerate units loaded in the cluster",
		Description: `Lists all units submitted or started on the cluster.

For easily parsable output, you can remove the column headers:
fleetctl list-units --no-legend

Output hashes of the units:
fleetctl list-units --hash

Output the list without ellipses:
fleetctl list-units --full`,
		Action: listUnitsAction,
		Flags: []cli.Flag{
			cli.BoolFlag{"full, l", "Do not ellipsize fields on output"},
			cli.BoolFlag{"hash", "Show the unit's hash"},
			cli.BoolFlag{"no-legend", "Do not print a legend (column headers)"},
		},
	}
}

func listUnitsAction(c *cli.Context) {
	showHash := c.Bool("hash")

	if !c.Bool("no-legend") {
		legend := "UNIT\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE"

		if showHash {
			legend += "\tHASH"
		}

		fmt.Fprintln(out, legend)
	}

	names, sortable := findAllUnits()

	full := c.Bool("full")
	for _, name := range sortable {
		state := registryCtl.GetJobState(name)
		description := names[name]
		printJobState(name, description, state, full, showHash)
	}

	out.Flush()
}

func findAllUnits() (names map[string]string, sortable sort.StringSlice) {
	names = make(map[string]string, 0)
	sortable = make(sort.StringSlice, 0)

	for _, p := range registryCtl.GetAllPayloads() {
		if _, ok := names[p.Name]; !ok {
			names[p.Name] = p.Unit.Description()
			sortable = append(sortable, p.Name)
		}
	}

	for _, j := range registryCtl.GetAllJobs() {
		if _, ok := names[j.Name]; !ok {
			var description string
			if j.Payload != nil {
				description = j.Payload.Unit.Description()
			}
			names[j.Name] = description
			sortable = append(sortable, j.Name)
		}
	}

	sortable.Sort()

	return names, sortable
}

func getUnitHash(unitName string, full bool) string {
	payloadHash := md5.New()
	payloadBytes := []byte(registryCtl.GetPayload(unitName).Unit.String())
	payloadHash.Write(payloadBytes)
	hash := fmt.Sprintf("%x", payloadHash.Sum(nil))
	if !full {
		hash = ellipsize(hash, ellipsizeMax)
	}
	return hash
}

func printJobState(name, description string, js *job.JobState, full bool, showHash bool) {
	loadState := "-"
	activeState := "-"
	subState := "-"
	mach := "-"

	if description == "" {
		description = "-"
	}

	if js != nil {
		loadState = js.LoadState
		activeState = js.ActiveState
		subState = js.SubState

		if js.MachineState != nil {
			mach = machineFullLegend(*js.MachineState, full)
		}
	}

	fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s", name, loadState, activeState, subState, description, mach)

	if showHash {
		fmt.Fprintf(out, "\t%s", getUnitHash(name, full))
	}

	fmt.Fprintf(out, "\n");
}
