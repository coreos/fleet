// Copyright 2016 The fleet Authors
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

package rpc

import (
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/coreos/fleet/protobuf"
)

func TestRPCRegistryClientCreation(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	_, port, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse listener address: %v", err)
	}
	addr := "localhost:" + port
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(5*time.Second), grpc.WithBlock())
	if err != nil {
		t.Fatalf("failed to dial to the server %q: %v", addr, err)
	}
	// Close unaccepted connection (i.e., conn).
	lis.Close()

	registryClient := pb.NewRegistryClient(conn)
	if registryClient == nil {
		t.Fatalf("failed to create a new grpc registry to the server %q: %v", addr, err)
	}
}
