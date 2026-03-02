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

// RoleOperations defines the interface for Role resource operations.
type RoleOperations interface {
	// Create creates a new Role resource.
	Create(ctx context.Context, inputs RoleInputs) error

	// Get retrieves an existing Role resource by its name.
	Get(ctx context.Context, name string) (*RoleOutputs, error)

	// Update updates an existing Role resource.
	Update(ctx context.Context, name string, inputs RoleInputs) error

	// Delete deletes an existing Role resource by its name.
	Delete(ctx context.Context, name string) error
}

// RoleInputs represents the input properties for the Role resource
type RoleInputs struct {
	Name       string   `pulumi:"name"                provider:"replaceOnChanges"`
	Privileges []string `pulumi:"privileges,optional"`
}

// Annotate adds descriptions to the Input properties for documentation and schema generation.
func (args *RoleInputs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "The name of the Proxmox role.")
	a.Describe(
		&args.Privileges,
		"A list of privileges assigned to this role. Each privilege should be a string identifier (e.g., 'VM.PowerMgmt').",
	)
}

// RoleOutputs represents the output properties for the Role resource
type RoleOutputs struct {
	RoleInputs
}
