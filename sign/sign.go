package sign

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os/user"
	"strings"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh"

	"github.com/coreos/fleet/ssh"
)

const (
	DefaultAuthorizedKeyFile = "~/.ssh/authorized_keys"
)

type SignatureSet struct {
	Tag string
	Signs [][]byte
}

type SignatureCreator struct {
	// keyring is used to sign data
	keyring gossh.ClientKeyring
}

func NewSignatureCreator(keyring gossh.ClientKeyring) *SignatureCreator {
	return &SignatureCreator{keyring}
}

// NewSignatureCreatorFromSSHAgent return SignatureCreator which uses ssh-agent to sign
func NewSignatureCreatorFromSSHAgent() (*SignatureCreator, error) {
	keyring, err := ssh.NewSSHAgentKeyring()
	if err != nil {
		return nil, err
	}
	return &SignatureCreator{keyring}, nil
}

// Sign generates signature for the data labled by tag
func (sc *SignatureCreator) Sign(tag string, data []byte) (*SignatureSet, error) {
	if sc.keyring == nil {
		return nil, errors.New("signature creator is uninitialized")
	}

	sigs := make([][]byte, 0)
	// Generate all possible signatures
	for i := 0; ; i++ {
		sig, err := sc.keyring.Sign(i, nil, data)
		if err == ssh.ErrKeyOutofIndex {
			break
		}
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, sig)
	}

	return &SignatureSet{tag, sigs}, nil
}

type SignatureVerifier struct {
	// keys is used to verify signing, created when needed
	pubkeys []gossh.PublicKey
}

// NewSignatureVerifierFromSSHAgent return SignatureVerifier which uses ssh-agent to verify
func NewSignatureVerifierFromSSHAgent() (*SignatureVerifier, error) {
	keyring, err := ssh.NewSSHAgentKeyring()
	if err != nil {
		return nil, err
	}
	return NewSignatureVerifierFromKeyring(keyring)
}

// NewSignatureVerifierFromKeyring return SignatureVerifier which uses public keys fetched from keyring to verify
func NewSignatureVerifierFromKeyring(keyring gossh.ClientKeyring) (*SignatureVerifier, error) {

	pubkeys := make([]gossh.PublicKey, 0)
	for i := 0; ; i++ {
		pubkey, err := keyring.Key(i)
		if err == ssh.ErrKeyOutofIndex {
			break
		}
		if err != nil {
			return nil, err
		}
		pubkeys = append(pubkeys, pubkey)
	}

	return &SignatureVerifier{pubkeys}, nil
}

// NewSignatureVerifierFromAuthorizedKeyFile return SignatureVerifier which uses authorized key file to verify
func NewSignatureVerifierFromAuthorizedKeyFile(filepath string) (*SignatureVerifier, error) {
	out, err := ioutil.ReadFile(parseFilepath(filepath))
	if err != nil {
		return nil, err
	}

	pubkeys, err := parseAuthorizedKeys(out)
	if err != nil {
		return nil, err
	}

	return &SignatureVerifier{pubkeys}, nil
}

// Verify verifies whether or not data fits the signature
func (sv *SignatureVerifier) Verify(data []byte, s *SignatureSet) (bool, error) {
	if sv.pubkeys == nil {
		return false, errors.New("signature verifier is uninitialized")
	}

	// Enumerate all pairs to verify signatures
	for _, authKey := range sv.pubkeys {
		for _, sign := range s.Signs {
			if authKey.Verify(data, sign) {
				return true, nil
			}
		}
	}

	return false, nil
}

// get file path considering user home directory
func parseFilepath(path string) string {
	if strings.Index(path, "~") != 0 {
		return path
	}

	usr, err := user.Current()
	if err == nil {
		path = strings.Replace(path, "~", usr.HomeDir, 1)
	}

	return path
}

func parseAuthorizedKeys(in []byte) ([]gossh.PublicKey, error) {
	pubkeys := make([]gossh.PublicKey, 0)
	for len(in) > 0 {
		pubkey, _, _, rest, ok := gossh.ParseAuthorizedKey(in)
		if !ok {
			return nil, errors.New("fail to parse authorized key file")
		}
		in = rest

		pubkeys = append(pubkeys, pubkey)
	}

	return pubkeys, nil
}

func marshal(obj interface{}) ([]byte, error) {
	encoded, err := json.Marshal(obj)
	if err == nil {
		return encoded, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Unable to JSON-serialize object: %s", err))
	}
}
