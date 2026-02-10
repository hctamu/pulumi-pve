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

	utils "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	api "github.com/luthermonson/go-proxmox"
)

// Ensure UserAdapter implements the UserOperations interface
var _ proxmox.UserOperations = (*UserAdapter)(nil)

// UserAdapter implements proxmox.UserOperations using the ProxmoxAdapter.
type UserAdapter struct {
	proxmoxAdapter *ProxmoxAdapter
}

// NewUserAdapter creates a new UserAdapter wrapping the given ProxmoxAdapter.
func NewUserAdapter(proxmoxAdapter *ProxmoxAdapter) *UserAdapter {
	return &UserAdapter{proxmoxAdapter: proxmoxAdapter}
}

// Create creates a new User resource.
func (user *UserAdapter) Create(ctx context.Context, inputs proxmox.UserInputs) (err error) {
	if err := user.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	newUser := api.NewUser{
		UserID:    inputs.Name,
		Comment:   inputs.Comment,
		Email:     inputs.Email,
		Enable:    inputs.Enable,
		Expire:    inputs.Expire,
		Firstname: inputs.Firstname,
		Groups:    inputs.Groups,
		Keys:      inputs.Keys,
		Lastname:  inputs.Lastname,
		Password:  inputs.Password,
	}

	// api.User and api.NewUser inconsistency
	if len(newUser.Keys) == 0 {
		newUser.Keys = nil
	}

	if err = user.proxmoxAdapter.client.NewUser(ctx, &newUser); err != nil {
		return fmt.Errorf("failed to create user %s: %v", inputs.Name, err)
	}
	return nil
}

// Get retrieves an existing User resource by its name.
func (user *UserAdapter) Get(ctx context.Context, name string) (*proxmox.UserOutputs, error) {
	if err := user.proxmoxAdapter.Connect(ctx); err != nil {
		return nil, err
	}

	apiUser, err := user.proxmoxAdapter.client.User(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get User resource: %w", err)
	}

	// Convert keys to slice; ensure nil when empty for consistency
	keys := utils.StringToSlice(apiUser.Keys)
	if len(keys) == 0 {
		keys = nil
	}

	return &proxmox.UserOutputs{
		UserInputs: proxmox.UserInputs{
			Name:      apiUser.UserID,
			Comment:   apiUser.Comment,
			Email:     apiUser.Email,
			Enable:    bool(apiUser.Enable),
			Expire:    apiUser.Expire,
			Firstname: apiUser.Firstname,
			Groups:    apiUser.Groups,
			Keys:      keys,
			Lastname:  apiUser.Lastname,
			// Password is not retrievable for security reasons
		},
	}, nil
}

// Update updates an existing User resource.
func (user *UserAdapter) Update(
	ctx context.Context,
	name string,
	inputs proxmox.UserInputs,
) error {
	if err := user.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	updatedUser := &api.User{
		UserID:    name,
		Comment:   inputs.Comment,
		Email:     inputs.Email,
		Enable:    api.IntOrBool(inputs.Enable),
		Expire:    inputs.Expire,
		Firstname: inputs.Firstname,
		Groups:    inputs.Groups,
		Keys:      utils.SliceToString(inputs.Keys),
		Lastname:  inputs.Lastname,
	}

	if err := user.proxmoxAdapter.client.Put(ctx, "/access/users/"+name, updatedUser, nil); err != nil {
		return fmt.Errorf("failed to update user %s: %w", name, err)
	}

	return nil
}

// Delete deletes an existing User resource by its name.
func (user *UserAdapter) Delete(ctx context.Context, name string) error {
	if err := user.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	if err := user.proxmoxAdapter.client.Delete(ctx, "/access/users/"+name, nil); err != nil {
		return fmt.Errorf("failed to delete user %s: %w", name, err)
	}
	return nil
}
