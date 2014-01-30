package main

import (
	"fmt"
	"log"
	"path"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/registry"
	"github.com/coreos/coreinit/ssh"
)

func newStatusUnitsCommand() cli.Command {
	return cli.Command{
		Name:        "status",
		Usage:       "Fetch the status of one or more units in the cluster",
		Action:      statusUnitsAction,
	}
}

func statusUnitsAction(c *cli.Context) {
	r := getRegistry(c)

	for i, v := range c.Args() {
		// This extra newline here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		name := path.Base(v)
		printUnitStatus(r, name)
	}
}

func printUnitStatus(r *registry.Registry, jobName string) {
	js := r.GetJobState(jobName)

	if js == nil {
		fmt.Println("%s does not appear to be running", jobName)
	}

	user := "core"
	addr := fmt.Sprintf("%s:22", js.Machine.PublicIP)
	cmd := fmt.Sprintf("systemctl status -l %s", jobName)
	stdout, err := ssh.Execute(user, addr, cmd)
	if err != nil {
		log.Fatalf("Unable to execute command over SSH: %s", err.Error())
	}

	for true {
		bytes, prefix, err := stdout.ReadLine()
		if err != nil {
			break
		}

		print(string(bytes))
		if !prefix {
			print("\n")
		}
	}
}
