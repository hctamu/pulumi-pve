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
	"fmt"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

const sdnVnetBasePath = "/cluster/sdn/vnets"

// Ensure SdnVnetAdapter implements the SdnVnetOperations interface.
var _ proxmox.SdnVnetOperations = (*SdnVnetAdapter)(nil)

// SdnVnetAdapter implements proxmox.SdnVnetOperations using a ProxmoxClient.
type SdnVnetAdapter struct {
	client proxmox.Client
}

// NewSdnVnetAdapter creates a new SdnVnetAdapter wrapping the given ProxmoxClient.
func NewSdnVnetAdapter(client proxmox.Client) *SdnVnetAdapter {
	return &SdnVnetAdapter{client: client}
}

// Create creates a new VNet.
func (adapter *SdnVnetAdapter) Create(ctx context.Context, inputs proxmox.SdnVnetInputs) error {
	apiResource := &proxmox.SdnVnetAPIResource{
		Vnet:  inputs.Vnet,
		Zone:  inputs.Zone,
		Tag:   inputs.Tag,
		Alias: inputs.Alias,
	}
	if err := adapter.client.Post(ctx, sdnVnetBasePath, apiResource, nil); err != nil {
		return fmt.Errorf("failed to create VNet: %w", err)
	}
	return nil
}

// Get retrieves an existing VNet by its name.
func (adapter *SdnVnetAdapter) Get(ctx context.Context, vnet string) (*proxmox.SdnVnetOutputs, error) {
	var response *proxmox.SdnVnetResponse
	url := fmt.Sprintf("%s/%s", sdnVnetBasePath, vnet)
	if err := adapter.client.Get(ctx, url, &response); err != nil {
		return nil, fmt.Errorf("failed to get VNet: %w", err)
	}
	return &proxmox.SdnVnetOutputs{
		SdnVnetInputs: proxmox.SdnVnetInputs{
			Vnet:  response.Vnet,
			Zone:  response.Zone,
			Tag:   response.Tag,
			Alias: response.Alias,
		},
	}, nil
}

// Update updates an existing VNet.
func (adapter *SdnVnetAdapter) Update(
	ctx context.Context,
	vnet string,
	inputs proxmox.SdnVnetInputs,
	oldOutputs proxmox.SdnVnetOutputs,
) error {
	apiResource := &proxmox.SdnVnetAPIResource{
		Zone: inputs.Zone,
		Tag:  inputs.Tag,
	}
	if inputs.Alias == "" && oldOutputs.Alias != "" {
		apiResource.Delete = []string{"alias"}
	} else if inputs.Alias != "" {
		apiResource.Alias = inputs.Alias
	}
	url := fmt.Sprintf("%s/%s", sdnVnetBasePath, vnet)
	if err := adapter.client.Put(ctx, url, apiResource, nil); err != nil {
		return fmt.Errorf("failed to update VNet: %w", err)
	}
	return nil
}

// Delete deletes an existing VNet by its name.
func (adapter *SdnVnetAdapter) Delete(ctx context.Context, vnet string) error {
	url := fmt.Sprintf("%s/%s", sdnVnetBasePath, vnet)
	if err := adapter.client.Delete(ctx, url, nil); err != nil {
		return fmt.Errorf("failed to delete VNet: %w", err)
	}
	return nil
}
