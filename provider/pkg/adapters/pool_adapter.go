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

	// Convert API Pool to domain outputs
	return &proxmox.PoolOutputs{
		PoolInputs: proxmox.PoolInputs{
			Name:    apiPool.PoolID,
			Comment: apiPool.Comment,
		},
	}, nil
}

// Update updates an existing Pool resource.
func (pool *PoolAdapter) Update(ctx context.Context, name string, inputs proxmox.PoolInputs) error {
	if err := pool.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	apiPool, err := pool.proxmoxAdapter.client.Pool(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get Pool resource for update: %w", err)
	}

	updateOption := &api.PoolUpdateOption{
		Comment: inputs.Comment,
	}

	if err := apiPool.Update(ctx, updateOption); err != nil {
		return fmt.Errorf("failed to update Pool resource: %w", err)
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

	if err := apiPool.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete Pool resource: %w", err)
	}
	return nil
}
