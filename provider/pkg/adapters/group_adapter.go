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
	api "github.com/luthermonson/go-proxmox"
)

// Ensure GroupAdapter implements the GroupOperations interface
var _ proxmox.GroupOperations = (*GroupAdapter)(nil)

// GroupAdapter implements proxmox.GroupOperations using the ProxmoxAdapter.
type GroupAdapter struct {
	proxmoxAdapter *ProxmoxAdapter
}

// NewGroupAdapter creates a new GroupAdapter wrapping the given ProxmoxAdapter.
func NewGroupAdapter(proxmoxAdapter *ProxmoxAdapter) *GroupAdapter {
	return &GroupAdapter{proxmoxAdapter: proxmoxAdapter}
}

// Create creates a new Group resource.
func (group *GroupAdapter) Create(ctx context.Context, inputs proxmox.GroupInputs) (err error) {
	if err := group.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	if err := group.proxmoxAdapter.client.NewGroup(ctx, inputs.Name, inputs.Comment); err != nil {
		return fmt.Errorf("failed to create group %s: %v", inputs.Name, err)
	}
	return nil
}

// Get retrieves an existing Group resource by its name.
func (group *GroupAdapter) Get(ctx context.Context, name string) (*proxmox.GroupOutputs, error) {
	if err := group.proxmoxAdapter.Connect(ctx); err != nil {
		return nil, err
	}

	apiGroup, err := group.proxmoxAdapter.client.Group(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get Group resource: %s: %w", name, err)
	}
	return &proxmox.GroupOutputs{
		GroupInputs: proxmox.GroupInputs{
			Name:    apiGroup.GroupID,
			Comment: apiGroup.Comment,
		},
	}, nil
}

// Update updates an existing Group resource.
func (group *GroupAdapter) Update(ctx context.Context, name string, inputs proxmox.GroupInputs) error {
	if err := group.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}
	updatedGroup := &api.Group{
		GroupID: name,
		Comment: inputs.Comment,
	}

	if err := group.proxmoxAdapter.client.Put(ctx, "/access/groups/"+name, updatedGroup, nil); err != nil {
		return fmt.Errorf("failed to update user %s: %w", name, err)
	}
	return nil
}

// Delete deletes an existing Group resource by its name.
func (group *GroupAdapter) Delete(ctx context.Context, name string) error {
	if err := group.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	if err := group.proxmoxAdapter.client.Delete(ctx, "/access/groups/"+name, nil); err != nil {
		return fmt.Errorf("failed to delete group %s: %w", name, err)
	}
	return nil
}
