package main

import (
	"fmt"
	"log"
	"path"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh"
	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/ssh"
)

func newStatusUnitsCommand() cli.Command {
	return cli.Command{
		Name:	"status",
		Usage:	"Output the status of one or more units in the cluster",
		Description: `Output the status of one or more units currently running in the cluster.
Supports glob matching of units in the current working directory or matches
previously started units.

Show status of a single unit:
fleetctl status foo.service

Show status of an entire directory with glob matching:
fleetctl status myservice/*`,
		Action:	statusUnitsAction,
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
		printUnitStatus(c, r, name)
	}
}

func printUnitStatus(c *cli.Context, r *registry.Registry, jobName string) {
	js := r.GetJobState(jobName)

	if js == nil {
		fmt.Println("%s does not appear to be running", jobName)
	}

	addr := fmt.Sprintf("%s:22", js.MachineState.PublicIP)

	var err error
	var sshClient *gossh.ClientConn
	if tun := getTunnelFlag(c); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr)
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr)
	}
	if err != nil {
		log.Fatalf("Unable to establish SSH connection: %v", err)
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

		print(string(bytes))
		if !prefix {
			print("\n")
		}
	}
}
