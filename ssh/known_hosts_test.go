package ssh

import (
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"testing"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/gosshnew/ssh"
)

const (
	hostLine           = "192.0.2.10:2222 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC3y6omlFuiBQfV2lqwqt3EuQHXLxvghhdfyZ840je6pRNnidgfCTmzNgIjmqdfkCwIthh+fhArkFPWIT6dRwim4hhtbpum7AzAay1h6mmLsmJVJQ/nK+zLwQ4JHs6+Tfj6F3iXJyrZR9JMTeLLs0mEd+VNHbX3LxIh7nXk5IM0G5LP2nnIYG96Luu4WunJzFsDVFLgxMl66T9VBYeAIbfUeCoCDYMmJK7kTleLD1XfL2KdoHkh0t9fkJVA5XJUZJPh3PJw+mT7eP3meAMc8EzyCGcRm+5GQzAe2/M4dNaZ5iqF7YIO7HJpA8UyAE+Dgd9WqhoBX/6ItdcuDXVAy63v\n"
	addrInHostLine     = "192.0.2.10:2222"
	hostFile           = "../fixtures/known_hosts"
	wrongAuthorizedKey = "ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAzJjWHWVDum5WukrlWTYPtPN/Ny8BTXzhHFf89vejOQukQNMPcoohjSOBkrFZXQMLQ0s/RqpTKly1omdo8TgfUE5f7rgegwPhzleuxw/Q/XJJJiiCi7KHSQv9Vs+fNlMr14VsF8JStpKei5jD/moM1Pk/q5asYtY9I4+0rJRq1KbFPR4gTGlCqZApvJWfEHlgQxwlug6zFKaVy3vG04ggvS4GREd6XQeVjAE5cPY31Yrtdgll/BETHAxvy1+ucWxiFy6BNrqPni6XSOkSZc44EEIj4TCRAQdv5nZyd2VKPQHENYLDaC9KkxllZdqNuJuXx9stRv8auwOFRnF+JSk+7Q=="
	hostFileBackup     = "../fixtures/known_hosts_backup"
	wrongHostFile      = "../fixtures/wrong_known_hosts"
	badHostFile        = "../fixtures/bad_known_hosts"
)

func trustHostAlways(addr, algo, fingerprint string) bool {
	return true
}

func trustHostNever(addr, algo, fingerprint string) bool {
	return false
}

// TestHostKeyChecker tests to check existing key
func TestHostKeyChecker(t *testing.T) {
	keyFile := NewHostKeyFile(hostFile)
	checker := NewHostKeyChecker(keyFile, nil, ioutil.Discard)

	addr, key, _ := parseHostLine([]byte(hostLine))
	tcpAddr, _ := net.ResolveTCPAddr("tcp", addr)

	if err := checker.Check("localhost", tcpAddr, key); err != nil {
		t.Fatalf("checker should succeed for %v: %v", tcpAddr.String(), err)
	}

	wrongKey, _, _, _, _ := gossh.ParseAuthorizedKey([]byte(wrongAuthorizedKey))
	if err := checker.Check("localhost", tcpAddr, wrongKey); err != ErrUnmatchKey {
		t.Fatalf("checker should fail with %v", ErrUnmatchKey)
	}
}

// TestHostKeyCheckerInteraction tests to check nonexisting key
func TestHostKeyCheckerInteraction(t *testing.T) {
	os.Remove(hostFileBackup)
	defer os.Remove(hostFileBackup)

	keyFile := NewHostKeyFile(hostFileBackup)
	checker := NewHostKeyChecker(keyFile, trustHostNever, ioutil.Discard)

	addr, key, _ := parseHostLine([]byte(hostLine))
	tcpAddr, _ := net.ResolveTCPAddr("tcp", addr)

	// Refuse to add new host key
	if err := checker.Check("localhost", tcpAddr, key); err != ErrUntrustHost {
		t.Fatalf("checker should fail to put %v, %v in known_hosts", addr, tcpAddr.String())
	}

	// Accept to add new host key
	checker.trustHost = trustHostAlways
	if err := checker.Check("localhost", tcpAddr, key); err != nil {
		t.Fatalf("checker should succeed to put %v, %v in known_hosts", addr, tcpAddr.String())
	}

	// Use authorized key that have been added
	checker.trustHost = trustHostNever
	if err := checker.Check("localhost", tcpAddr, key); err != nil {
		t.Fatalf("checker should succeed to put %v, %v in known_hosts", addr, tcpAddr.String())
	}
}

// TestHostLine tests how to parse and render host line
func TestHostLine(t *testing.T) {
	addr, key, _ := parseHostLine([]byte(hostLine))
	if addr != addrInHostLine {
		t.Fatalf("addr is %v instead of %v", addr, addrInHostLine)
	}
	if key.Type() != gossh.KeyAlgoRSA {
		t.Fatalf("key type is %v instead of %v", key.Type(), gossh.KeyAlgoRSA)
	}

	line := renderHostLine(addr, key)
	if string(line) != hostLine {
		t.Fatal("unmatched host line after save and load")
	}
}

// TestHostKeyFile tests to read and write from HostKeyFile
/*
func TestHostKeyFile(t *testing.T) {
	os.Remove(hostFileBackup)
	defer os.Remove(hostFileBackup)

	in := NewHostKeyFile(hostFile)
	out := NewHostKeyFile(hostFileBackup)

	hostKeys, err := in.GetHostKeys()
	if err != nil {
		t.Fatal("reading host file error:", err)
	}

	for i, v := range hostKeys {
		if err = out.PutHostKey(i, v); err != nil {
			t.Fatal("append error:", err)
		}
	}

	keysByte, _ := ioutil.ReadFile(hostFile)
	keysByteBackup, _ := ioutil.ReadFile(hostFileBackup)
	keyBytes := bytes.Split(keysByte, []byte{'\n'})
	keyBytesBackup := bytes.Split(keysByteBackup, []byte{'\n'})
	for _, keyByte := range keyBytes {
		find := false
		for _, keyByteBackup := range keyBytesBackup {
			find = bytes.Compare(keyByte, keyByteBackup) == 0
			if find {
				break
			}
		}
		if !find {
			t.Fatalf("host file difference")
		}
	}
}
*/

// TestHostKeyFile tests to read and write from wrong HostKeyFile
func TestWrongHostKeyFile(t *testing.T) {
	f := NewHostKeyFile(wrongHostFile)
	_, err := f.GetHostKeys()
	if err == nil {
		t.Fatal("should fail to read wrong host file")
	}
	if _, ok := err.(*os.PathError); !ok {
		t.Fatal("should fail to read wrong host file due to file miss")
	}

	os.OpenFile(wrongHostFile, os.O_CREATE, 0000)
	defer os.Remove(wrongHostFile)
	err = f.PutHostKey("", nil)
	if err == nil {
		t.Fatal("append to wrong host file")
	}
}

// TestHostKeyFile tests to read from bad HostKeyFile
func TestBadHostKeyFile(t *testing.T) {
	f := NewHostKeyFile(badHostFile)
	hostKeys, _ := f.GetHostKeys()
	if len(hostKeys) > 0 {
		t.Fatal("read key from bad host file")
	}
}

// TestAlgorithmString tests the string representation of key algorithm
func TestAlgorithmString(t *testing.T) {
	if algoString(gossh.KeyAlgoRSA) != "RSA" {
		t.Fatal("wrong printout for RSA")
	}
	if algoString(gossh.KeyAlgoDSA) != "DSA" {
		t.Fatal("wrong printout for DSA")
	}
	if algoString(gossh.KeyAlgoECDSA256) != "ECDSA256" {
		t.Fatal("wrong printout for ECDSA256")
	}
	if algoString(gossh.KeyAlgoECDSA384) != "ECDSA384" {
		t.Fatal("wrong printout for ECDSA384")
	}
	if algoString(gossh.KeyAlgoECDSA521) != "ECDSA521" {
		t.Fatal("wrong printout for ECDSA521")
	}
	if algoString("UNKNOWN") != "UNKNOWN" {
		t.Fatal("wrong printout for UNKNOWN")
	}
}

func TestMD5String(t *testing.T) {
	sum := [16]byte{0, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	if md5String(sum) != "00:11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff" {
		t.Fatal("wrong md5 string conversion")
	}
}

func TestAddrToHostPort(t *testing.T) {
	keyFile := NewHostKeyFile(hostFile)
	checker := NewHostKeyChecker(keyFile, nil, ioutil.Discard)

	badAddrs := []string{
		"12:12:12",
		"foobar:baz",
		"[12:323",
		"[127.0.0.1:]",
		// raw IPv6 addresses should fail
		"2001:0db8:85a3:0000:0000:8a2e:0370:7334",
		"2001:db8:85a3:0:0:8a2e:370:7334",
		"2001:db8:85a3::8a2e:370:7334",
		"::1",
		"::",
		// IPv6 addresses without ports should fail
		"[2001:db8:85a3::8a2e:370:7334]",
		"[::1]",
	}

	for _, a := range badAddrs {
		_, err := checker.addrToHostPort(a)
		if err == nil {
			t.Errorf("addr %v did not fail hostport conversion!", a)
		}
	}

	goodAddrs := []struct {
		in  string
		out string
	}{
		{"foo.com", "foo.com"},
		{"127.0.0.1", "127.0.0.1"},
		{"127.0.0.1:" + strconv.Itoa(sshDefaultPort), "127.0.0.1"},
		{"127.0.0.1:0", "127.0.0.1"},
		{"127.0.0.1:12345", "127.0.0.1:12345"},
		// escaped IPv6 addresses with ports should succeed
		{"[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:22", "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]"},
		{"[2001:db8:85a3:0:0:8a2e:370:7334]:12345", "[2001:db8:85a3:0:0:8a2e:370:7334]:12345"},
		{"[2001:db8:85a3::8a2e:370:7334]:12345", "[2001:db8:85a3::8a2e:370:7334]:12345"},
		{"[::1]:22", "[::1]"},
	}

	for _, a := range goodAddrs {
		got, err := checker.addrToHostPort(a.in)
		if err != nil {
			t.Errorf("addr %s failed hostport conversation: %v", a.in, err)
			continue
		}
		if got != a.out {
			t.Errorf("bad hostport conversion for %s: got %s, want %s", a.in, got, a.out)
		}
	}
}

func TestMatchHost(t *testing.T) {
	tests := []struct {
		host    string
		pattern string
		success bool
	}{
		{"foo.com", "foo.com", true},
		{"foo.com", "foo.com,bar,baz", true},
		{"foo.com", "bar,foo.com,baz", true},
		{"foo.com", "!foo.com,bar,baz", false},
		{"foo.com", "foo.com,!foo.com,bar,baz", false},
	}
	for _, test := range tests {
		if matchHost(test.host, test.pattern) != test.success {
			t.Errorf("matching %v against %v: got %v, want %v!", test.host, test.pattern, test.success, !test.success)
		}
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		good    []string
		bad     []string
	}{
		{"foo",
			[]string{"foo"},
			[]string{"fob", "FOO"},
		},
		{"f*o",
			[]string{"foo", "fooo", "fasdfo"},
			[]string{"foc"},
		},
		{"f*",
			[]string{"fasdf", "f0939", "fa"},
			[]string{"g", "asdfadsff"},
		},
		{"f**",
			[]string{"fasdf", "f0939", "fa"},
			[]string{"g", "asdfasdfff"},
		},
		{"a*c?e",
			[]string{"abcde", "accce", "a2c3e", "abbcde", "acde"},
			[]string{"abce", "ace", "abbbbbcdde"},
		},
		{"a*c*e",
			[]string{"abcde", "accce", "a2c3e", "abbcde", "acde", "abbbcdddde"},
			[]string{"abc", "ae"},
		},
		{"a**c*e",
			[]string{"abcde", "accce", "a2c3e", "abbcde", "acde", "abbbcdddde"},
			[]string{"abc", "ae"},
		},
		{"f?b",
			[]string{"fob", "fab"},
			[]string{"fb", "feeb"},
		},
		{"h??d",
			[]string{"hood", "heed", "h12d"},
			[]string{"heldd", "hoob"},
		},
		{"a?c?e",
			[]string{"abcde", "accce", "a2c3e"},
			[]string{"abbcde", "abce", "ace"},
		},
	}

	for _, test := range tests {
		for _, s := range test.good {
			if !match(s, test.pattern) {
				t.Errorf("%v failed match against %v!", s, test.pattern)
			}
		}
		for _, s := range test.bad {
			if match(s, test.pattern) {
				t.Errorf("%v did not fail match against %v!", s, test.pattern)
			}
		}
	}

}
