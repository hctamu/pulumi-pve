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

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	api "github.com/luthermonson/go-proxmox"
)

// Ensure RoleAdapter implements the RoleOperations interface
var _ proxmox.RoleOperations = (*RoleAdapter)(nil)

// RoleAdapter implements proxmox.RoleOperations using the ProxmoxAdapter.
type RoleAdapter struct {
	proxmoxAdapter *ProxmoxAdapter
}

// NewRoleAdapter creates a new RoleAdapter wrapping the given ProxmoxAdapter.
func NewRoleAdapter(proxmoxAdapter *ProxmoxAdapter) *RoleAdapter {
	return &RoleAdapter{proxmoxAdapter: proxmoxAdapter}
}

// Create creates a new Role resource.
func (role *RoleAdapter) Create(ctx context.Context, inputs proxmox.RoleInputs) error {
	if err := role.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	privileges := utils.SliceToString(inputs.Privileges)

	if err := role.proxmoxAdapter.client.NewRole(ctx, inputs.Name, privileges); err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}

	return nil
}

// Get retrieves an existing Role resource by its name.
func (role *RoleAdapter) Get(ctx context.Context, name string) (*proxmox.RoleOutputs, error) {
	if err := role.proxmoxAdapter.Connect(ctx); err != nil {
		return nil, err
	}

	existingRolePrivs, err := role.proxmoxAdapter.client.Role(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get Role resource: %w", err)
	}

	// Convert the privileges from the API response to the expected output format
	privileges := utils.MapToStringSlice(existingRolePrivs)
	if len(privileges) == 0 {
		privileges = nil
	}

	return &proxmox.RoleOutputs{
		RoleInputs: proxmox.RoleInputs{
			Name:       name,
			Privileges: privileges,
		},
	}, nil
}

// Update updates an existing Role resource.
func (role *RoleAdapter) Update(ctx context.Context, name string, inputs proxmox.RoleInputs) error {
	if err := role.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	updatedRole := &api.Role{
		RoleID: name,
		Privs:  utils.SliceToString(inputs.Privileges),
	}

	if err := role.proxmoxAdapter.client.Put(ctx, "/access/roles/"+name, updatedRole, nil); err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	return nil
}

// Delete deletes an existing Role resource by its name.
func (role *RoleAdapter) Delete(ctx context.Context, name string) error {
	if err := role.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	if err := role.proxmoxAdapter.client.Delete(ctx, "/access/roles/"+name, nil); err != nil {
		return fmt.Errorf("failed to delete role %s: %w", name, err)
	}
	return nil
}
