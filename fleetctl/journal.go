package main

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"syscall"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/ssh"
)

func newJournalCommand() cli.Command {
	return cli.Command{
		Name:   "journal",
		Usage:  "Print the journal of a unit in the cluster to stdout",
		Action: journalAction,
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

	js := registryCtl.GetJobState(jobName)

	if js == nil {
		fmt.Printf("%s does not appear to be running\n", jobName)
		syscall.Exit(1)
	}

	cmd := fmt.Sprintf("journalctl -u %s --no-pager -l -n %d", jobName, c.Int("lines"))
	if c.Bool("follow") {
		cmd += " -f"
	}

	// check if the job is running on this machine
	var out *bufio.Reader
	var err error
	if machine.IsLocalMachineState(js.MachineState) {
		out = runLocalCommand(cmd)
	} else {
		out, err = runRemoteCommand(cmd, js.MachineState.PublicIP)
		if err != nil {
			log.Fatalf("Unable to run command over SSH: %v", err)
		}
	}

	readAllLines(out)
}

func runLocalCommand(cmd string) *bufio.Reader {
	cmdSlice := strings.Split(cmd, " ")
	osCmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	stdout, _ := osCmd.StdoutPipe()
	bstdout := bufio.NewReader(stdout)

	osCmd.Start()
	go osCmd.Wait()

	return bstdout
}

func runRemoteCommand(cmd string, ip string) (*bufio.Reader, error) {
	addr := fmt.Sprintf("%s:22", ip)

	var sshClient *ssh.SSHForwardingClient
	var err error
	if tun := getTunnelFlag(); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr, getChecker(), false)
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr, getChecker(), false)
	}
	if err != nil {
		return nil, err
	}

	defer sshClient.Close()

	stdout, err := ssh.Execute(sshClient, cmd)
	if err != nil {
		return nil, err
	}
	return stdout, nil
}

func readAllLines(r *bufio.Reader) {
	for true {
		bytes, prefix, err := r.ReadLine()
		if err != nil {
			break
		}

		fmt.Print(string(bytes))
		if !prefix {
			fmt.Print("\n")
		}
	}
}
