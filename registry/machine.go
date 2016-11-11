// Copyright 2014 The fleet Authors
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
	"strings"
	"time"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/coreos/fleet/machine"
	"path"
)

const (
	machinePrefix = "machines"
)

func (r *EtcdRegistry) Machines() (machines []machine.MachineState, err error) {
	key := r.prefixed(machinePrefix)
	opts := &etcd.GetOptions{
		Sort:      true,
		Recursive: true,
	}

	resp, err := r.kAPI.Get(context.Background(), key, opts)
	if err != nil {
		if isEtcdError(err, etcd.ErrorCodeKeyNotFound) {
			err = nil
		}
		return
	}

	for _, node := range resp.Node.Nodes {
		var mach machine.MachineState
		mach, err = readMachineState(node)
		if err != nil {
			return
		}

		if mach.ID != "" {
			machines = append(machines, mach)
		}
	}

	return
}

func (r *EtcdRegistry) CreateMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	val, err := marshal(ms)
	if err != nil {
		return uint64(0), err
	}

	key := r.prefixed(machinePrefix, ms.ID, "object")
	opts := &etcd.SetOptions{
		PrevExist: etcd.PrevNoExist,
		TTL:       ttl,
	}
	resp, err := r.kAPI.Set(context.Background(), key, val, opts)
	if err != nil {
		return uint64(0), err
	}

	return resp.Node.ModifiedIndex, nil
}

func (r *EtcdRegistry) SetMachineState(ms machine.MachineState, ttl time.Duration) (uint64, error) {
	val, err := marshal(ms)
	if err != nil {
		return uint64(0), err
	}

	key := r.prefixed(machinePrefix, ms.ID, "object")
	opts := &etcd.SetOptions{
		PrevExist: etcd.PrevExist,
		TTL:       ttl,
	}
	resp, err := r.kAPI.Set(context.Background(), key, val, opts)
	if err == nil {
		return resp.Node.ModifiedIndex, nil
	}

	// If state was not present, explicitly create it so the other members
	// in the cluster know this is a new member
	opts.PrevExist = etcd.PrevNoExist

	resp, err = r.kAPI.Set(context.Background(), key, val, opts)
	if err != nil {
		return uint64(0), err
	}

	return resp.Node.ModifiedIndex, nil
}

func (r *EtcdRegistry) MachineState(machID string) (machine.MachineState, error) {
	key := path.Join(r.keyPrefix, machinePrefix, machID)
	opts := &etcd.GetOptions{
		Recursive: true,
		Sort:      true,
	}

	resp, err := r.kAPI.Get(context.Background(), key, opts)
	if err != nil {
		return machine.MachineState{}, err
	}

	return readMachineState(resp.Node)
}

func (r *EtcdRegistry) SetMachineMetadata(machID string, key string, value string) error {
	key = path.Join(r.keyPrefix, machinePrefix, machID, "metadata", key)
	opts := &etcd.SetOptions{}
	_, err := r.kAPI.Set(context.Background(), key, value, opts)
	return err
}

func (r *EtcdRegistry) DeleteMachineMetadata(machID string, key string) error {
	// Deleting a key sets its value to "" to allow for intelligent merging
	// between the machine-defined metadata and the dynamic metadata.
	// See mergeMetadata for more detail.
	return r.SetMachineMetadata(machID, key, "")
}

func (r *EtcdRegistry) RemoveMachineState(machID string) error {
	key := r.prefixed(machinePrefix, machID, "object")
	_, err := r.kAPI.Delete(context.Background(), key, nil)
	if isEtcdError(err, etcd.ErrorCodeKeyNotFound) {
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

// readMachineState reads machine state from an etcd node
func readMachineState(node *etcd.Node) (mach machine.MachineState, err error) {
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

	mach.Metadata = mergeMetadata(mach.Metadata, metadata)
	return
}
