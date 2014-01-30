package main

import (
	"fmt"
	"log"
	"syscall"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/ssh"
)

func newJournalCommand() cli.Command {
	return cli.Command{
		Name:        "journal",
		Usage:       "Print the journal of a unit in the cluster to stdout",
		Action:      journalAction,
		Flags: []cli.Flag{
			cli.IntFlag{"lines, n", 10, "Number of log lines to return."},
		},
	}
}

func journalAction(c *cli.Context) {
	if len(c.Args()) != 1 {
		fmt.Println("One unit file must be provided.")
		syscall.Exit(1)
	}
	jobName := c.Args()[0]

	r := getRegistry(c)
	js := r.GetJobState(jobName)

	if js == nil {
		fmt.Println("Unit does not appear to be running")
		syscall.Exit(1)
	}

	user := "core"
	addr := fmt.Sprintf("%s:22", js.Machine.PublicIP)
	cmd := fmt.Sprintf("journalctl -u %s --no-pager -l -n %d", jobName, c.Int("lines"))
	stdout, err := ssh.Execute(user, addr, cmd)
	if err != nil {
		log.Fatalf("Unable to run command over SSH: %s", err.Error())
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
