package registry

import (
	"encoding/json"
	"fmt"
	"path"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/unit"
)

const (
	unitPrefix = "/unit/"
)

func (r *EtcdRegistry) storeOrGetUnit(u unit.Unit) (err error) {
	um := unitModel{
		Raw: u.String(),
	}

	json, err := marshal(um)
	if err != nil {
		return err
	}

	req := etcd.Create{
		Key:   r.hashedUnitPath(u.Hash()),
		Value: json,
	}
	_, err = r.etcd.Do(&req)
	// unit is already stored
	if err != nil && isNodeExist(err) {
		// TODO(jonboulle): verify more here?
		err = nil
	}
	return
}

// getUnitByHash retrieves from the Registry the Unit associated with the given Hash
func (r *EtcdRegistry) getUnitByHash(hash unit.Hash) *unit.Unit {
	req := etcd.Get{
		Key:       r.hashedUnitPath(hash),
		Recursive: true,
	}
	resp, err := r.etcd.Do(&req)
	if err != nil {
		if isKeyNotFound(err) {
			err = nil
		}
		return nil
	}
	var um unitModel
	if err := unmarshal(resp.Node.Value, &um); err != nil {
		log.Errorf("error unmarshaling Unit(%s): %v", hash, err)
		return nil
	}

	u, err := unit.NewUnit(um.Raw)
	if err != nil {
		log.Errorf("error parsing Unit(%s): %v", hash, err)
		return nil
	}

	return u
}

func (r *EtcdRegistry) hashedUnitPath(hash unit.Hash) string {
	return path.Join(r.keyPrefix, unitPrefix, hash.String())
}

type unitModel struct {
	Raw string
}
