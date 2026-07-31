/*
	Copyright 2025, Pulumi Corporation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package proxmox provides interfaces and domain types for interacting with Proxmox VE.
package proxmox

import (
	"reflect"

	p "github.com/pulumi/pulumi-go-provider"

	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

// FieldDiffer is implemented by input types that require custom diff logic.
// DiffFrom compares the receiver (input/new value) against state (prior value)
// and returns Pulumi property diffs keyed by property path. Nil pointer receivers
// are valid; implementations must handle them.
type FieldDiffer interface {
	DiffFrom(name string, state any) map[string]p.PropertyDiff
}

// FieldDiffValidator is implemented by fields that reject unsupported changes
// before their custom diff is returned.
type FieldDiffValidator interface {
	ValidateDiffFrom(state any) error
}

// PeerList is an order-sensitive list of VXLAN peers.
type PeerList []string

// DiffFrom returns an update when peer order or membership changes.
func (peers PeerList) DiffFrom(name string, state any) map[string]p.PropertyDiff {
	statePeers, _ := state.(PeerList)
	if !reflect.DeepEqual(peers, statePeers) {
		return map[string]p.PropertyDiff{name: {Kind: p.Update}}
	}
	return nil
}

// NodeList is an order-insensitive list of SDN nodes.
type NodeList []string

// DiffFrom returns an update when node membership changes.
func (nodes NodeList) DiffFrom(name string, state any) map[string]p.PropertyDiff {
	stateNodes, _ := state.(NodeList)
	if utils.StringSliceChanged(nodes, stateNodes) {
		return map[string]p.PropertyDiff{name: {Kind: p.Update}}
	}
	return nil
}
