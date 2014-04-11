package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/job"
)

const (
	unitCheckInterval = 1 * time.Second
)

func newStartUnitCommand() cli.Command {
	return cli.Command{
		Name:  "start",
		Usage: "Schedule and execute one or more units in the cluster",
		Description: `Start one or many units on the cluster. Select units to start by glob matching
for units in the current working directory or matching names of previously
submitted units.

Start a single unit:
fleetctl start foo.service

Start an entire directory of units with glob matching:
fleetctl start myservice/*

You may filter suitable hosts based on metadata provided by the machine.
Machine metadata is located in the fleet configuration file.`,
		Flags: []cli.Flag{
			cli.BoolFlag{"sign", "Sign unit file signatures using local SSH identities"},
			cli.IntFlag{"block-attempts", 10, "Wait until the jobs are scheduled. Perform N attempts before giving up, 10 by default."},
			cli.BoolFlag{"no-block", "Do not wait until the units have been scheduled to exit start."},
		},
		Action: startUnitAction,
	}
}

func startUnitAction(c *cli.Context) {
	// Attempt to create payloads for convenience
	payloads, err := submitPayloads(c.Args(), c.Bool("sign"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed creating payloads: %v", err)
		os.Exit(1)
	}

	// TODO: This must be done in a transaction!
	registeredJobs := make(map[string]bool)
	for _, jp := range payloads {
		j := job.NewJob(jp.Name, jp)
		log.V(1).Infof("Created new Job(%s) from Payload(%s)", j.Name, jp.Name)
		err := registryCtl.CreateJob(j)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed creating job %s: %v\n", j.Name, err)
			os.Exit(1)
		}
		registeredJobs[j.Name] = true
	}

	if !c.Bool("no-block") {
		waitForScheduledUnits(registeredJobs, c.Int("block-attempts"), os.Stdout)
	}
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
