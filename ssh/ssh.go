package ssh

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"time"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh"
	"github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh/terminal"
)

var (
	ErrKeyOutofIndex = errors.New("key index is out of range")
	ErrMalformedResp = errors.New("malformed signature response from agent client")
)

func Execute(client *gossh.ClientConn, cmd string) (*bufio.Reader, error) {
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

func Shell(client *gossh.ClientConn) error {
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

func sshAgentClient() (*gossh.AgentClient, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK environment variable is not set. Verify ssh-agent is running. See https://github.com/coreos/fleet/blob/master/Documentation/remote-access.md for help.")
	}

	agent, err := net.Dial("unix", sock)
	if err != nil {
		return nil, err
	}

	return gossh.NewAgentClient(agent), nil
}

func sshClientConfig(user string, checker gossh.HostKeyChecker) (*gossh.ClientConfig, error) {
	agentClient, err := sshAgentClient()
	if err != nil {
		return nil, err
	}

	cfg := gossh.ClientConfig{
		User: user,
		Auth: []gossh.ClientAuth{
			gossh.ClientAuthAgent(agentClient),
		},
		HostKeyChecker: checker,
	}

	return &cfg, nil
}

func NewSSHClient(user, addr string, checker gossh.HostKeyChecker) (*gossh.ClientConn, error) {
	clientConfig, err := sshClientConfig(user, checker)
	if err != nil {
		return nil, err
	}

	var client *gossh.ClientConn
	dialFunc := func(echan chan error) {
		var err error
		client, err = gossh.Dial("tcp", addr, clientConfig)
		echan <- err
	}
	err = timeoutSSHDial(dialFunc)
	return client, err
}

func NewTunnelledSSHClient(user, tunaddr, tgtaddr string, checker gossh.HostKeyChecker) (*gossh.ClientConn, error) {
	clientConfig, err := sshClientConfig(user, checker)
	if err != nil {
		return nil, err
	}

	var tunnelClient *gossh.ClientConn
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
		var err error
		targetConn, err = tunnelClient.Dial("tcp", tgtaddr)
		echan <- err
	}
	err = timeoutSSHDial(dialFunc)
	if err != nil {
		return nil, err
	}

	targetClient, err := gossh.Client(targetConn, clientConfig)
	if err != nil {
		return nil, err
	}

	return targetClient, nil
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

// AgentKeyring implements the interface of gossh.ClientKeyring.
type AgentKeyring struct {
	client *gossh.AgentClient
	keys   []*gossh.AgentKey
}

// NewSSHAgentKeyring inits AgentKeyring variable and returns
func NewSSHAgentKeyring() (*AgentKeyring, error) {
	client, err := sshAgentClient()
	if err != nil {
		return nil, err
	}

	return &AgentKeyring{client, nil}, nil
}

// Key returns i-th key in the keyring
func (ak *AgentKeyring) Key(i int) (gossh.PublicKey, error) {
	if ak.keys == nil {
		var err error
		if ak.keys, err = ak.client.RequestIdentities(); err != nil {
			return nil, err
		}
	}

	if i >= len(ak.keys) || i < 0 {
		return nil, ErrKeyOutofIndex
	}
	return ak.keys[i].Key()
}

func parseString(in []byte) (out, rest []byte, ok bool) {
	if len(in) < 4 {
		return
	}
	// First 4-byte is the length of the field
	length := binary.BigEndian.Uint32(in)
	if uint32(len(in)) < length+4 {
		return
	}
	return in[4:length+4], in[length+4:], true
}

// Sign returns the signing of data using i-th key in the keyring
func (ak *AgentKeyring) Sign(i int, rand io.Reader, data []byte) ([]byte, error) {
	key, err := ak.Key(i)
	if err != nil {
		return nil, err
	}

	sig, err := ak.client.SignRequest(key, data)
	if err != nil {
		return nil, err
	}

	// Unmarshal the signature
	var ok bool
	if _, sig, ok = parseString(sig); !ok {
		return nil, ErrMalformedResp
	}
	if sig, _, ok = parseString(sig); !ok {
		return nil, ErrMalformedResp
	}
	return sig, nil
}
