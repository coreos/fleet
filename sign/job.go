package sign

import (
	"errors"
	"path"

	"github.com/coreos/fleet/job"
)

const (
	payloadPrefix = "/payload/"
)

// TagForPayload returns tag used for payload
func TagForPayload(name string) string {
	return path.Join(payloadPrefix, name)
}

// SignPayload signs the payload
func (sc *SignatureCreator) SignPayload(jp *job.JobPayload) (*SignatureSet, error) {
	tag := path.Join(payloadPrefix, jp.Name)
	data, _ := marshal(jp)
	return sc.Sign(tag, data)
}

// VerifyPayload verifies the payload using signature
func (sc *SignatureVerifier) VerifyPayload(jp *job.JobPayload, s *SignatureSet) (bool, error) {
	if s == nil {
		return false, errors.New("signature to verify is nil")
	}

	tag := path.Join(payloadPrefix, jp.Name)
	if tag != s.Tag {
		return false, errors.New("unmatched payload and signature")
	}

	data, _ := marshal(jp)
	return sc.Verify(data, s)
}
