package main

import (
	"net"
	"os"
	"log"

	gossh "code.google.com/p/go.crypto/ssh"
)

func ssh(user, addr string) (*gossh.Session, error)  {
	agent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	defer agent.Close()

	auths := []gossh.ClientAuth{
		gossh.ClientAuthAgent(gossh.NewAgentClient(agent)),
	}

	clientConfig := &gossh.ClientConfig{
		User: user,
		Auth: auths,
	}

	log.Printf("Dialing %s", addr)
	client, err := gossh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	return session, nil
}
