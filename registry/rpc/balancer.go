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
	"sync"
	"sync/atomic"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// simpleBalancer implements grpc.Balancer interface, being as simple as possible.
// to be used only for fleet.
//
// In principle grpc.Balancer is meant to be handling load balancer across
// multiple connections via addresses for RPCs.
//  * Start() does initialization work to bootstrap a Balancer.
//  * Up() informs the Balancer that gRPC has a connection to the server at addr.
//    It returns Down() which is called once the connection to addr gets lost
//    or closed.
//  * Get() gets the address of a server for the RPC corresponding to ctx.
//  * Notify() returns a channel that is used by gRPC internals to watch the
//    addresses gRPC needs to connect.
//  * Close() shuts down the balancer.
//
// However, as fleet needs to care only about a single connection, simpleBalancer
// in fleet should be kept as simple as possible. Most crucially simpleBalancer
// provides a simple channel, readyc, to notify the rpcRegistry of the connection
// being available. readyc gets closed in Up(), which will cause, for example,
// IsRegistryReady() to recognize that the connection is available. We don't need
// to care about which value the readyc has.
type simpleBalancer struct {
	addrs   []string
	numGets uint32

	// readyc closes once the first connection is up
	readyc    chan struct{}
	readyOnce sync.Once
}

func newSimpleBalancer(eps []string) *simpleBalancer {
	return &simpleBalancer{
		addrs:  eps,
		readyc: make(chan struct{}),
	}
}

func (b *simpleBalancer) Start(target string) error { return nil }

func (b *simpleBalancer) Up(addr grpc.Address) func(error) {
	b.readyOnce.Do(func() { close(b.readyc) })
	return func(error) {}
}

func (b *simpleBalancer) Get(ctx context.Context, opts grpc.BalancerGetOptions) (grpc.Address, func(), error) {
	v := atomic.AddUint32(&b.numGets, 1)
	addr := b.addrs[v%uint32(len(b.addrs))]

	return grpc.Address{Addr: addr}, func() {}, nil
}

func (b *simpleBalancer) Notify() <-chan []grpc.Address { return nil }

func (b *simpleBalancer) Close() error {
	b.readyc = make(chan struct{})
	return nil
}
