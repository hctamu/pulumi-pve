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
	"errors"
	"fmt"
	"strings"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
)

// Ensure ACLAdapter implements the ACLOperations interface
var _ proxmox.ACLOperations = (*ACLAdapter)(nil)

// ACLAdapter implements proxmox.ACLOperations using the ProxmoxAdapter.
type ACLAdapter struct {
	proxmoxAdapter *ProxmoxAdapter
}

// NewACLAdapter creates a new ACLAdapter wrapping the given ProxmoxAdapter.
func NewACLAdapter(proxmoxAdapter *ProxmoxAdapter) *ACLAdapter {
	return &ACLAdapter{proxmoxAdapter: proxmoxAdapter}
}

// Create creates a new ACL resource.
func (acl *ACLAdapter) Create(ctx context.Context, inputs proxmox.ACLInputs) error {
	if err := acl.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	// create new resource
	newACL := api.ACLOptions{
		Path:      inputs.Path,
		Roles:     inputs.RoleID,
		Propagate: api.IntOrBool(inputs.Propagate),
	}
	switch inputs.Type {
	case proxmox.ACLTypeGroup:
		if _, err := acl.proxmoxAdapter.client.Group(ctx, inputs.UGID); err != nil {
			return fmt.Errorf("failed to find group %s for ACL: %w", inputs.UGID, err)
		}
		newACL.Groups = inputs.UGID
	case proxmox.ACLTypeUser:
		if _, err := acl.proxmoxAdapter.client.User(ctx, inputs.UGID); err != nil {
			return fmt.Errorf("failed to find user %s for ACL: %w", inputs.UGID, err)
		}
		newACL.Users = inputs.UGID
	case proxmox.ACLTypeToken:
		newACL.Tokens = inputs.UGID
		// No validation for tokens
	default:
		return proxmox.ErrInvalidACLType
	}

	if err := acl.proxmoxAdapter.client.UpdateACL(ctx, newACL); err != nil {
		return fmt.Errorf("failed to create ACL resource: %w", err)
	}

	return nil
}

// Get retrieves an existing ACL resource by its composed ID.
func (acl *ACLAdapter) Get(ctx context.Context, aclid string) (*proxmox.ACLOutputs, error) {
	if err := acl.proxmoxAdapter.Connect(ctx); err != nil {
		return nil, err
	}

	decomposedACL, err := decomposeACLID(aclid)
	if err != nil {
		return nil, err
	}

	apiACL, err := GetACL(ctx, decomposedACL, acl)
	if err != nil {
		return nil, err
	}

	return &proxmox.ACLOutputs{
		ACLInputs: proxmox.ACLInputs{
			Path:      apiACL.Path,
			RoleID:    apiACL.RoleID,
			Type:      apiACL.Type,
			UGID:      apiACL.UGID,
			Propagate: bool(apiACL.Propagate),
		},
	}, nil
}

// Update updates an existing ACL resource.
func (acl *ACLAdapter) Update(ctx context.Context, aclid string, inputs proxmox.ACLInputs) error {
	return errors.New("ACL resource update is not supported, because ACLs are uniquely identified by their properties")
}

// Delete deletes an existing ACL resource by its composed ID.
func (acl *ACLAdapter) Delete(ctx context.Context, outputs proxmox.ACLOutputs) error {
	if err := acl.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	deletedACL := api.ACLOptions{
		Path:      outputs.Path,
		Roles:     outputs.RoleID,
		Propagate: api.IntOrBool(outputs.Propagate),
		Delete:    api.IntOrBool(true),
	}
	switch outputs.Type {
	case proxmox.ACLTypeGroup:
		deletedACL.Groups = outputs.UGID
	case proxmox.ACLTypeUser:
		deletedACL.Users = outputs.UGID
	case proxmox.ACLTypeToken:
		deletedACL.Tokens = outputs.UGID
	default:
		return proxmox.ErrInvalidACLType
	}

	if err := acl.proxmoxAdapter.client.UpdateACL(ctx, deletedACL); err != nil {
		return fmt.Errorf("failed to delete ACL resource: %w", err)
	}

	return nil
}

// decomposeACLID splits a composite ID into its components.
func decomposeACLID(id string) (proxmox.ACLInputs, error) {
	parts := strings.Split(id, "|")
	if len(parts) != 4 {
		return proxmox.ACLInputs{}, fmt.Errorf("invalid ACL ID: %s", id)
	}
	acl := proxmox.ACLInputs{
		Path:      parts[0],
		RoleID:    parts[1],
		Type:      parts[2],
		UGID:      parts[3],
		Propagate: false,
	}
	return acl, nil
}

// GetACL is used to retrieve an ACL by its keys
func GetACL(
	ctx context.Context,
	aclparams proxmox.ACLInputs,
	aclAdapter *ACLAdapter,
) (*api.ACL, error) {
	l := p.GetLogger(ctx)
	l.Debugf(
		"GetACL called for ACL with keys: %s, %s, %s, %s",
		aclparams.Path,
		aclparams.RoleID,
		aclparams.Type,
		aclparams.UGID,
	)

	allACLs, err := aclAdapter.proxmoxAdapter.client.ACL(ctx)
	if err != nil {
		wrapped := fmt.Errorf("failed to get ACLs: %v", err)
		l.Error(wrapped.Error())
		return nil, wrapped
	}

	for _, r := range allACLs {
		if r.Path == aclparams.Path && r.RoleID == aclparams.RoleID && r.Type == aclparams.Type &&
			r.UGID == aclparams.UGID {
			return r, nil
		}
	}
	return nil, proxmox.ErrACLNotFound
}
