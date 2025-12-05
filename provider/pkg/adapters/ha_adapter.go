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
	"strconv"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

const (
	haResourcePath = "/cluster/ha/resources/"
)

// Ensure HAAdapter implements the HAOperations interface
var _ proxmox.HAOperations = (*HAAdapter)(nil)

// HAAdapter implements proxmox.HAOperations using a ProxmoxClient.
type HAAdapter struct {
	client proxmox.Client
}

// NewHAAdapter creates a new HAAdapter wrapping the given ProxmoxClient.
func NewHAAdapter(client proxmox.Client) *HAAdapter {
	return &HAAdapter{client: client}
}

// Create creates a new HA resource.
func (ha *HAAdapter) Create(ctx context.Context, inputs proxmox.HAInputs) error {
	// Convert domain inputs to API resource
	apiResource := &proxmox.HaResource{
		Group: inputs.Group,
		State: string(inputs.State),
		Sid:   strconv.Itoa(inputs.ResourceID),
	}

	if err := ha.client.Post(ctx, haResourcePath, apiResource, nil); err != nil {
		return fmt.Errorf("failed to create HA resource: %w", err)
	}
	return nil
}

// Get retrieves an existing HA resource by its ID.
func (ha *HAAdapter) Get(ctx context.Context, id int) (*proxmox.HAOutputs, error) {
	var apiResource *proxmox.HaResource
	url := fmt.Sprintf("%s%v", haResourcePath, id)

	if err := ha.client.Get(ctx, url, &apiResource); err != nil {
		return nil, fmt.Errorf("failed to get HA resource: %w", err)
	}

	// Convert API resource to domain outputs
	return &proxmox.HAOutputs{
		HAInputs: proxmox.HAInputs{
			Group:      apiResource.Group,
			State:      proxmox.HAState(apiResource.State),
			ResourceID: id,
		},
	}, nil
}

// Update updates an existing HA resource.
func (ha *HAAdapter) Update(ctx context.Context, id int, inputs proxmox.HAInputs, oldOutputs proxmox.HAOutputs) error {
	// Convert domain inputs to API resource
	apiResource := &proxmox.HaResource{
		State: string(inputs.State),
	}

	// Handle group field removal or update
	if inputs.Group == "" && oldOutputs.Group != "" {
		apiResource.Delete = []string{"group"}
	} else if inputs.Group != "" {
		apiResource.Group = inputs.Group
	}

	url := fmt.Sprintf("%s%v", haResourcePath, id)
	if err := ha.client.Put(ctx, url, apiResource, nil); err != nil {
		return fmt.Errorf("failed to update HA resource: %w", err)
	}
	return nil
}

// Delete deletes an existing HA resource by its ID.
func (ha *HAAdapter) Delete(ctx context.Context, id int) error {
	deleteURL := fmt.Sprintf("%s%v", haResourcePath, id)
	if err := ha.client.Delete(ctx, deleteURL, nil); err != nil {
		return fmt.Errorf("failed to delete HA resource: %w", err)
	}
	return nil
}
