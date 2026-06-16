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

// VxlanZoneOperations defines the interface for SDN VXLAN zone operations.
type VxlanZoneOperations interface {
	// Create creates a new VXLAN zone.
	Create(ctx context.Context, inputs VxlanZoneInputs) error

	// Get retrieves an existing VXLAN zone by zone name.
	Get(ctx context.Context, name string) (*VxlanZoneOutputs, error)

	// Update updates an existing VXLAN zone.
	Update(ctx context.Context, name string, inputs VxlanZoneInputs, oldOutputs VxlanZoneOutputs) error

	// Delete deletes an existing VXLAN zone by zone name.
	Delete(ctx context.Context, name string) error
}

// VxlanZoneInputs represents the input properties for the SDN VXLAN zone resource.
type VxlanZoneInputs struct {
	Name       string   `pulumi:"name"                provider:"replaceOnChanges"`
	Fabric     *string  `pulumi:"fabric,optional"`
	Peers      []string `pulumi:"peers,optional"`
	MTU        *int     `pulumi:"mtu,optional"`
	VXLANPort  *int     `pulumi:"vxlanPort,optional"`
	Nodes      []string `pulumi:"nodes,optional"`
	DNS        *string  `pulumi:"dns,optional"`
	DNSZone    *string  `pulumi:"dnsZone,optional"`
	ReverseDNS *string  `pulumi:"reverseDns,optional"`
	IPAM       *string  `pulumi:"ipam"`
}

// Annotate adds descriptions to the input properties.
func (inputs *VxlanZoneInputs) Annotate(a infer.Annotator) {
	a.Describe(&inputs.Name, "The unique VXLAN zone name.")
	a.Describe(&inputs.Fabric, "SDN fabric identifier used to connect this VXLAN zone to an EVPN fabric.")
	a.Describe(
		&inputs.Peers,
		"A list of peer IP addresses in the VXLAN underlay network. "+
			"Note: peers are not returned by the Proxmox API on read and will not be refreshed by `pulumi refresh`.",
	)
	a.Describe(&inputs.MTU, "MTU for the VXLAN zone. If unset, Proxmox uses its automatic/default MTU.")
	a.Describe(&inputs.VXLANPort, "UDP destination port used for VXLAN encapsulation.")
	a.SetDefault(&inputs.VXLANPort, 4789)
	a.Describe(&inputs.Nodes, "Cluster nodes where this zone is active.")
	a.Describe(&inputs.DNS, "DNS plugin ID associated with this zone.")
	a.Describe(&inputs.DNSZone, "DNS zone/domain name used for host registrations.")
	a.Describe(&inputs.ReverseDNS, "Reverse DNS plugin ID associated with this zone.")
	a.Describe(&inputs.IPAM, "IPAM plugin ID associated with this zone.")
}

// VxlanZoneOutputs represents the output properties for the SDN VXLAN zone resource.
type VxlanZoneOutputs struct {
	VxlanZoneInputs
}

// VxlanZoneAPIResource is the API-level payload for create/update requests and reads.
type VxlanZoneAPIResource struct {
	Zone       string                   `json:"zone,omitempty"`
	Type       string                   `json:"type,omitempty"`
	Fabric     *string                  `json:"fabric,omitempty"`
	Peers      utils.CommaSeparatedList `json:"peers,omitempty"`
	MTU        *int                     `json:"mtu,omitempty"`
	VXLANPort  *int                     `json:"vxlan-port,omitempty"`
	Nodes      utils.CommaSeparatedList `json:"nodes,omitempty"`
	DNS        *string                  `json:"dns,omitempty"`
	DNSZone    *string                  `json:"dnszone,omitempty"`
	ReverseDNS *string                  `json:"reversedns,omitempty"`
	IPAM       *string                  `json:"ipam,omitempty"`
	Delete     []string                 `json:"delete,omitempty"`
}
