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

// Package role provides resources for managing Proxmox roles.
package role

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	utils "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Role represents a Proxmox role resource.
type Role struct{}

var (
	_ = (infer.CustomResource[Inputs, Outputs])((*Role)(nil))
	_ = (infer.CustomDelete[Outputs])((*Role)(nil))
	_ = (infer.CustomRead[Inputs, Outputs])((*Role)(nil))
	_ = (infer.CustomUpdate[Inputs, Outputs])((*Role)(nil))
)

// ErrRoleNotFound is returned when a role cannot be found in Proxmox.
var ErrRoleNotFound = errors.New("role not found")

// Inputs defines the input properties for a Proxmox role resource.
type Inputs struct {
	Name       string   `pulumi:"name"                provider:"replaceOnChanges"`
	Privileges []string `pulumi:"privileges,optional"`
}

// Annotate is used to annotate the input and output properties of the resource.
func (args *Inputs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "The name of the Proxmox role.")
	a.Describe(
		&args.Privileges,
		"A list of privileges assigned to this role. Each privilege should be a string identifier (e.g., 'VM.PowerMgmt').",
	)
}

// Outputs defines the output properties for a Proxmox role resource.
type Outputs struct {
	Inputs
}

// Create is used to create a new role resource
func (role *Role) Create(
	ctx context.Context,
	request infer.CreateRequest[Inputs],
) (response infer.CreateResponse[Outputs], err error) {
	response.ID = request.Name
	response.Output = Outputs{Inputs: Inputs{
		Name:       request.Inputs.Name,
		Privileges: sort.StringSlice(request.Inputs.Privileges),
	}}

	l := p.GetLogger(ctx)
	l.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)
	if request.DryRun {
		return response, nil
	}

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	privsStr := utils.SliceToString(request.Inputs.Privileges)

	err = pxc.NewRole(ctx, request.Inputs.Name, privsStr)
	if err != nil {
		return response, fmt.Errorf("failed to create role %s: %w", request.Inputs.Name, err)
	}

	return response, nil
}

// Delete is used to delete a role resource
func (role *Role) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Deleting role %s", request.State.Name)

	// get client
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	// perform delete
	if err = pxc.Req(ctx, http.MethodDelete, "/access/roles/"+request.State.Name, nil, nil); err != nil {
		return response, fmt.Errorf("failed to delete role %s: %w", request.State.Name, err)
	}

	l.Debugf("Successfully deleted role %s", request.State.Name)
	return response, nil
}

// Read is used to read the state of a role resource
func (role *Role) Read(
	ctx context.Context,
	request infer.ReadRequest[Inputs, Outputs],
) (response infer.ReadResponse[Inputs, Outputs], err error) {
	response.ID = request.ID
	response.Inputs = request.Inputs
	l := p.GetLogger(ctx)
	l.Debugf(
		"Read called for Role with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	if request.ID == "" {
		l.Warningf("Missing Role ID")
		err = errors.New("missing role ID")
		return response, err
	}

	var existingRole *api.Role
	if existingRole, err = GetRole(ctx, request.State.Name, pxc); err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			l.Debugf("Role %s not found during read, marking as deleted.", request.State.Name)
			return infer.ReadResponse[Inputs, Outputs]{}, nil
		}
		return response, fmt.Errorf("failed to get role: %w", err)

	}

	l.Debugf("Successfully fetched role: %+v", existingRole.RoleID)

	response.State = Outputs{
		Inputs: Inputs{
			Name:       existingRole.RoleID,
			Privileges: utils.StringToSlice(existingRole.Privs),
		},
	}

	response.Inputs = response.State.Inputs

	l.Debugf("Returning updated state: %+v", response.State)
	return response, nil
}

// Update is used to update a role resource
func (role *Role) Update(
	ctx context.Context,
	request infer.UpdateRequest[Inputs, Outputs],
) (response infer.UpdateResponse[Outputs], err error) {
	response.Output = request.State

	l := p.GetLogger(ctx)
	l.Debugf("Update called for Role with ID: %s, Inputs: %+v, State: %+v",
		request.State.Name,
		request.Inputs,
		request.State,
	)

	if request.DryRun {
		return response, nil
	}

	// compare and update fields
	if utils.SliceToString(request.Inputs.Privileges) != utils.SliceToString(request.State.Privileges) {
		l.Infof("Updating privileges from %q to %q", request.State.Privileges, request.Inputs.Privileges)
		response.Output.Privileges = request.Inputs.Privileges
	}

	// prepare updated resource
	updatedRole := &api.Role{
		RoleID: response.Output.Name,
		Privs:  utils.SliceToString(response.Output.Privileges),
	}

	// get client
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	// perform update
	if err = pxc.Put(ctx, "/access/roles/"+updatedRole.RoleID, updatedRole, nil); err != nil {
		return response, fmt.Errorf("failed to update role %s: %w", request.State.Name, err)
	}

	l.Debugf("Successfully updated role %s", request.State.Name)
	return response, nil
}

// GetRole is used to retrieve a role by its ID
func GetRole(
	ctx context.Context,
	roleid string,
	pxc *px.Client,
) (role *api.Role, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("GetRole called for Role with ID: %s", roleid)

	allRoles, err := pxc.Roles(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get roles: %v", err)
		l.Error(err.Error())
		return nil, err
	}

	for _, r := range allRoles {
		if r.RoleID == roleid {
			return r, nil
		}
	}
	return nil, ErrRoleNotFound
}
