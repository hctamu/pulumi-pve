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
	"fmt"
	"regexp"

	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// SdnVnetOperations defines the interface for SDN VNet resource operations.
type SdnVnetOperations interface {
	// Create creates a new VNet.
	Create(ctx context.Context, inputs SdnVnetInputs) error

	// Get retrieves an existing VNet by its name.
	Get(ctx context.Context, vnet string) (*SdnVnetOutputs, error)

	// Update updates an existing VNet.
	Update(ctx context.Context, vnet string, inputs SdnVnetInputs, oldOutputs SdnVnetOutputs) error

	// Delete deletes an existing VNet by its name.
	Delete(ctx context.Context, vnet string) error
}

// vnetNameRegexp enforces the Proxmox VNet naming rule: start with a letter, alphanumeric only.
var vnetNameRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`)

// ValidateVnetName checks the Proxmox 8-character alphanumeric limit for VNet names.
func ValidateVnetName(ctx context.Context, name string) error {
	if name == "" || len(name) > 8 {
		err := fmt.Errorf("invalid vnet name %q: must be 1-8 characters", name)
		p.GetLogger(ctx).Error(err.Error())
		return err
	}
	if !vnetNameRegexp.MatchString(name) {
		err := fmt.Errorf("invalid vnet name %q: must be alphanumeric and start with a letter", name)
		p.GetLogger(ctx).Error(err.Error())
		return err
	}
	return nil
}

// SdnVnetInputs represents the input properties for the SDN VNet resource.
type SdnVnetInputs struct {
	Vnet         string `pulumi:"vnet"                provider:"replaceOnChanges"`
	Zone         string `pulumi:"zone"`
	Tag          int    `pulumi:"tag"`
	Alias        string `pulumi:"alias,optional"`
	Vlanaware    bool   `pulumi:"vlanaware,optional"`
	IsolatePorts bool   `pulumi:"isolatePorts,optional"`
}

// Annotate adds descriptions to the input properties for documentation and schema generation.
func (inputs *SdnVnetInputs) Annotate(a infer.Annotator) {
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

// SdnVnetOutputs represents the output properties for the SDN VNet resource.
type SdnVnetOutputs struct {
	SdnVnetInputs
	State  string `pulumi:"state,optional"`
	Digest string `pulumi:"digest,optional"`
}

// Annotate adds descriptions to the output-only properties (read-only metadata).
func (outputs *SdnVnetOutputs) Annotate(a infer.Annotator) {
	outputs.SdnVnetInputs.Annotate(a)
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

// SdnVnetAPIResource is the write payload for POST/PUT (API level only).
type SdnVnetAPIResource struct {
	Vnet         string         `json:"vnet,omitempty"`
	Zone         string         `json:"zone,omitempty"`
	Tag          int            `json:"tag,omitempty"`
	Alias        string         `json:"alias,omitempty"`
	Vlanaware    *api.IntOrBool `json:"vlanaware,omitempty"`
	IsolatePorts *api.IntOrBool `json:"isolate-ports,omitempty"`
	Delete       []string       `json:"delete,omitempty"`
}

// SdnVnetResponse is the read payload returned by GET (API level only).
type SdnVnetResponse struct {
	Vnet         string        `json:"vnet"`
	Zone         string        `json:"zone"`
	Tag          int           `json:"tag"`
	Alias        string        `json:"alias"`
	Type         string        `json:"type"`
	Vlanaware    api.IntOrBool `json:"vlanaware"`
	IsolatePorts api.IntOrBool `json:"isolate-ports"`
	State        string        `json:"state"`
	Digest       string        `json:"digest"`
}
