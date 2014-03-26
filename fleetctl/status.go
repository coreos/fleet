package main

import (
	"fmt"
	"log"
	"path"
	"syscall"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/ssh"
)

func newStatusUnitsCommand() cli.Command {
	return cli.Command{
		Name:  "status",
		Usage: "Output the status of one or more units in the cluster",
		Description: `Output the status of one or more units currently running in the cluster.
Supports glob matching of units in the current working directory or matches
previously started units.

Show status of a single unit:
fleetctl status foo.service

Show status of an entire directory with glob matching:
fleetctl status myservice/*`,
		Action: statusUnitsAction,
	}
}

func statusUnitsAction(c *cli.Context) {
	for i, v := range c.Args() {
		// This extra newline here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		name := path.Base(v)
		printUnitStatus(c, name)
	}
}

func printUnitStatus(c *cli.Context, jobName string) {
	js := registryCtl.GetJobState(jobName)

	if js == nil {
		fmt.Printf("%s does not appear to be running\n", jobName)
		syscall.Exit(1)
	}

	addr := fmt.Sprintf("%s:22", js.MachineState.PublicIP)

	var err error
	var sshClient *ssh.SSHForwardingClient

	if tun := getTunnelFlag(); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr, getChecker(), false)
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr, getChecker(), false)
	}
	if err != nil {
		log.Fatal(err.Error())
	}

	defer sshClient.Close()

	cmd := fmt.Sprintf("systemctl status -l %s", jobName)
	stdout, err := ssh.Execute(sshClient, cmd)
	if err != nil {
		log.Fatalf("Unable to execute command over SSH: %s", err.Error())
	}

	for true {
		bytes, prefix, err := stdout.ReadLine()
		if err != nil {
			break
		}

		fmt.Print(string(bytes))
		if !prefix {
			fmt.Print("\n")
		}
	}
}
