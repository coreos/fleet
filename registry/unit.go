package registry

import (
	"encoding/json"
	"fmt"
	"path"

	goetcd "github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/unit"
)

const (
	unitPrefix = "/unit/"
	// Legacy versions of fleet stored payloads instead of units
	payloadPrefix = "/payload/"
)

func (r *EtcdRegistry) storeOrGetUnit(u unit.Unit) (err error) {
	key := r.hashedUnitPath(u.Hash())
	json, err := marshal(u)
	if err != nil {
		return err
	}

	log.V(3).Infof("Storing Unit(%s) in Registry: %s", u.Hash(), json)
	_, err = r.etcd.Create(key, json, 0)
	// unit is already stored
	if err != nil && err.(*goetcd.EtcdError).ErrorCode == etcd.EcodeNodeExist {
		log.V(2).Infof("Unit(%s) already exists in Registry", u.Hash())
		// TODO(jonboulle): verify more here?
		err = nil
	}
	return
}

// getUnitFromLegacyPayload tries to extract a Unit from a legacy JobPayload of the given name
func (r *EtcdRegistry) getUnitFromLegacyPayload(name string) (*unit.Unit, error) {
	key := path.Join(r.keyPrefix, payloadPrefix, name)
	resp, err := r.etcd.Get(key, true, true)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return nil, err
	}

	var ljp LegacyJobPayload
	if err := unmarshal(resp.Node.Value, &ljp); err != nil {
		return nil, fmt.Errorf("error unmarshaling LegacyJobPayload(%s): %v", name, err)
	}
	if ljp.Name != name {
		return nil, fmt.Errorf("payload name in Registry (%s) does not match expected name (%s)", ljp.Name, name)
	}
	// After the unmarshaling, the LegacyPayload should contain a fully hydrated Unit
	return &ljp.Unit, nil
}

// getUnitByHash retrieves from the Registry the Unit associated with the given Hash
func (r *EtcdRegistry) getUnitByHash(hash unit.Hash) *unit.Unit {
	key := r.hashedUnitPath(hash)
	resp, err := r.etcd.Get(key, false, true)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return nil
	}
	var u unit.Unit
	if err := unmarshal(resp.Node.Value, &u); err != nil {
		log.Errorf("error unmarshaling Unit(%s): %v", hash, err)
		return nil
	}
	return &u
}

func (r *EtcdRegistry) hashedUnitPath(hash unit.Hash) string {
	return path.Join(r.keyPrefix, unitPrefix, hash.String())
}

// LegacyJobPayload deals with the legacy concept of a "JobPayload" (deprecated by Units).
// The associated marshaling/unmarshaling methods deal with Payloads encoded in this legacy format.
type LegacyJobPayload struct {
	Name string
	Unit unit.Unit
}

func (ljp *LegacyJobPayload) UnmarshalJSON(data []byte) error {
	var ljpm legacyJobPayloadModel
	err := json.Unmarshal(data, &ljpm)
	if err != nil {
		return fmt.Errorf("unable to JSON-deserialize object: %s", err)
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
