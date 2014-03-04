package sign

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh"

	"github.com/coreos/fleet/ssh"
)

const (
	privateKeyFile = "../fixtures/insecure_private_key"
	authorizedKeyFile = "../fixtures/authorized_keys"
	nonexistAuthorizedKeyFile = "../fixtures/nonexist_authorized_keys"
	signatureForGood = "903a528536371744b4f7f3720e531321f128e164254600dba3753e26aa0bd4d6f86cd9da2f4463aca90549427f26547df821ff403722825651abbdb5a674b9bab07ed89a0b89e249cf93341325dd243300dc72a168b0faf06d3e154efd7e42f24aba312407b658634cb89e2f37d19cb7341feba9aca591d09894da4b9d5fe2713f69408a8d7c3fe28fbe07e80b2b1617158b510aadb487e37baf33a2497ffeb2e2e4091ec1a025adc59acae1ea28ee41632806389ffefc47272ef37405cf1c30933e427b8996106df6ca4cd4e5fa8c8f27d4ef74b8a4632d687ef2ccb520015034c72573ed4c95d927b53732bde72641eeb438c8e8f9374d091ba8deb2bfd929"
	firstAuthorizedKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDMmq9hew++XUyYKDRIuq4K3VaVJUaE76LscrJ4Ov+UPJ0nTm0/VH2z0eOX9fQijmZ3c0/uMN03bdTfZG2w4TLYwxkgtIgch6nYG540oeKGHfcx3D/LYQ1isTwlLFelSAnDjaIsiLmxv0XHc4lojhLEtjf1OyHMf06snQscizYTmin29/7qSehf9WBEAPxdMuBGWYMi4yHnDn0cT4b7iowzZ+LQFjhZDthz2WTdSqofHbjPQSLGm65IotCJh8WRROKYPVLqnlZtQV7ntkzxsDSVpv5gsGMfZpuF1LGkQ89p/dCvpShoiklORMDA0Stm0wSemoKkwWvaYTbiyj1ZreXl\n"
	secondAuthorizedKey = "ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA6NF8iallvQVp22WDkTkyrtvp9eWW6A8YVr+kz4TjGYe7gHzIw+niNltGEFHzD8+v1I2YJ6oXevct1YeS0o9HZyN1Q9qgCgzUFtdOKLv6IedplqoPkcmF0aYet2PkEDo3MlTBckFXPITAMzF8dJSIFo9D8HfdOV0IAdx4O7PtixWKn5y2hMNG0zQPyUecp4pzC6kivAIhyfHilFR61RGL+GPXQ2MWZWFYbAGjyiYJnAmCP3NOTd0jMZEnDkbUvxhMmBYSdETk1rRgm+R4LOzFUGaHqHDLKLX+FIPKcF96hrucXzcWyLbIbEgE98OHlnVYCzRdK8jlqm8tehUc9c9WhQ==\n"
)

type stubKeyring struct {
	signer gossh.Signer
}

func newStubKeyring(pemBytes []byte) (*stubKeyring, error) {
	signer, err := gossh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, err
	}
	return &stubKeyring{signer}, err
}

func (k *stubKeyring) Key(i int) (gossh.PublicKey, error) {
	if i != 0 {
		return nil, ssh.ErrKeyOutofIndex
	}
	return k.signer.PublicKey(), nil
}

func (k *stubKeyring) Sign(i int, rand io.Reader, data []byte) ([]byte, error) {
	if i != 0 {
		return nil, ssh.ErrKeyOutofIndex
	}
	return k.signer.Sign(rand, data)
}

type badStubKeyring struct {
}

func newBadStubKeyring() (*badStubKeyring) {
	return &badStubKeyring{}
}

func (k *badStubKeyring) Key(i int) (gossh.PublicKey, error) {
	return nil, errors.New("")
}

func (k *badStubKeyring) Sign(i int, rand io.Reader, data []byte) ([]byte, error) {
	return nil, errors.New("")
}


func initSign(t *testing.T) (*SignatureCreator, *SignatureVerifier) {
	out, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		t.Fatal("Miss private key file:", err)
	}
	keyring, err := newStubKeyring(out)
	if err != nil {
		t.Fatal("Fail to add ssh private key:", err)
	}

	c := NewSignatureCreator(keyring)
	v, err := NewSignatureVerifierFromAuthorizedKeysFile(authorizedKeyFile)
	if err != nil {
		t.Fatal("Fail to read from authorized key file:", err)
	}

	return c, v
}

// TestNewSignatureVerifierFromFile tests initializing SignatureVerifier from file
func TestNewSignatureVerifierFromFile(t *testing.T) {
	v, err := NewSignatureVerifierFromAuthorizedKeysFile(authorizedKeyFile)
	if err != nil {
		t.Fatal("Fail to read from authorized key file:", err)
	}

	keys := v.pubkeys
	if bytes.Compare(gossh.MarshalAuthorizedKey(keys[0]), []byte(firstAuthorizedKey)) != 0 {
		t.Fatal("Wrong first authorized key")
	}
	if bytes.Compare(gossh.MarshalAuthorizedKey(keys[1]), []byte(secondAuthorizedKey)) != 0 {
		t.Fatal("Wrong second authorized key")
	}
}

// TestBadNewSignatureVerifierFromFile tests initializing SignatureVerifier from file incorrectly
func TestBadNewSignatureVerifierFromFile(t *testing.T) {
	_, err := NewSignatureVerifierFromAuthorizedKeysFile(nonexistAuthorizedKeyFile)
	if err == nil {
		t.Fatal("succeed to new signature verifier")
	}

	_, err = NewSignatureVerifierFromAuthorizedKeysFile(privateKeyFile)
	if err == nil {
		t.Fatal("succeed to new signature verifier")
	}
}

// TestNewSignatureVerifierFromKeyring tests initializing SignatureVerifier from keyring
func TestNewSignatureVerifierFromKeyring(t *testing.T) {
	c, _ := initSign(t)
	v, err := NewSignatureVerifierFromKeyring(c.keyring)
	if err != nil {
		t.Fatal("fail to new signature verifier")
	}
	if len(v.pubkeys) != 1 {
		t.Fatal("fail to get correct number of key")
	}
}

// TestBadNewSignatureVerifierFromKeyring tests initializing SignatureVerifier from keyring incorrectly
func TestBadNewSignatureVerifierFromKeyring(t *testing.T) {
	_, err := NewSignatureVerifierFromKeyring(newBadStubKeyring())
	if err == nil {
		t.Fatal("succeed to new signature verifier")
	}
}

// TestSign tests the creation of correct signature
func TestSign(t *testing.T) {
	c, _ := initSign(t)
	tag := "1"
	data := []byte("Good")

	sig, err := c.Sign(tag, data)
	if err != nil {
		t.Fatal("Fail to create signature:", err)
	}

	if sig.Tag != tag {
		t.Fatal("Expect tag %v instead of %v", tag, sig.Tag)
	}
	var signatureByte []byte
	fmt.Sscanf(signatureForGood, "%x", &signatureByte)
	if bytes.Compare(sig.Signs[0], signatureByte) != 0 {
		t.Fatal("Wrong signature for data 'Good'", string(sig.Signs[0]), signatureByte)
	}
}

// TestBadSign tests the incorrect creation of correct signature
func TestBadSign(t *testing.T) {
	c, _ := initSign(t)
	tag := "1"
	data := []byte("Good")

	c.keyring = newBadStubKeyring()
	sig, err := c.Sign(tag, nil)
	if sig != nil || err == nil {
		t.Fatal("Succeed to create signature")
	}

	c.keyring = nil
	sig, err = c.Sign(tag, data)
	if sig != nil || err == nil {
		t.Fatal("Succeed to create signature")
	}
}

// TestVerify tests the verification of correct signature
func TestVerify(t *testing.T) {
	_, v := initSign(t)
	tag := "1"
	badData := []byte("Bad")
	data := []byte("Good")
	sig := &SignatureSet{tag, make([][]byte, 1)}
	fmt.Sscanf(signatureForGood, "%x", &sig.Signs[0])

	ok, err := v.Verify(data, sig)
	if err != nil {
		t.Fatal("Fail to verify signature:", err)
	}
	if !ok {
		t.Fatal("Fail to verify signature is correct")
	}

	ok, err = v.Verify(badData, sig)
	if err != nil {
		t.Fatal("Fail to verify signature:", err)
	}
	if ok {
		t.Fatal("Fail to verify signature is incorrect")
	}
}

// TestBadVerify tests the incorrect verification of correct signature
func TestBadVerify(t *testing.T) {
	_, v := initSign(t)
	data := []byte("Good")

	v.pubkeys = nil
	ok, err := v.Verify(data, nil)
	if ok != false || err == nil {
		t.Fatal("Succeed to create signature")
	}
}

// TestParseFilepath tests parsing filepath
func TestParseFilepath(t *testing.T) {
	path := parseFilepath("~/")
	if path[0] != '/' {
		t.Fatal("fail to parse ~")
	}
}

// TestBadMarshal tests incorrect marshal
func TestBadMarshal(t *testing.T) {
	c := make(chan bool)
	_, err := marshal(c)
	if err == nil {
		t.Fatal("succeed to marshal channel")
	}
}
