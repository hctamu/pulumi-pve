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

package adapters

import (
	"context"
	"errors"
	"fmt"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

const (
	sdnZoneResourcePath = "/cluster/sdn/zones"
	vxlanType           = "vxlan"
)

// Ensure VxlanZoneAdapter implements the VxlanZoneOperations interface.
var _ proxmox.VxlanZoneOperations = (*VxlanZoneAdapter)(nil)

// VxlanZoneAdapter implements proxmox.VxlanZoneOperations using a ProxmoxClient.
type VxlanZoneAdapter struct {
	client proxmox.Client
}

// NewVxlanZoneAdapter creates a new VxlanZoneAdapter wrapping the given ProxmoxClient.
func NewVxlanZoneAdapter(client proxmox.Client) *VxlanZoneAdapter {
	return &VxlanZoneAdapter{client: client}
}

// Create creates a new VXLAN zone.
func (adapter *VxlanZoneAdapter) Create(ctx context.Context, inputs proxmox.VxlanZoneInputs) error {
	apiResource := proxmox.VxlanZoneAPIResource{
		Zone:       inputs.Name,
		Type:       vxlanType,
		Fabric:     inputs.Fabric,
		Peers:      utils.CommaSeparatedList(inputs.Peers),
		MTU:        inputs.MTU,
		VXLANPort:  inputs.VXLANPort,
		Nodes:      utils.CommaSeparatedList(inputs.Nodes),
		DNS:        inputs.DNS,
		DNSZone:    inputs.DNSZone,
		ReverseDNS: inputs.ReverseDNS,
		IPAM:       inputs.IPAM,
	}

	if err := adapter.client.Post(ctx, sdnZoneResourcePath, apiResource, nil); err != nil {
		return fmt.Errorf("failed to create SDN VXLAN zone: %w", err)
	}

	return nil
}

// Get retrieves an existing VXLAN zone by zone name.
func (adapter *VxlanZoneAdapter) Get(ctx context.Context, name string) (*proxmox.VxlanZoneOutputs, error) {
	var response *proxmox.VxlanZoneAPIResource

	if err := adapter.client.Get(ctx, fmt.Sprintf("%s/%s", sdnZoneResourcePath, name), &response); err != nil {
		return nil, fmt.Errorf("failed to get SDN VXLAN zone: %w", err)
	}
	if response == nil {
		return nil, errors.New("failed to get SDN VXLAN zone: empty response")
	}

	outputs := &proxmox.VxlanZoneOutputs{}
	outputs.Name = response.Zone
	outputs.Fabric = response.Fabric
	outputs.MTU = response.MTU
	outputs.VXLANPort = response.VXLANPort
	outputs.Nodes = []string(response.Nodes)
	outputs.DNS = response.DNS
	outputs.DNSZone = response.DNSZone
	outputs.ReverseDNS = response.ReverseDNS
	outputs.IPAM = response.IPAM

	return outputs, nil
}

// Update updates an existing VXLAN zone.
func (adapter *VxlanZoneAdapter) Update(
	ctx context.Context,
	name string,
	inputs proxmox.VxlanZoneInputs,
	oldOutputs proxmox.VxlanZoneOutputs,
) error {
	apiResource := proxmox.VxlanZoneAPIResource{}
	deleteFields := make([]string, 0)

	utils.SetOrDeletePtr(&deleteFields, &apiResource.DNS, "dns", inputs.DNS, oldOutputs.DNS)
	utils.SetOrDeletePtr(&deleteFields, &apiResource.Fabric, "fabric", inputs.Fabric, oldOutputs.Fabric)
	utils.SetOrDeletePtr(&deleteFields, &apiResource.DNSZone, "dnszone", inputs.DNSZone, oldOutputs.DNSZone)
	utils.SetOrDeletePtr(
		&deleteFields,
		&apiResource.ReverseDNS,
		"reversedns",
		inputs.ReverseDNS,
		oldOutputs.ReverseDNS,
	)
	utils.SetOrDeletePtr(&deleteFields, &apiResource.IPAM, "ipam", inputs.IPAM, oldOutputs.IPAM)

	utils.SetOrDeleteCSL(&deleteFields, &apiResource.Peers, "peers", inputs.Peers, oldOutputs.Peers)
	utils.SetOrDeleteCSL(&deleteFields, &apiResource.Nodes, "nodes", inputs.Nodes, oldOutputs.Nodes)
	utils.SetOrDeletePtr(&deleteFields, &apiResource.MTU, "mtu", inputs.MTU, oldOutputs.MTU)
	utils.SetOrDeletePtr(
		&deleteFields,
		&apiResource.VXLANPort,
		"vxlan-port",
		inputs.VXLANPort,
		oldOutputs.VXLANPort,
	)

	if len(deleteFields) > 0 {
		apiResource.Delete = deleteFields
	}

	if err := adapter.client.Put(ctx, fmt.Sprintf("%s/%s", sdnZoneResourcePath, name), apiResource, nil); err != nil {
		return fmt.Errorf("failed to update SDN VXLAN zone: %w", err)
	}

	return nil
}

// Delete deletes an existing VXLAN zone by zone name.
func (adapter *VxlanZoneAdapter) Delete(ctx context.Context, name string) error {
	if err := adapter.client.Delete(ctx, fmt.Sprintf("%s/%s", sdnZoneResourcePath, name), nil); err != nil {
		return fmt.Errorf("failed to delete SDN VXLAN zone: %w", err)
	}

	return nil
}
