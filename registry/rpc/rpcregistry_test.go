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
