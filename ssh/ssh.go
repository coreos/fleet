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

type SSHForwardingClient struct {
	agentForwarding bool
	*gossh.Client
}

func (s *SSHForwardingClient) ForwardAgentAuthentication(session *gossh.Session) error {
	if s.agentForwarding {
		return gosshagent.RequestAgentForwarding(session)
	}
	return nil
}

func newSSHForwardingClient(client *gossh.Client, agentForwarding bool) (*SSHForwardingClient, error) {
	a, err := sshAgentClient()
	if err != nil {
		return nil, err
	}

	err = gosshagent.ForwardToAgent(client, a)
	if err != nil {
		return nil, err
	}

	return &SSHForwardingClient{agentForwarding, client}, nil
}

type Channel struct {
	Stdout *bufio.Reader
	Stderr *bufio.Reader
	Exit   chan error
}

func Execute(client *SSHForwardingClient, cmd string) (*Channel, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	if err = client.ForwardAgentAuthentication(session); err != nil {
		return nil, err
	}

	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()

	channel := &Channel{
		bufio.NewReader(stdout),
		bufio.NewReader(stderr),
		make(chan error),
	}

	session.Start(cmd)
	go func() {
		err := session.Wait()

		channel.Exit <- err
		session.Close()
	}()

	return channel, nil
}

func Shell(client *SSHForwardingClient) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	if err = client.ForwardAgentAuthentication(session); err != nil {
		return err
	}

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

func NewSSHClient(user, addr string, checker *HostKeyChecker, agentForwarding bool) (*SSHForwardingClient, error) {
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
	if err != nil {
		return nil, err
	}

	return newSSHForwardingClient(client, agentForwarding)
}

func NewTunnelledSSHClient(user, tunaddr, tgtaddr string, checker *HostKeyChecker, agentForwarding bool) (*SSHForwardingClient, error) {
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
		tgtTCPAddr, err := net.ResolveTCPAddr("tcp", tgtaddr)
		if err != nil {
			echan <- err
			return
		}
		targetConn, err = tunnelClient.DialTCP("tcp", nil, tgtTCPAddr)
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
	return newSSHForwardingClient(gossh.NewClient(c, chans, reqs), agentForwarding)
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
