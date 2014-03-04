package main

import (
	"fmt"
	"log"
	"syscall"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh"
	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/ssh"
)

func newJournalCommand() cli.Command {
	return cli.Command{
		Name:	"journal",
		Usage:	"Print the journal of a unit in the cluster to stdout",
		Action:	journalAction,
		Description: `Outputs the journal of a unit by connecting to the machine that the unit
occupies.

Read the last 10 lines:
fleetctl journal foo.service

Read the last 100 lines:
fleetctl journal --lines 100 foo.service`,
		Flags: []cli.Flag{
			cli.IntFlag{"lines, n", 10, "Number of recent log lines to return."},
			cli.BoolFlag{"follow, f", "Continuously print new entries as they are appended to the journal."},
		},
	}
}

func journalAction(c *cli.Context) {
	if len(c.Args()) != 1 {
		fmt.Println("One unit file must be provided.")
		syscall.Exit(1)
	}
	jobName := c.Args()[0]

	r := getRegistry()
	js := r.GetJobState(jobName)

	if js == nil {
		fmt.Printf("%s does not appear to be running\n", jobName)
		syscall.Exit(1)
	}

	addr := fmt.Sprintf("%s:22", js.MachineState.PublicIP)

	var err error
	var sshClient *gossh.ClientConn
	if tun := getTunnelFlag(); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr)
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr)
	}
	if err != nil {
		log.Fatal(err.Error())
	}

	defer sshClient.Close()

	cmd := fmt.Sprintf("journalctl -u %s --no-pager -l -n %d", jobName, c.Int("lines"))
	if c.Bool("follow") {
		cmd += " -f"
	}
	stdout, err := ssh.Execute(sshClient, cmd)
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
