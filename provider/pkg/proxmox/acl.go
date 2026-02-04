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
	"errors"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// ACLOperations defines the interface for ACL resource operations.
type ACLOperations interface {
	// Create creates a new ACL resource.
	Create(ctx context.Context, inputs ACLInputs) error

	// Get retrieves an existing ACL resource by its name.
	Get(ctx context.Context, name string) (*ACLOutputs, error)

	// Update updates an existing ACL resource.
	Update(ctx context.Context, name string, inputs ACLInputs) error

	// Delete deletes an existing ACL resource.
	Delete(ctx context.Context, ACLOutputs ACLOutputs) error
}

// ACLInputs represents the input properties for the ACL resource
type ACLInputs struct {
	Path      string `pulumi:"path"               provider:"replaceOnChanges"`
	RoleID    string `pulumi:"roleid"             provider:"replaceOnChanges"`
	Type      string `pulumi:"type"               provider:"replaceOnChanges"`
	UGID      string `pulumi:"ugid"               provider:"replaceOnChanges"`
	Propagate bool   `pulumi:"propagate,optional" provider:"replaceOnChanges"`
}

// Annotate adds descriptions to the Input properties for documentation and schema generation.
func (inputs *ACLInputs) Annotate(a infer.Annotator) {
	a.Describe(&inputs.Path, "The path of the ACL.")
	a.Describe(&inputs.RoleID, "The role ID of the ACL.")
	a.Describe(&inputs.Type, "The type of the ACL. Must be one of 'user', 'group', or 'token'.")
	a.Describe(&inputs.UGID, "The user, group, or token ID associated with the ACL.")
	a.Describe(&inputs.Propagate, "Whether the ACL should propagate to child objects.")
}

// ACLOutputs represents the output properties for the ACL resource
type ACLOutputs struct {
	ACLInputs
}

// Enum-like constants for ACL Type
const (
	ACLTypeUser  = "user"
	ACLTypeGroup = "group"
	ACLTypeToken = "token"
)

// ErrACLNotFound is returned when an ACL cannot be found in Proxmox.
var ErrACLNotFound = errors.New("acl not found")

// ErrInvalidACLType is returned when an ACL has an invalid type.
var ErrInvalidACLType = errors.New("invalid type (must be 'user', 'group', or 'token')")
