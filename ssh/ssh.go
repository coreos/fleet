package ssh

import (
	"bufio"
	"errors"
	"net"
	"os"
	"time"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/gosshnew/ssh"
	gosshagent "github.com/coreos/fleet/third_party/code.google.com/p/gosshnew/ssh/agent"
	"github.com/coreos/fleet/third_party/code.google.com/p/gosshnew/ssh/terminal"
)

func Execute(client *gossh.Client, cmd string) (*bufio.Reader, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	stdout, _ := session.StdoutPipe()
	bstdout := bufio.NewReader(stdout)

	session.Start(cmd)
	go session.Wait()

	return bstdout, nil
}

func Shell(client *gossh.Client) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	modes := gossh.TerminalModes{
		gossh.ECHO:          1,     // enable echoing
		gossh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		gossh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	fd := int(os.Stdin.Fd())
	oldState, err := terminal.MakeRaw(fd)
	defer terminal.Restore(fd, oldState)

	termWidth, termHeight, err := terminal.GetSize(fd)
	if err != nil {
		return err
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	if err := session.RequestPty("xterm-256color", termHeight, termWidth, modes); err != nil {
		return err
	}

	if err = session.Shell(); err != nil {
		return err
	}

	session.Wait()
	return nil
}

func sshAgentClient() (gosshagent.Agent, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK environment variable is not set. Verify ssh-agent is running. See https://github.com/coreos/fleet/blob/master/Documentation/remote-access.md for help.")
	}

	agent, err := net.Dial("unix", sock)
	if err != nil {
		return nil, err
	}

	return gosshagent.NewClient(agent), nil
}

func sshClientConfig(user string, checker *HostKeyChecker) (*gossh.ClientConfig, error) {
	agentClient, err := sshAgentClient()
	if err != nil {
		return nil, err
	}

	signers, err := agentClient.Signers()
	if err != nil {
		return nil, err
	}

	cfg := gossh.ClientConfig{
		User: user,
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(signers...),
		},
		HostKeyCallback: checker.Check,
	}

	return &cfg, nil
}

func NewSSHClient(user, addr string, checker *HostKeyChecker) (*gossh.Client, error) {
	clientConfig, err := sshClientConfig(user, checker)
	if err != nil {
		return nil, err
	}

	var client *gossh.Client
	dialFunc := func(echan chan error) {
		var err error
		client, err = gossh.Dial("tcp", addr, clientConfig)
		echan <- err
	}
	err = timeoutSSHDial(dialFunc)
	return client, err
}

func NewTunnelledSSHClient(user, tunaddr, tgtaddr string, checker *HostKeyChecker) (*gossh.Client, error) {
	clientConfig, err := sshClientConfig(user, checker)
	if err != nil {
		return nil, err
	}

	var tunnelClient *gossh.Client
	dialFunc := func(echan chan error) {
		var err error
		tunnelClient, err = gossh.Dial("tcp", tunaddr, clientConfig)
		echan <- err
	}
	err = timeoutSSHDial(dialFunc)
	if err != nil {
		return nil, err
	}

	var targetConn net.Conn
	dialFunc = func(echan chan error) {
		targetConn, err = tunnelClient.Dial("tcp", tgtaddr)
		echan <- err
	}
	err = timeoutSSHDial(dialFunc)
	if err != nil {
		return nil, err
	}

	c, chans, reqs, err := gossh.NewClientConn(targetConn, tgtaddr, clientConfig)
	if err != nil {
		return nil, err
	}
	return gossh.NewClient(c, chans, reqs), nil
}

func timeoutSSHDial(dial func(chan error)) error {
	var err error

	echan := make(chan error)
	go dial(echan)

	select {
	case <-time.After(time.Duration(time.Second * 10)):
		return errors.New("Timed out while initiating SSH connection")
	case err = <-echan:
		return err
	}
}
