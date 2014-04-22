package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"

	etcdErr "github.com/coreos/fleet/third_party/github.com/coreos/etcd/error"
	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/unit"
)

const (
	unitPrefix = "/unit/"
	// Legacy versions of fleet stored payloads instead of units
	payloadPrefix = "/payload/"
)

func (r *Registry) storeOrGetUnit(u unit.Unit) (err error) {
	key := hashedUnitPath(u.Hash())
	json, err := marshal(u)
	if err != nil {
		return err
	}

	_, err = r.etcd.Create(key, json, 0)
	// unit is already stored
	if err != nil && err.(*etcd.EtcdError).ErrorCode == etcdErr.EcodeNodeExist {
		// TODO(jonboulle): verify more here?
		err = nil
	}
	return
}

// getUnitFromLegacyPayload tries to extract a Unit from a legacy JobPayload of the given name
func (r *Registry) getUnitFromLegacyPayload(name string) (*unit.Unit, error) {
	key := path.Join(keyPrefix, payloadPrefix, name)
	resp, err := r.etcd.Get(key, true, true)

	if err != nil {
		return nil, err
	}

	var ljp LegacyJobPayload
	if err := unmarshal(resp.Node.Value, &ljp); err != nil {
		return nil, errors.New(fmt.Sprintf("Error unmarshaling LegacyJobPayload(%s): %v", name, err))
	}
	if ljp.Name != name {
		return nil, errors.New(fmt.Sprintf("Payload name in Registry (%s) does not match expected name (%s)", ljp.Name, name))
	}
	// After the unmarshaling, the LegacyPayload should contain a fully hydrated Unit
	return &ljp.Unit, nil
}

// getUnitByHash retrieves from the Registry the Unit associated with the given Hash
func (r *Registry) getUnitByHash(hash unit.Hash) *unit.Unit {
	key := hashedUnitPath(hash)
	resp, err := r.etcd.Get(key, false, true)
	if err != nil {
		return nil
	}
	var u unit.Unit
	if err := unmarshal(resp.Node.Value, &u); err != nil {
		log.Errorf("Error unmarshaling Unit(%s): %v", hash, err)
		return nil
	}
	return &u
}

func hashedUnitPath(hash unit.Hash) string {
	return path.Join(keyPrefix, unitPrefix, hash.String())
}

// LegacyJobPayload deals with the legacy concept of a "JobPayload" (deprecated by Units).
// The associated marshaling/unmarshaling methods deal with Payloads encoded in this legacy format.
type LegacyJobPayload struct {
	Name string
	Unit unit.Unit
}

func (ljp *LegacyJobPayload) MarshalJSON() ([]byte, error) {
	ufm := unitFileModel{
		Contents: getLegacyUnitContents(ljp.Unit),
		Raw:      ljp.Unit.String(),
	}
	jpm := legacyJobPayloadModel{Name: ljp.Name, Unit: ufm}
	return json.Marshal(jpm)
}

// getLegacyUnitContents serializes the contents of a unit file into an obsolete datastructure.
// This datastructure is lossy and only used to perform signature verifications on old fleet Jobs.
func getLegacyUnitContents(unit unit.Unit) map[string]map[string]string {
	coerced := make(map[string]map[string]string, len(unit.Contents))
	for section, options := range unit.Contents {
		coerced[section] = make(map[string]string)
		for key, values := range options {
			if len(values) == 0 {
				continue
			}
			coerced[section][key] = values[len(values)-1]
		}
	}
	return coerced
}

func (ljp *LegacyJobPayload) UnmarshalJSON(data []byte) error {
	var ljpm legacyJobPayloadModel
	err := json.Unmarshal(data, &ljpm)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to JSON-deserialize object: %s", err))
	}

	if len(ljpm.Unit.Raw) > 0 {
		ljp.Unit = *unit.NewUnit(ljpm.Unit.Raw)
	} else {
		ljp.Unit = *unit.NewUnitFromLegacyContents(ljpm.Unit.Contents)
	}
	ljp.Name = ljpm.Name

	return nil
}

// legacyJobPayloadModel is an abstraction to deal with serialized LegacyJobPayloads
type legacyJobPayloadModel struct {
	Name string
	Unit unitFileModel
}

// unitFileModel is an abstraction to deal with serialized LegacyJobPayloads
type unitFileModel struct {
	Contents map[string]map[string]string
	Raw      string
}
