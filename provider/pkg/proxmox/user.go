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

// UserOperations defines the interface for User resource operations.
type UserOperations interface {
	// Create creates a new User resource.
	Create(ctx context.Context, inputs UserInputs) error

	// Get retrieves an existing User resource by its name.
	Get(ctx context.Context, name string) (*UserOutputs, error)

	// Update updates an existing User resource.
	Update(ctx context.Context, name string, inputs UserInputs) error

	// Delete deletes an existing User resource by its name.
	Delete(ctx context.Context, name string) error
}

// UserInputs represents the input properties for the User resource
type UserInputs struct {
	Name      string   `pulumi:"userid"             provider:"replaceOnChanges"` // contains realm (e.g., user@pve)
	Comment   string   `pulumi:"comment,optional"`
	Email     string   `pulumi:"email,optional"`
	Enable    bool     `pulumi:"enable,optional"`
	Expire    int      `pulumi:"expire,optional"`
	Firstname string   `pulumi:"firstname,optional"`
	Groups    []string `pulumi:"groups,optional"`
	Keys      []string `pulumi:"keys,optional"`
	Lastname  string   `pulumi:"lastname,optional"`
	Password  string   `pulumi:"password,optional"  provider:"secret,replaceOnChanges"`
}

// Annotate adds descriptions to the Input properties for documentation and schema generation.
func (inputs *UserInputs) Annotate(a infer.Annotator) {
	a.Describe(&inputs.Name, "The user ID of the Proxmox user, including the realm (e.g., 'user@pve').")
	a.Describe(&inputs.Comment, "An optional comment for the user.")
	a.Describe(&inputs.Email, "An optional email address for the user.")
	a.Describe(&inputs.Enable, "Whether the user is enabled. Defaults to true.")
	a.Describe(&inputs.Expire, "The expiration time for the user as a Unix timestamp.")
	a.Describe(&inputs.Firstname, "The first name of the user.")
	a.Describe(&inputs.Groups, "A list of groups the user belongs to.")
	a.Describe(&inputs.Keys, "A list of SSH keys associated with the user.")
	a.Describe(&inputs.Lastname, "The last name of the user.")
	a.Describe(&inputs.Password, "The password for the user. This field is treated as a secret.")
}

// UserOutputs represents the output properties for the User resource
type UserOutputs struct {
	UserInputs
}
