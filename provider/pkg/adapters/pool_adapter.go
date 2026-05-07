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
	"strings"

	api "github.com/luthermonson/go-proxmox"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

// Ensure PoolAdapter implements the PoolOperations interface
var _ proxmox.PoolOperations = (*PoolAdapter)(nil)

// PoolAdapter implements proxmox.PoolOperations using the ProxmoxAdapter.
type PoolAdapter struct {
	proxmoxAdapter *ProxmoxAdapter
}

// NewPoolAdapter creates a new PoolAdapter wrapping the given ProxmoxAdapter.
func NewPoolAdapter(proxmoxAdapter *ProxmoxAdapter) *PoolAdapter {
	return &PoolAdapter{proxmoxAdapter: proxmoxAdapter}
}

// Create creates a new Pool resource.
func (pool *PoolAdapter) Create(ctx context.Context, inputs proxmox.PoolInputs) error {
	if err := pool.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	if err := pool.proxmoxAdapter.client.NewPool(ctx, inputs.Name, inputs.Comment); err != nil {
		return fmt.Errorf("failed to create Pool resource: %w", err)
	}

	if len(inputs.VMs) == 0 && len(inputs.Storage) == 0 {
		return nil
	}

	apiPool, err := pool.proxmoxAdapter.client.Pool(ctx, inputs.Name)
	if err != nil {
		return fmt.Errorf("failed to get Pool resource after creation: %w", err)
	}

	updateOption := &api.PoolUpdateOption{
		VirtualMachines: utils.JoinInts(inputs.VMs),
		Storage:         strings.Join(inputs.Storage, ","),
	}
	if err := apiPool.Update(ctx, updateOption); err != nil {
		return fmt.Errorf("failed to add members to Pool resource: %w", err)
	}
	return nil
}

// Get retrieves an existing Pool resource by its name.
func (pool *PoolAdapter) Get(ctx context.Context, name string) (*proxmox.PoolOutputs, error) {
	if err := pool.proxmoxAdapter.Connect(ctx); err != nil {
		return nil, err
	}

	apiPool, err := pool.proxmoxAdapter.client.Pool(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get Pool resource: %w", err)
	}

	var vms []int
	var storages []string
	for i := range apiPool.Members {
		member := &apiPool.Members[i]
		switch member.Type {
		case "qemu", "lxc":
			vms = append(vms, int(member.VMID)) //nolint:gosec // VMID values are always small positive integers
		case "storage":
			storages = append(storages, member.Storage)
		}
	}

	return &proxmox.PoolOutputs{
		PoolInputs: proxmox.PoolInputs{
			Name:    apiPool.PoolID,
			Comment: apiPool.Comment,
			VMs:     vms,
			Storage: storages,
		},
	}, nil
}

// Update updates an existing Pool resource.
func (pool *PoolAdapter) Update(
	ctx context.Context,
	name string,
	state proxmox.PoolInputs,
	inputs proxmox.PoolInputs,
) error {
	if err := pool.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	apiPool, err := pool.proxmoxAdapter.client.Pool(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get Pool resource for update: %w", err)
	}

	vmAdds := utils.IntSliceDiff(inputs.VMs, state.VMs)
	vmRemoves := utils.IntSliceDiff(state.VMs, inputs.VMs)
	storageAdds := utils.StringSliceDiff(inputs.Storage, state.Storage)
	storageRemoves := utils.StringSliceDiff(state.Storage, inputs.Storage)
	commentChanged := inputs.Comment != state.Comment

	// Issue add call only when there is something to add or the comment changed.
	if commentChanged || len(vmAdds) > 0 || len(storageAdds) > 0 {
		addOption := &api.PoolUpdateOption{
			Comment:         inputs.Comment,
			VirtualMachines: utils.JoinInts(vmAdds),
			Storage:         strings.Join(storageAdds, ","),
		}
		if err := apiPool.Update(ctx, addOption); err != nil {
			return fmt.Errorf("failed to update Pool resource: %w", err)
		}
	}

	// Remove members in a separate call if needed.
	if len(vmRemoves) > 0 || len(storageRemoves) > 0 {
		removeOption := &api.PoolUpdateOption{
			Delete:          true,
			VirtualMachines: utils.JoinInts(vmRemoves),
			Storage:         strings.Join(storageRemoves, ","),
		}
		if err := apiPool.Update(ctx, removeOption); err != nil {
			return fmt.Errorf("failed to remove members from Pool resource: %w", err)
		}
	}

	return nil
}

// Delete deletes an existing Pool resource by its name.
func (pool *PoolAdapter) Delete(ctx context.Context, name string) error {
	if err := pool.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	apiPool, err := pool.proxmoxAdapter.client.Pool(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get Pool resource for deletion: %w", err)
	}

	// Proxmox requires the pool to be empty before deletion.
	// Collect all current member VMs and storage and remove them first.
	var vmIDs []int
	var storageNames []string
	for i := range apiPool.Members {
		member := &apiPool.Members[i]
		switch member.Type {
		case "qemu", "lxc":
			vmIDs = append(vmIDs, int(member.VMID)) //nolint:gosec // VMID values are always small positive integers
		case "storage":
			storageNames = append(storageNames, member.Storage)
		}
	}

	if len(vmIDs) > 0 || len(storageNames) > 0 {
		removeOption := &api.PoolUpdateOption{
			Delete:          true,
			VirtualMachines: utils.JoinInts(vmIDs),
			Storage:         strings.Join(storageNames, ","),
		}
		if err := apiPool.Update(ctx, removeOption); err != nil {
			return fmt.Errorf("failed to remove members from Pool resource before deletion: %w", err)
		}
	}

	if err := apiPool.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete Pool resource: %w", err)
	}
	return nil
}
