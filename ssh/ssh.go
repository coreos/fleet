package ssh

import (
	"bufio"
	"net"
	"os"

	gossh "code.google.com/p/go.crypto/ssh"
)

func Execute(user, addr, cmd string) (*bufio.Reader, error)  {
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


