package ssh

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/gosshnew/ssh"
	glog "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/pkg"
)

const (
	DefaultKnownHostsFile = "~/.fleetctl/known_hosts"

	sshDefaultPort = 22  // ssh.h
	sshHashDelim   = "|" // hostfile.h

	warningRemoteHostChanged = `@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!
Someone could be eavesdropping on you right now (man-in-the-middle attack)!
It is also possible that a host key has just been changed.
The fingerprint for the %v key sent by the remote host is
%v.
Please contact your system administrator.
Add correct host key in %v to get rid of this message.
Host key verification failed.
`
)

var (
	ErrUnsetTrustFunc = errors.New("unset trustHost function")
	ErrUntrustHost    = errors.New("unauthorized host")
	ErrUnmatchKey     = errors.New("host key mismatch")
)

// HostKeyChecker implements the gossh.HostKeyChecker interface
// It is used for key validation during the cryptographic handshake
type HostKeyChecker struct {
	m         HostKeyManager
	trustHost func(addr, algo, fingerprint string) bool
	// errLog is used to print out error/warning message
	errLog *log.Logger
}

// NewHostKeyChecker returns a new HostKeyChecker
// m manages existing host keys, trustHost tells whether or not to trust
// new host, errWriter indicates the place to write error msg
func NewHostKeyChecker(m HostKeyManager, trustHost func(addr, algo, fingerprint string) bool, errWriter io.Writer) *HostKeyChecker {
	if errWriter == nil {
		errWriter = os.Stderr
	}

	return &HostKeyChecker{m, trustHost, log.New(errWriter, "", 0)}
}

// Check is called during the handshake to check the server's public key for
// unexpected changes. The key argument is in SSH wire format. It can be parsed
// using ssh.ParsePublicKey. The address before DNS resolution is passed in the
// addr argument, so the key can also be checked against the hostname.
// It returns any error encountered while checking the public key. A nil return
// value indicates that the key was either successfully verified (against an
// existing known_hosts entry), or accepted by the user as a new key.
func (kc *HostKeyChecker) Check(addr string, remote net.Addr, key gossh.PublicKey) error {
	remoteAddr, err := kc.addrToHostPort(remote.String())
	if err != nil {
		return err
	}

	algoStr := algoString(key.Type())
	keyFingerprintStr := md5String(md5.Sum(key.Marshal()))

	hostKeys, err := kc.m.GetHostKeys()
	_, ok := err.(*os.PathError)
	if err != nil && !ok {
		kc.errLog.Println("Failed to read known_hosts file %v: %v", kc.m.String(), err)
	}

	mismatched := false
	for pattern, keys := range hostKeys {
		if !matchHost(remoteAddr, pattern) {
			continue
		}
		for _, hostKey := range keys {
			// Any matching key is considered a success, irrespective of previous
			if hostKey.Type() == key.Type() && bytes.Compare(hostKey.Marshal(), key.Marshal()) == 0 {
				return nil
			} else {
				mismatched = true
			}
		}
	}

	if mismatched {
		kc.errLog.Printf(warningRemoteHostChanged, algoStr, keyFingerprintStr, kc.m.String())
		return ErrUnmatchKey
	}

	// If we get this far, we haven't matched on any of the hostname patterns,
	// so it's considered a new key

	if kc.trustHost == nil {
		return ErrUnsetTrustFunc
	}
	if !kc.trustHost(remoteAddr, algoStr, keyFingerprintStr) {
		kc.errLog.Println("Host key verification failed.")
		return ErrUntrustHost
	}

	if err := kc.m.PutHostKey(remoteAddr, key); err != nil {
		kc.errLog.Printf("Failed to add the host to the list of known hosts (%v).\n", kc.m)
		return nil
	}

	kc.errLog.Printf("Warning: Permanently added '%v' (%v) to the list of known hosts.\n", remoteAddr, algoStr)
	return nil
}

// addrToHostPort takes the given address and parses it into a string suitable
// for use in the 'hostnames' field in a known_hosts file.  For more details,
// see the `SSH_KNOWN_HOSTS FILE FORMAT` section of `man 8 sshd`
func (kc *HostKeyChecker) addrToHostPort(a string) (string, error) {
	if !strings.Contains(a, ":") {
		// No port, so return unadulterated
		return a, nil
	}
	host, p, err := net.SplitHostPort(a)
	if err != nil {
		kc.errLog.Printf("Unable to parse addr %s: %v", a, err)
		return "", err
	}

	port, err := strconv.Atoi(p)
	if err != nil {
		kc.errLog.Printf("Error parsing port %s: %v", p, err)
		return "", err
	}

	// see `put_host_port` in openssh/misc.c
	if port == 0 || port == sshDefaultPort {
		// IPv6 addresses must be enclosed in square brackets
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		return host, nil
	}

	return net.JoinHostPort(host, p), nil
}

// HostKeyManager defines an interface for managing "known hosts" keys
type HostKeyManager interface {
	String() string
	// GetHostKeys returns a map from host patterns to a list of PublicKeys
	GetHostKeys() (map[string][]gossh.PublicKey, error)
	// put new host key under management
	PutHostKey(addr string, hostKey gossh.PublicKey) error
}

// HostKeyFile is an implementation of HostKeyManager that saves and loads
// "known hosts" keys from a file
type HostKeyFile struct {
	path string
}

// NewHostKeyFile returns a new HostKeyFile using the given file path
func NewHostKeyFile(path string) *HostKeyFile {
	return &HostKeyFile{pkg.ParseFilepath(path)}
}

func (f *HostKeyFile) String() string {
	return f.path
}

func (f *HostKeyFile) GetHostKeys() (map[string][]gossh.PublicKey, error) {
	in, err := os.Open(f.path)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	hostKeys := make(map[string][]gossh.PublicKey)
	n := 0
	s := bufio.NewScanner(in)
	for s.Scan() {
		n++
		line := s.Bytes()

		// Skip any leading whitespace.
		line = bytes.TrimLeft(line, "\t ")

		// Skip comments and empty lines.
		if bytes.HasPrefix(line, []byte("#")) || len(line) == 0 {
			continue
		}

		// Skip markers.
		if bytes.HasPrefix(line, []byte("@")) {
			glog.V(2).Infof("Marker functionality not implemented - skipping line %d", n)
			continue
		}

		// Find the end of the host name(s) portion.
		end := bytes.IndexAny(line, "\t ")
		if end <= 0 {
			glog.V(2).Infof("Bad format (insufficient fields) - skipping line %d", n)
			continue
		}
		hosts := string(line[:end])
		keyBytes := line[end+1:]

		// Check for hashed host names.
		if strings.HasPrefix(hosts, sshHashDelim) {
			glog.V(2).Infof("Hashed hosts not implemented - skipping line %d", n)
			continue
		}

		key, _, _, _, err := gossh.ParseAuthorizedKey(keyBytes)
		if err != nil {
			glog.V(2).Infof("Error parsing key, skipping line %d: %v", n, err)
			continue
		}

		// It is permissible to have several lines for the same host name(s)
		hostKeys[hosts] = append(hostKeys[hosts], key)
	}

	return hostKeys, nil
}

func (f *HostKeyFile) PutHostKey(addr string, hostKey gossh.PublicKey) error {
	// Make necessary directories if needed
	err := os.MkdirAll(path.Dir(f.path), 0700)
	if err != nil {
		return err
	}

	out, err := os.OpenFile(f.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.Write(renderHostLine(addr, hostKey))
	if err != nil {
		return err
	}
	return nil
}

func renderHostLine(addr string, key gossh.PublicKey) []byte {
	keyByte := gossh.MarshalAuthorizedKey(key)
	// allocate line space in advance
	length := len(addr) + 1 + len(keyByte)
	line := make([]byte, 0, length)

	w := bytes.NewBuffer(line)
	w.Write([]byte(addr))
	w.WriteByte(' ')
	w.Write(keyByte)
	return w.Bytes()
}

// algoString returns a short-name representation of an algorithm type
func algoString(algo string) string {
	switch algo {
	case gossh.KeyAlgoRSA:
		return "RSA"
	case gossh.KeyAlgoDSA:
		return "DSA"
	case gossh.KeyAlgoECDSA256, gossh.KeyAlgoECDSA384, gossh.KeyAlgoECDSA521:
		return "ECDSA"
	}
	return algo
}

// md5String returns a formatted string representing the given md5Sum in hex
func md5String(md5Sum [16]byte) string {
	md5Str := fmt.Sprintf("% x", md5Sum)
	md5Str = strings.Replace(md5Str, " ", ":", -1)
	return md5Str
}
