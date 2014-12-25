// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"path"
	"strings"
	"time"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/machine"
)

const (
	machinePrefix = "machines"
)

func (r *EtcdRegistry) Machines() (machines []machine.MachineState, err error) {
	req := etcd.Get{
		Key:       path.Join(r.keyPrefix, machinePrefix),
		Sorted:    true,
		Recursive: true,
	}

	resp, err := r.etcd.Do(&req)
	if err != nil {
		if etcd.IsKeyNotFound(err) {
			err = nil
		}
		return
	}

	for _, node := range resp.Node.Nodes {
		var mach machine.MachineState
		var metadata map[string]string

		for _, obj := range node.Nodes {
			if strings.HasSuffix(obj.Key, "/object") {
				err = unmarshal(obj.Value, &mach)
				if err != nil {
					return
				}
			} else if strings.HasSuffix(obj.Key, "/metadata") {
				// Load metadata into a separate map to avoid stepping on it when deserializing the object key
				metadata = make(map[string]string, len(obj.Nodes))
				for _, mdnode := range obj.Nodes {
					metadata[path.Base(mdnode.Key)] = mdnode.Value
				}
			}
		}

		if mach.ID != "" {
			mach.Metadata = mergeMetadata(mach.Metadata, metadata)
			machines = append(machines, mach)
		}
	}

	return
}

func (r *EtcdRegistry) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	json, err := marshal(ms)
	if err != nil {
		return uint64(0), err
	}

	update := etcd.Update{
		Key:   path.Join(r.keyPrefix, machinePrefix, ms.ID, "object"),
		Value: json,
		TTL:   ttl,
	}

	resp, err := r.etcd.Do(&update)
	if err == nil {
		return resp.Node.ModifiedIndex, nil
	}

	// If state was not present, explicitly create it so the other members
	// in the cluster know this is a new member
	create := etcd.Create{
		Key:   path.Join(r.keyPrefix, machinePrefix, ms.ID, "object"),
		Value: json,
		TTL:   ttl,
	}

	resp, err = r.etcd.Do(&create)
	if err != nil {
		return uint64(0), err
	}

	return resp.Node.ModifiedIndex, nil
}

func (r *EtcdRegistry) SetMachineMetadata(machID string, key string, value string) error {
	update := etcd.Set{
		Key:   path.Join(r.keyPrefix, machinePrefix, machID, "metadata", key),
		Value: value,
	}

	_, err := r.etcd.Do(&update)
	return err
}

func (r *EtcdRegistry) DeleteMachineMetadata(machID string, key string) error {
	// Deleting a key sets its value to "" to allow for intelligent merging
	// between the machine-defined metadata and the dynamic metadata.
	// See mergeMetadata for more detail.
	return r.SetMachineMetadata(machID, key, "")
}

func (r *EtcdRegistry) RemoveMachineState(machID string) error {
	req := etcd.Delete{
		Key: path.Join(r.keyPrefix, machinePrefix, machID, "object"),
	}
	_, err := r.etcd.Do(&req)
	if etcd.IsKeyNotFound(err) {
		err = nil
	}
	return err
}

// mergeMetadata merges the machine-set metadata with the dynamic metadata to better facilitate
// machines leaving and rejoining a cluster.
// Merging metadata uses the following rules:
// - Any keys that are only in one collection are added as-is
// - Any keys that exist in both, the dynamic value takes precence
// - Any keys that have a zero-value string in the dynamic metadata are considered deleted
//   and are not included in the final collection
func mergeMetadata(machineMetadata, dynamicMetadata map[string]string) map[string]string {
	if dynamicMetadata == nil {
		return machineMetadata
	}
	finalMetadata := make(map[string]string, len(dynamicMetadata))
	for k, v := range machineMetadata {
		finalMetadata[k] = v
	}
	for k, v := range dynamicMetadata {
		if v == "" {
			delete(finalMetadata, k)
		} else {
			finalMetadata[k] = v
		}
	}
	return finalMetadata
}
