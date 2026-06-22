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
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

// VnetOperations defines the interface for SDN VNet resource operations.
type VnetOperations interface {
	// Create creates a new VNet.
	Create(ctx context.Context, inputs VnetInputs) error

	// Get retrieves an existing VNet by its name.
	Get(ctx context.Context, vnet string) (*VnetOutputs, error)

	// Update updates an existing VNet.
	Update(ctx context.Context, vnet string, inputs VnetInputs, oldOutputs VnetOutputs) error

	// Delete deletes an existing VNet by its name.
	Delete(ctx context.Context, vnet string) error
}

// VnetInputs represents the input properties for the SDN VNet resource.
type VnetInputs struct {
	Vnet         string `pulumi:"vnet"                provider:"replaceOnChanges"`
	Zone         string `pulumi:"zone"`
	Tag          int    `pulumi:"tag"`
	Alias        string `pulumi:"alias,optional"`
	Vlanaware    bool   `pulumi:"vlanaware,optional"`
	IsolatePorts bool   `pulumi:"isolatePorts,optional"`
}

// Annotate adds descriptions to the input properties for documentation and schema generation.
func (inputs *VnetInputs) Annotate(a infer.Annotator) {
	a.Describe(
		&inputs.Vnet,
		"The VNet identifier/name (max 8 alphanumeric characters). This is the bridge name VMs reference.",
	)
	a.Describe(&inputs.Zone, "The SDN zone this VNet belongs to (e.g. \"ringfence\").")
	a.Describe(&inputs.Tag, "The unique VNI/VLAN tag for this VNet (convention: 10000 + pool number).")
	a.Describe(&inputs.Alias, "An optional descriptive alias for the VNet.")
	a.Describe(&inputs.Vlanaware, "If true, allow guest VLANs to pass through (trunk) this VNet.")
	a.Describe(
		&inputs.IsolatePorts,
		"If true, sets the isolated property for all interfaces on the bridge of this VNet.",
	)
}

// VnetOutputs represents the output properties for the SDN VNet resource.
type VnetOutputs struct {
	VnetInputs
	State  string `pulumi:"state,optional"`
	Digest string `pulumi:"digest,optional"`
}

// Annotate adds descriptions to the output-only properties (read-only metadata).
func (outputs *VnetOutputs) Annotate(a infer.Annotator) {
	outputs.VnetInputs.Annotate(a)
	a.Describe(
		&outputs.State,
		"Read-only pending-apply state of the VNet (\"new\", \"changed\", or \"deleted\"); "+
			"empty once the config has been applied.",
	)
	a.Describe(
		&outputs.Digest,
		"Read-only digest of the VNet configuration section, used by Proxmox for change detection.",
	)
}

// VnetAPIObject is the API-level representation of a VNet used for both
// reads (GET) and writes (POST/PUT). Read-only fields (Type, State, Digest)
// are populated on GET and ignored on POST/PUT. The write-only field (Delete)
// is sent on PUT to remove optional fields and is absent in GET responses.
type VnetAPIObject struct {
	Vnet         string         `json:"vnet,omitempty"`
	Zone         string         `json:"zone,omitempty"`
	Tag          int            `json:"tag,omitempty"`
	Alias        string         `json:"alias,omitempty"`
	Type         string         `json:"type,omitempty"`
	Vlanaware    *utils.IntBool `json:"vlanaware,omitempty"`
	IsolatePorts *utils.IntBool `json:"isolate-ports,omitempty"`
	State        string         `json:"state,omitempty"`
	Digest       string         `json:"digest,omitempty"`
	Delete       []string       `json:"delete,omitempty"`
}
