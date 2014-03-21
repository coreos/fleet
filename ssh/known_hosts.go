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
	"strings"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh"

	"github.com/coreos/fleet/pkg"
)

const (
	DefaultKnownHostsFile = "~/.fleetctl/known_hosts"

	warningRemoteHostChanged =
`@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
IT IS POSSIBLE THAT SOMEONE IS DOING SOMETHING NASTY!`
)

var (
	ErrUnparsableKey   = errors.New("unparsable key bytes")
	ErrUnsetTrustFunc  = errors.New("unset trustHost function")
	ErrUntrustHost     = errors.New("unauthorized host")
	ErrUnmatchKey      = errors.New("host key mismatch")
	ErrUnfoundHostAddr = errors.New("cannot find out host address")
)

// HostKeyChecker implements gossh.HostKeyChecker interface
// It is used for validation during the cryptographic handshake
type HostKeyChecker struct {
	m         HostKeyManager
	trustHost func(addr, algo, fingerprint string) bool
	// errLog is used to print out error/warning message
	errLog    *log.Logger
}

// NewHostKeyChecker returns new HostKeyChecker
// m manages existing host keys, trustHost tells whether or not to trust
// new host, errWriter indicates the place to write error msg
func NewHostKeyChecker(m HostKeyManager, trustHost func(addr, algo, fingerprint string) bool, errWriter io.Writer) *HostKeyChecker {
	if errWriter == nil {
		errWriter = os.Stderr
	}

	return &HostKeyChecker{m, trustHost, log.New(errWriter, "", 0)}
}

// SetTrustHost sets trustHost field
func (kc *HostKeyChecker) SetTrustHost(trustHost func(addr, algo, fingerprint string) bool) {
	kc.trustHost = trustHost
}

// Check is called during the handshake to check server's
// public key for unexpected changes. The hostKey argument is
// in SSH wire format. It can be parsed using
// ssh.ParsePublicKey. The address before DNS resolution is
// passed in the addr argument, so the key can also be checked
// against the hostname.
func (kc *HostKeyChecker) Check(addr string, remote net.Addr, algo string, keyByte []byte) error {
	key, _, ok := gossh.ParsePublicKey(keyByte)
	if !ok {
		return ErrUnparsableKey
	}

	remoteAddr := remote.String()
	algoStr := algoString(algo)
	keyFingerprintStr := md5String(md5.Sum(keyByte))

	// get existing host keys
	hostKeys, err := kc.m.GetHostKeys()
	_, ok = err.(*os.PathError)
	if err != nil && !ok {
		kc.errLog.Println("Warning: read host file with", err)
	}

	// check existing host keys
	hostKey, ok := hostKeys[remoteAddr]
	if !ok {
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

	if hostKey.PublicKeyAlgo() != algo || bytes.Compare(hostKey.Marshal(), key.Marshal()) != 0 {
		kc.errLog.Printf(`%s
Someone could be eavesdropping on you right now (man-in-the-middle attack)!
It is also possible that a host key has just been changed.
The fingerprint for the %v key sent by the remote host is
%v.
Please contact your system administrator.
Add correct host key in %v to get rid of this message.
Host key verification failed.%c`,
		warningRemoteHostChanged, algoStr, keyFingerprintStr, kc.m.String(), '\n')

		return ErrUnmatchKey
	}
	return nil
}

// HostKeyManager gives the interface to manage host keys
type HostKeyManager interface {
	String() string
	// get all host keys
	GetHostKeys() (map[string]gossh.PublicKey, error)
	// put new host key under management
	PutHostKey(addr string, hostKey gossh.PublicKey) error
}

// HostKeyFile is an implementation of HostKeyManager interface
// Host keys are saved and loaded from file
type HostKeyFile struct {
	path string
}

// NewHostKeyFile returns new HostKeyFile using file path
func NewHostKeyFile(path string) *HostKeyFile {
	return &HostKeyFile{pkg.ParseFilepath(path)}
}

func (f *HostKeyFile) String() string {
	return f.path
}

func (f *HostKeyFile) GetHostKeys() (map[string]gossh.PublicKey, error) {
	in, err := os.Open(f.path)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	hostKeys := make(map[string]gossh.PublicKey)
	s := bufio.NewScanner(in)
	lineNo := 0
	for s.Scan() {
		lineNo++
		addr, hostKey, err := parseHostLine(s.Bytes())
		if err == nil {
			hostKeys[addr] = hostKey
		}
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

func parseHostLine(line []byte) (string, gossh.PublicKey, error) {
	end := bytes.IndexByte(line, ' ')
	if end <= 0 {
		return "", nil, ErrUnfoundHostAddr
	}
	addr := string(line[:end])
	keyByte := line[end+1:]
	key, _, _, _, ok := gossh.ParseAuthorizedKey(keyByte)
	if !ok {
		return "", nil, ErrUnparsableKey
	}
	return addr, key, nil
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

func algoString(algo string) string {
	switch algo {
	case gossh.KeyAlgoRSA:
		return "RSA"
	case gossh.KeyAlgoDSA:
		return "DSA"
	case gossh.KeyAlgoECDSA256:
		return "ECDSA256"
	case gossh.KeyAlgoECDSA384:
		return "ECDSA384"
	case gossh.KeyAlgoECDSA521:
		return "ECDSA521"
	}
	return algo
}

func md5String(md5Sum [16]byte) string {
	md5Str := fmt.Sprintf("% x", md5Sum)
	md5Str = strings.Replace(md5Str, " ", ":", -1)
	return md5Str
}
