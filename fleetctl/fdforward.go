// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"io"
	"net"
	"os"
)

var (
	cmdFDForward = &Command{
		Name:        "fd-forward",
		Summary:     "Proxy stdin and stdout to a unix domain socket",
		Usage:       "SOCKET",
		Description: `fleetctl utilizes fd-forward when --tunnel is used and --endpoint is a unix socket. This command is not intended to be called by users directly.`,
		Run:         runFDForward,
	}
)

func runFDForward(args []string) (exit int) {
	if len(args) != 1 {
		stderr("Provide a single argument")
		return 1
	}

	to := args[0]

	raddr, err := net.ResolveUnixAddr("unix", to)
	if err != nil {
		stderr("Unable to use %q as unix address: %v", to, err)
		return 1
	}

	sock, err := net.DialUnix("unix", nil, raddr)
	if err != nil {
		stderr("Failed dialing remote address: %v", err)
		return
	}
	defer sock.Close()

	errchan := make(chan error)
	go cp(os.Stdout, sock, errchan)
	go cp(sock, os.Stdin, errchan)

	select {
	case err := <-errchan:
		if err != nil {
			stderr("Encountered error during copy: %v", err)
			return 1
		}
	}

	return
}

func cp(to io.Writer, from io.Reader, errchan chan error) {
	_, err := io.Copy(to, from)
	errchan <- err
}
