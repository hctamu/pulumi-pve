/* Copyright 2025, Pulumi Corporation.

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

package proxmox

import (
	"fmt"
	"reflect"
	"regexp"

	p "github.com/pulumi/pulumi-go-provider"

	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

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

var (
	_ FieldDiffer = PeerList(nil)
	_ FieldDiffer = NodeList(nil)
)

// sdnNameRegexp enforces the Proxmox SDN identifier rule: start with a letter, alphanumeric only, max 8 chars.
var sdnNameRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]{0,7}$`)

// ValidateSDNName enforces the Proxmox SDN identifier rule used by VNet and zone names:
// start with a letter, alphanumeric only, 1-8 characters.
func ValidateSDNName(name string) error {
	if !sdnNameRegexp.MatchString(name) {
		return fmt.Errorf(
			"invalid name %q: must start with a letter, contain only letters and numbers, and be at most 8 characters",
			name,
		)
	}
	return nil
}
