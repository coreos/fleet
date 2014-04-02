package sign

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/gosshnew/ssh"
	gosshagent "github.com/coreos/fleet/third_party/code.google.com/p/gosshnew/ssh/agent"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/ssh"
)

const (
	DefaultAuthorizedKeysFile = "~/.ssh/authorized_keys"
)

var (
	ErrMalformedResp = errors.New("malformed signature response from agent client")
)

type SignatureSet struct {
	Tag   string
	Signs []*gossh.Signature
}

type SignatureCreator struct {
	// keyring is used to sign data
	keyring gosshagent.Agent
}

func NewSignatureCreator(keyring gosshagent.Agent) *SignatureCreator {
	return &SignatureCreator{keyring}
}

// NewSignatureCreatorFromSSHAgent return SignatureCreator which uses ssh-agent to sign
func NewSignatureCreatorFromSSHAgent() (*SignatureCreator, error) {
	keyring, err := ssh.SSHAgentClient()
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

	sigs := make([]*gossh.Signature, 0)

	keys, err := sc.keyring.List()
	if err != nil {
		return nil, err
	}

	for _, k := range keys {
		sig, err := sc.keyring.Sign(k, data)
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

func NewSignatureVerifier() *SignatureVerifier {
	return &SignatureVerifier{}
}

// NewSignatureVerifierFromSSHAgent return SignatureVerifier which uses ssh-agent to verify
func NewSignatureVerifierFromSSHAgent() (*SignatureVerifier, error) {
	keyring, err := ssh.SSHAgentClient()
	if err != nil {
		return nil, err
	}
	return NewSignatureVerifierFromKeyring(keyring)
}

// NewSignatureVerifierFromKeyring return SignatureVerifier which uses public keys fetched from keyring to verify
func NewSignatureVerifierFromKeyring(keyring gosshagent.Agent) (*SignatureVerifier, error) {
	keys, err := keyring.List()
	if err != nil {
		return nil, err
	}

	pubkeys := make([]gossh.PublicKey, len(keys))

	for i, k := range keys {
		pubkeys[i] = k
	}

	return &SignatureVerifier{pubkeys}, nil
}

// NewSignatureVerifierFromAuthorizedKeysFile return SignatureVerifier which uses authorized key file to verify
func NewSignatureVerifierFromAuthorizedKeysFile(filepath string) (*SignatureVerifier, error) {
	out, err := ioutil.ReadFile(pkg.ParseFilepath(filepath))
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
		// Manually coerce the keys provided by the agent to PublicKeys
		// since gosshnew does not want to do it for us.
		key, err := gossh.ParsePublicKey(authKey.Marshal())
		if err != nil {
			log.V(1).Infof("Unable to use SSH key: %v", err)
			continue
		}

		for _, sign := range s.Signs {
			if err := key.Verify(data, sign); err == nil {
				return true, nil
			}
		}
	}

	return false, nil
}

func parseAuthorizedKeys(in []byte) ([]gossh.PublicKey, error) {
	pubkeys := make([]gossh.PublicKey, 0)
	for len(in) > 0 {
		pubkey, _, _, rest, err := gossh.ParseAuthorizedKey(in)
		if err != nil {
			return nil, err
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

func parseString(in []byte) (out, rest []byte, ok bool) {
	if len(in) < 4 {
		return
	}
	// First 4-byte is the length of the field
	length := binary.BigEndian.Uint32(in)
	if uint32(len(in)) < length+4 {
		return
	}
	return in[4 : length+4], in[length+4:], true
}
