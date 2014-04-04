// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssh

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Listen requests the remote peer open a listening socket
// on addr. Incoming connections will be available by calling
// Accept on the returned net.Listener.
func (c *Client) Listen(n, addr string) (net.Listener, error) {
	laddr, err := net.ResolveTCPAddr(n, addr)
	if err != nil {
		return nil, err
	}
	return c.ListenTCP(laddr)
}

// Automatic port allocation is broken with OpenSSH before 6.0. See
// also https://bugzilla.mindrot.org/show_bug.cgi?id=2017.  In
// particular, OpenSSH 5.9 sends a channelOpenMsg with port number 0,
// rather than the actual port number. This means you can never open
// two different listeners with auto allocated ports. We work around
// this by trying explicit ports until we succeed.

const openSSHPrefix = "OpenSSH_"

var portRandomizer = rand.New(rand.NewSource(time.Now().UnixNano()))

// isBrokenOpenSSHVersion returns true if the given version string
// specifies a version of OpenSSH that is known to have a bug in port
// forwarding.
func isBrokenOpenSSHVersion(versionStr string) bool {
	i := strings.Index(versionStr, openSSHPrefix)
	if i < 0 {
		return false
	}
	i += len(openSSHPrefix)
	j := i
	for ; j < len(versionStr); j++ {
		if versionStr[j] < '0' || versionStr[j] > '9' {
			break
		}
	}
	version, _ := strconv.Atoi(versionStr[i:j])
	return version < 6
}

// autoPortListenWorkaround simulates automatic port allocation by
// trying random ports repeatedly.
func (c *Client) autoPortListenWorkaround(laddr *net.TCPAddr) (net.Listener, error) {
	var sshListener net.Listener
	var err error
	const tries = 10
	for i := 0; i < tries; i++ {
		addr := *laddr
		addr.Port = 1024 + portRandomizer.Intn(60000)
		sshListener, err = c.ListenTCP(&addr)
		if err == nil {
			laddr.Port = addr.Port
			return sshListener, err
		}
	}
	return nil, fmt.Errorf("ssh: listen on random port failed after %d tries: %v", tries, err)
}

// RFC 4254 7.1
type channelForwardMsg struct {
	addr  string
	rport uint32
}

// ListenTCP requests the remote peer open a listening socket
// on laddr. Incoming connections will be available by calling
// Accept on the returned net.Listener.
func (c *Client) ListenTCP(laddr *net.TCPAddr) (net.Listener, error) {
	if laddr.Port == 0 && isBrokenOpenSSHVersion(string(c.ServerVersion())) {
		return c.autoPortListenWorkaround(laddr)
	}

	m := channelForwardMsg{
		laddr.IP.String(),
		uint32(laddr.Port),
	}
	// send message
	ok, resp, err := c.SendRequest("tcpip-forward", true, Marshal(&m))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("tcpip-forward denied")
	}

	// If the original port was 0, then the remote side will
	// supply a real port number in the response.
	if laddr.Port == 0 {
		var p struct {
			Port uint32
		}
		if err := Unmarshal(resp, &p); err != nil {
			return nil, err
		}
		laddr.Port = int(p.Port)
	}

	// Register this forward, using the port number we obtained.
	ch := c.forwards.add(*laddr)

	return &tcpListener{laddr, c, ch}, nil
}

// forwardList stores a mapping between remote
// forward requests and the tcpListeners.
type forwardList struct {
	sync.Mutex
	entries []forwardEntry
}

// forwardEntry represents an established mapping of a laddr on a
// remote ssh server to a channel connected to a tcpListener.
type forwardEntry struct {
	laddr net.TCPAddr
	c     chan forward
}

// forward represents an incoming forwarded tcpip connection. The
// arguments to add/remove/lookup should be address as specified in
// the original forward-request.
type forward struct {
	c     Channel      // the ssh client channel underlying this forward
	raddr *net.TCPAddr // the raddr of the incoming connection
}

func (l *forwardList) add(addr net.TCPAddr) chan forward {
	l.Lock()
	defer l.Unlock()
	f := forwardEntry{
		addr,
		make(chan forward, 1),
	}
	l.entries = append(l.entries, f)
	return f.c
}

func (l *forwardList) handleChannels(in <-chan NewChannel) {
	for ch := range in {
		laddr, rest, ok := parseTCPAddr(ch.ExtraData())
		if !ok {
			// invalid request
			ch.Reject(ConnectionFailed, "could not parse TCP address")
			return
		}

		l, ok := l.lookup(*laddr)
		if !ok {
			// Section 7.2, implementations MUST reject spurious incoming
			// connections.
			ch.Reject(Prohibited, "no forward for address")
			return
		}

		raddr, rest, ok := parseTCPAddr(rest)
		if !ok {
			// invalid request
			ch.Reject(ConnectionFailed, "could not parse TCP address")
			return
		}

		if ch, incoming, err := ch.Accept(); err == nil {
			go DiscardRequests(incoming)
			l <- forward{ch, raddr}
		}
	}
}

// remove removes the forward entry, and the channel feeding its
// listener.
func (l *forwardList) remove(addr net.TCPAddr) {
	l.Lock()
	defer l.Unlock()
	for i, f := range l.entries {
		if addr.IP.Equal(f.laddr.IP) && addr.Port == f.laddr.Port {
			l.entries = append(l.entries[:i], l.entries[i+1:]...)
			close(f.c)
			return
		}
	}
}

// closeAll closes and clears all forwards.
func (l *forwardList) closeAll() {
	l.Lock()
	defer l.Unlock()
	for _, f := range l.entries {
		close(f.c)
	}
	l.entries = nil
}

func (l *forwardList) lookup(addr net.TCPAddr) (chan forward, bool) {
	l.Lock()
	defer l.Unlock()
	for _, f := range l.entries {
		if addr.IP.Equal(f.laddr.IP) && addr.Port == f.laddr.Port {
			return f.c, true
		}
	}
	return nil, false
}

type tcpListener struct {
	laddr *net.TCPAddr

	conn *Client
	in   <-chan forward
}

// Accept waits for and returns the next connection to the listener.
func (l *tcpListener) Accept() (net.Conn, error) {
	s, ok := <-l.in
	if !ok {
		return nil, io.EOF
	}
	return &tcpChanConn{
		Channel: s.c,
		laddr:   l.laddr,
		raddr:   s.raddr,
	}, nil
}

// Close closes the listener.
func (l *tcpListener) Close() error {
	m := channelForwardMsg{
		l.laddr.IP.String(),
		uint32(l.laddr.Port),
	}

	// this also closes the listener.
	l.conn.forwards.remove(*l.laddr)
	ok, _, err := l.conn.SendRequest("cancel-tcpip-forward", true, Marshal(&m))
	if err == nil && !ok {
		err = errors.New("ssh: cancel-tcpip-forward failed")
	}
	return err
}

// Addr returns the listener's network address.
func (l *tcpListener) Addr() net.Addr {
	return l.laddr
}

// Dial initiates a connection to the addr from the remote host.
// The resulting connection has a zero LocalAddr() and RemoteAddr().
func (c *Client) Dial(n, addr string) (net.Conn, error) {
	// Parse the address into host and numeric port.
	host, portString, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.ParseUint(portString, 10, 16)
	if err != nil {
		return nil, err
	}
	// Use a zero address for local and remote address.
	zeroAddr := &net.TCPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	}
	ch, err := c.dial(net.IPv4zero.String(), 0, host, int(port))
	if err != nil {
		return nil, err
	}
	return &tcpChanConn{
		Channel: ch,
		laddr:   zeroAddr,
		raddr:   zeroAddr,
	}, nil
}

// DialTCP connects to the remote address raddr on the network net,
// which must be "tcp", "tcp4", or "tcp6".  If laddr is not nil, it is used
// as the local address for the connection.
func (c *Client) DialTCP(n string, laddr, raddr *net.TCPAddr) (net.Conn, error) {
	if laddr == nil {
		laddr = &net.TCPAddr{
			IP:   net.IPv4zero,
			Port: 0,
		}
	}
	ch, err := c.dial(laddr.IP.String(), laddr.Port, raddr.IP.String(), raddr.Port)
	if err != nil {
		return nil, err
	}
	return &tcpChanConn{
		Channel: ch,
		laddr:   laddr,
		raddr:   raddr,
	}, nil
}

// RFC 4254 7.2
type channelOpenDirectMsg struct {
	raddr string
	rport uint32
	laddr string
	lport uint32
}

func (c *Client) dial(laddr string, lport int, raddr string, rport int) (Channel, error) {
	msg := channelOpenDirectMsg{
		raddr: raddr,
		rport: uint32(rport),
		laddr: laddr,
		lport: uint32(lport),
	}
	ch, in, err := c.OpenChannel("direct-tcpip", Marshal(&msg))
	go DiscardRequests(in)
	return ch, err
}

type tcpChan struct {
	Channel // the backing channel
}

// tcpChanConn fulfills the net.Conn interface without
// the tcpChan having to hold laddr or raddr directly.
type tcpChanConn struct {
	Channel
	laddr, raddr net.Addr
}

// LocalAddr returns the local network address.
func (t *tcpChanConn) LocalAddr() net.Addr {
	return t.laddr
}

// RemoteAddr returns the remote network address.
func (t *tcpChanConn) RemoteAddr() net.Addr {
	return t.raddr
}

// SetDeadline sets the read and write deadlines associated
// with the connection.
func (t *tcpChanConn) SetDeadline(deadline time.Time) error {
	if err := t.SetReadDeadline(deadline); err != nil {
		return err
	}
	return t.SetWriteDeadline(deadline)
}

// SetReadDeadline sets the read deadline.
// A zero value for t means Read will not time out.
// After the deadline, the error from Read will implement net.Error
// with Timeout() == true.
func (t *tcpChanConn) SetReadDeadline(deadline time.Time) error {
	return errors.New("ssh: tcpChan: deadline not supported")
}

// SetWriteDeadline exists to satisfy the net.Conn interface
// but is not implemented by this type.  It always returns an error.
func (t *tcpChanConn) SetWriteDeadline(deadline time.Time) error {
	return errors.New("ssh: tcpChan: deadline not supported")
}
