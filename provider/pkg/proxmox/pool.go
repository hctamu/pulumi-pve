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
)

// PoolOperations defines the interface for Pool resource operations.
type PoolOperations interface {
	// Create creates a new Pool resource.
	Create(ctx context.Context, inputs PoolInputs) error

	// Get retrieves an existing Pool resource by its name.
	Get(ctx context.Context, name string) (*PoolOutputs, error)

	// Update updates an existing Pool resource.
	Update(ctx context.Context, name string, state PoolInputs, inputs PoolInputs) error

	// Delete deletes an existing Pool resource by its name.
	Delete(ctx context.Context, name string) error
}

// PoolInputs represents the input properties for the Pool resource
type PoolInputs struct {
	Name    string   `pulumi:"name"             provider:"replaceOnChanges"`
	Comment string   `pulumi:"comment,optional"`
	VMs     []int    `pulumi:"vms,optional"`
	Storage []string `pulumi:"storage,optional"`
}

// Annotate adds descriptions to the Input properties for documentation and schema generation.
func (inputs *PoolInputs) Annotate(a infer.Annotator) {
	a.Describe(&inputs.Name, "The name of the Proxmox pool.")
	a.Describe(
		&inputs.Comment,
		"An optional comment for the pool",
	)
	a.Describe(&inputs.VMs, "An optional list of VM IDs to assign to the pool.")
	a.Describe(&inputs.Storage, "An optional list of storage names to assign to the pool.")
}

// PoolOutputs represents the output properties for the Pool resource
type PoolOutputs struct {
	PoolInputs
}
