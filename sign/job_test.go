package sign

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

const (
	echoPayloadSignature = "d46d80ac4b397e07be2f4bc1b226422745840539e69420640db1f2b40b1e944193bd5a068fcbe296de57f2d611cd87369949fe22d50bd673f8e6a696e3fb97f8d35cb6e684c424717fbba87a6cb7954fb089cd5970c43c69516d743867bfa394f1f05991a51f7392e26cdd6525de434ec273cd9e0878a952c9a79a4be75063ea1d4c3730a669138f748347e97c344270a0cb357828dbe4e27d0988d8e83ae47c28dd600bd5fd404b09c662fa4b14fa903d93fecffbc916892b9a7d258f6a4ac075a0bf5f9509f860dbc8306246535d435aabdc5dd64e2a41f1e43926cbcb2d486dbfa1713e60b7055224677dd6487a0f3f5f91db2c8ca15b2f8787059476bdc6"
)

func TestSignJobPayload(t *testing.T) {
	c, _ := initSign(t)
	payload := job.NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))

	s, err := c.SignPayload(payload)
	if err != nil {
		t.Fatal("sign payload error:", err)
	}
	if s.Tag != TagForPayload("echo.service") {
		t.Fatal("sign tag error:", err)
	}

	var sign []byte
	fmt.Sscanf(echoPayloadSignature, "%x", &sign)
	if len(s.Signs) != 1 {
		t.Fatal("expect 1 signature instead of", len(s.Signs))
	}
	if bytes.Compare(s.Signs[0], sign) != 0 {
		t.Fatal("wrong signature")
	}
}

func TestVerifyJobPayload(t *testing.T) {
	_, v := initSign(t)
	payload := job.NewJobPayload("echo.service", *unit.NewSystemdUnitFile("Echo"))
	s := &SignatureSet{TagForPayload("echo.service"), make([][]byte, 1)}
	fmt.Sscanf(echoPayloadSignature, "%x", &s.Signs[0])

	ok, err := v.VerifyPayload(payload, s)
	if err != nil {
		t.Fatal("veirfy payload error:", err)
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
