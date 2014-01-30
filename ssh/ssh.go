package ssh

import (
	"bufio"
	"net"
	"os"

	gossh "code.google.com/p/go.crypto/ssh"
	"code.google.com/p/go.crypto/ssh/terminal"
)

func Execute(user, addr, cmd string) (*bufio.Reader, error) {
	client, err := NewSSHClient(user, addr)
	if err != nil {
		return nil, err
	}

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

func Shell(user, addr string) error {
	client, err := NewSSHClient(user, addr)
	if err != nil {
		return err
	}
	defer client.Close()

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

	err = session.Wait()
	return err
}

func NewSSHClient(user, addr string) (*gossh.ClientConn, error) {
	agent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}

	auths := []gossh.ClientAuth{
		gossh.ClientAuthAgent(gossh.NewAgentClient(agent)),
	}

	clientConfig := &gossh.ClientConfig{
		User: user,
		Auth: auths,
	}

	return gossh.Dial("tcp", addr, clientConfig)
}
