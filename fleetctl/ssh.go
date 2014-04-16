package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/gosshnew/ssh"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/ssh"
)

var (
	flagMachine            string
	flagUnit               string
	flagSSHAgentForwarding bool
	cmdSSH                 = &Command{
		Name:    "ssh",
		Summary: "Open interactive shell on a machine in the cluster",
		Usage:   "[--forward-agent] [--machine|--unit] {MACHINE|UNIT}",
		Description: `Open an interactive shell on a specific machine in the cluster or on the machine 
where the specified unit is located.

fleetctl tries to detect whether your first argument is a machine or a unit. 
To skip this check use the --machine or --unit flags.

Open a shell on a machine:
	fleetctl ssh 2444264c-eac2-4eff-a490-32d5e5e4af24

Open a shell from your laptop, to the machine running a specific unit, using a
cluster member as a bastion host:
	fleetctl --tunnel 10.10.10.10 ssh foo.service

Open a shell on a machine and forward the authentication agent connection:
	fleetctl ssh --forward-agent 2444264c-eac2-4eff-a490-32d5e5e4af24


Tip: Create an alias for --tunnel.
	- Add "alias fleetctl=fleetctl --tunnel 10.10.10.10" to your bash profile.
	- Now you can run all fleet commands locally.`,
		Run: runSSH,
	}
)

func init() {
	cmdSSH.Flags.StringVar(&flagMachine, "machine", "", "Open SSH connection to a specific machine.")
	cmdSSH.Flags.StringVar(&flagUnit, "unit", "", "Open SSH connection to machine running provided unit.")
	cmdSSH.Flags.BoolVar(&flagSSHAgentForwarding, "forward-agent", false, "Forward local ssh-agent to target machine.")
}

func runSSH(args []string) (exit int) {
	if flagUnit != "" && flagMachine != "" {
		fmt.Fprintln(os.Stderr, "Both machine and unit flags provided, please specify only one.")
		return 1
	}

	var err error
	var addr string

	switch {
	case flagMachine != "":
		addr, _ = findAddressInMachineList(flagMachine)
	case flagUnit != "":
		addr, _ = findAddressInRunningUnits(flagUnit)
	default:
		addr, err = globalMachineLookup(args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		// trim machine/unit name from args
		if len(args) > 0 {
			args = args[1:]
		}
	}

	if addr == "" {
		fmt.Fprintln(os.Stderr, "Requested machine could not be found.")
		return 1
	}

	var sshClient *ssh.SSHForwardingClient
	if tun := getTunnelFlag(); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr, getChecker(), flagSSHAgentForwarding)
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr, getChecker(), flagSSHAgentForwarding)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed building SSH client: %v\n", err)
		return 1
	}

	defer sshClient.Close()

	if len(args) > 0 {
		cmd := strings.Join(args, " ")
		channel, err := ssh.Execute(sshClient, cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed running command over SSH: %v\n", err)
			return 1
		}

		os.Exit(readSSHChannel(channel))
	} else {
		if err := ssh.Shell(sshClient); err != nil {
			fmt.Fprintf(os.Stderr, "Failed opening shell over SSH: %v\n", err)
			return 1
		}
	}
	return 0
}

func globalMachineLookup(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("Provide one machine or unit.")
	}

	lookup := args[0]

	machineAddr, machineOk := findAddressInMachineList(lookup)
	unitAddr, unitOk := findAddressInRunningUnits(lookup)

	switch {
	case machineOk && unitOk:
		return "", fmt.Errorf("Ambiguous argument, both machine and unit found for `%s`.\nPlease use flag `-m` or `-u` to refine the search.", lookup)
	case machineOk:
		return machineAddr, nil
	case unitOk:
		return unitAddr, nil
	}

	return "", nil
}

func findAddressInMachineList(lookup string) (string, bool) {
	states := registryCtl.GetActiveMachines()
	var match *machine.MachineState

	for i, _ := range states {
		machState := states[i]
		if !strings.HasPrefix(machState.BootID, lookup) {
			continue
		} else if match != nil {
			fmt.Fprintln(os.Stderr, "Found more than one Machine, be more specific.")
			os.Exit(1)
		}
		match = &machState
	}

	if match == nil {
		return "", false
	}

	return fmt.Sprintf("%s:22", match.PublicIP), true
}

func findAddressInRunningUnits(lookup string) (string, bool) {
	j := registryCtl.GetJob(lookup)
	if j == nil || j.PayloadState == nil {
		return "", false
	}
	return fmt.Sprintf("%s:22", j.PayloadState.MachineState.PublicIP), true
}

// Read stdout from SSH channel and print to local stdout.
// If remote command fails, also read stderr and print to local stderr.
// Returns exit status from remote command.
func readSSHChannel(channel *ssh.Channel) int {
	readSSHChannelOutput(channel.Stdout, os.Stdout)

	exitErr := <-channel.Exit
	if exitErr == nil {
		return 0
	}

	readSSHChannelOutput(channel.Stderr, os.Stderr)

	exitStatus := -1
	switch exitError := exitErr.(type) {
	case *gossh.ExitError:
		exitStatus = exitError.ExitStatus()
	case *exec.ExitError:
		status := exitError.Sys().(syscall.WaitStatus)
		exitStatus = status.ExitStatus()
	}

	fmt.Fprintf(os.Stderr, "Failed reading SSH channel: %v\n", exitErr)
	return exitStatus
}

// Read bytes from a bufio.Reader and write as a string to out
func readSSHChannelOutput(in *bufio.Reader, out io.Writer) {
	for {
		bytes, err := in.ReadBytes('\n')
		if err != nil {
			break
		}

		fmt.Fprint(out, string(bytes))
	}
}

// runCommand will attempt to run a command on a given machine. It will attempt
// to SSH to the machine if it is identified as being remote.
func runCommand(cmd string, ms *machine.MachineState) (retcode int, err error) {
	if machine.IsLocalMachineState(ms) {
		retcode = runLocalCommand(cmd)
	} else {
		retcode, err = runRemoteCommand(cmd, ms.PublicIP)
	}
	return
}

func runLocalCommand(cmd string) int {
	cmdSlice := strings.Split(cmd, " ")
	osCmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	stdout, _ := osCmd.StdoutPipe()
	stderr, _ := osCmd.StderrPipe()

	channel := &ssh.Channel{
		bufio.NewReader(stdout),
		bufio.NewReader(stderr),
		make(chan error),
	}

	osCmd.Start()
	go func() {
		err := osCmd.Wait()
		channel.Exit <- err
	}()

	return readSSHChannel(channel)
}

func runRemoteCommand(cmd string, ip string) (int, error) {
	addr := fmt.Sprintf("%s:22", ip)

	var sshClient *ssh.SSHForwardingClient
	var err error
	if tun := getTunnelFlag(); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr, getChecker(), false)
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr, getChecker(), false)
	}
	if err != nil {
		return 1, err
	}

	defer sshClient.Close()

	channel, err := ssh.Execute(sshClient, cmd)
	if err != nil {
		return 1, err
	}

	return readSSHChannel(channel), nil
}
