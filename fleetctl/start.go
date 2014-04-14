package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/job"
)

const (
	unitCheckInterval = 1 * time.Second
)

var (
	flagRequire       string
	flagBlockAttempts int
	flagNoBlock       bool

	cmdStartUnit = &Command{
		Name:    "start",
		Summary: "Schedule and execute one or more units in the cluster",
		Description: `Start one or many units on the cluster. Select units to start by glob matching
for units in the current working directory or matching names of previously
submitted units.

Start a single unit:
fleetctl start foo.service

Start an entire directory of units with glob matching:
fleetctl start myservice/*

You may filter suitable hosts based on metadata provided by the machine.
Machine metadata is located in the fleet configuration file.`,
		Run: runStartUnit,
	}
)

func init() {
	cmdStartUnit.Flags.BoolVar(&sharedFlags.Sign, "sign", false, "Sign unit file signatures using local SSH identities")
	cmdStartUnit.Flags.IntVar(&flagBlockAttempts, "block-attempts", 10, "Wait until the jobs are scheduled. Perform N attempts before giving up.")
	cmdStartUnit.Flags.BoolVar(&flagNoBlock, "no-block", false, "Do not wait until the units have been scheduled to exit start.")
}

func runStartUnit(args []string) (exit int) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "No units specified.")
		return 1
	}
	jobs, err := findOrCreateJobs(args, sharedFlags.sign)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed creating jobs: %v", err)
		return 1
	}

	// TODO: This must be done in a transaction!
	registeredJobs := make(map[string]bool)
	for _, j := range jobs {
		//TODO: Replace this with the actual method of starting once
		// it is no longer covered by `fleetctl submit`
		//registryCtl.StartJob(j.Name)
		registeredJobs[j.Name] = true
	}

	if !flagNoBlock {
		waitForScheduledUnits(registeredJobs, flagBlockAttempts, os.Stdout)
	}
	return 0
}

func waitForScheduledUnits(jobs map[string]bool, maxAttempts int, out io.Writer) {
	var wg sync.WaitGroup

	for jobName, _ := range jobs {
		wg.Add(1)
		go checkJobTarget(jobName, maxAttempts, out, &wg)
	}

	wg.Wait()
}

func checkJobTarget(jobName string, maxAttempts int, out io.Writer, wg *sync.WaitGroup) {
	defer wg.Done()

	for attempts := 0; attempts < maxAttempts; attempts++ {
		tgt := registryCtl.GetJobTarget(jobName)

		if tgt != "" {
			m := registryCtl.GetMachineState(tgt)
			fmt.Fprintf(out, "Job %s scheduled to %s\n", jobName, machineFullLegend(*m, false))
			return
		}

		time.Sleep(unitCheckInterval)
	}
	fmt.Fprintf(out, "Job %s still queued for scheduling\n", jobName)
}
