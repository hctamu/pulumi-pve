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

// GroupOperations defines the interface for Group resource operations.
type GroupOperations interface {
	// Create creates a new Group resource.
	Create(ctx context.Context, inputs GroupInputs) error

	// Get retrieves an existing Group resource by its name.
	Get(ctx context.Context, name string) (*GroupOutputs, error)

	// Update updates an existing Group resource.
	Update(ctx context.Context, name string, inputs GroupInputs) error

	// Delete deletes an existing Group resource by its name.
	Delete(ctx context.Context, name string) error
}

// GroupInputs represents the input properties for the Group resource
type GroupInputs struct {
	Name    string `pulumi:"name"             provider:"replaceOnChanges"`
	Comment string `pulumi:"comment,optional"`
}

// Annotate is used to annotate the input and output properties of the resource.
func (args *GroupInputs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "The name of the Proxmox group.")
	a.SetDefault(&args.Comment, "Default group comment")
	a.Describe(
		&args.Comment,
		"An optional comment for the group. If not provided, defaults to 'Default group comment'.",
	)
}

// GroupOutputs represents the output properties for the Group resource
type GroupOutputs struct {
	GroupInputs
}
