package sign

import (
	"bytes"
	"testing"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/gosshnew/ssh"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

func TestSignJobPayload(t *testing.T) {
	c, _ := initSign(t)
	payload := job.NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))

	data, err := marshal(payload)
	if err != nil {
		t.Fatal("marshal error:", err)
	}

	expectedSig, err := c.keyring.Sign(testPublicKeys["rsa"], data)
	if err != nil {
		t.Fatal("sign error:", err)
	}

	s, err := c.SignPayload(payload)
	if err != nil {
		t.Fatal("sign payload error:", err)
	}
	if s.Tag != TagForPayload("echo.service") {
		t.Fatal("sign tag error:", err)
	}

	if len(s.Signs) != 1 {
		t.Fatal("expect 1 signature instead of", len(s.Signs))
	}
	if bytes.Compare(s.Signs[0].Blob, expectedSig.Blob) != 0 {
		t.Fatal("wrong signature")
	}
}

func TestVerifyJobPayload(t *testing.T) {
	c, v := initSign(t)
	payload := job.NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))

	data, err := marshal(payload)
	if err != nil {
		t.Fatal("marshal error:", err)
	}

	v.pubkeys = append(v.pubkeys, testPublicKeys["rsa"])
	signature, err := c.keyring.Sign(testPublicKeys["rsa"], data)
	if err != nil {
		t.Fatal("sign error:", err)
	}

	s := &SignatureSet{TagForPayload("echo.service"), []*gossh.Signature{signature}}

	ok, err := v.VerifyPayload(payload, s)
	if err != nil {
		t.Fatal("verify payload error:", err)
	}
	if !ok {
		t.Fatal("fail to verify payload")
	}

	s.Tag = ""
	ok, err = v.VerifyPayload(payload, s)
	if err == nil || ok == true {
		t.Fatal("should fail on payload verification")
	}

	ok, err = v.VerifyPayload(payload, nil)
	if err == nil || ok == true {
		t.Fatal("should fail on payload verification")
	}
}
