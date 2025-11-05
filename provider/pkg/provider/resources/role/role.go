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
	l := p.GetLogger(ctx)
	l.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)

	// set provider id to resource primary key
	response.ID = request.Inputs.Name

	// set output properties
	response.Output = Outputs{Inputs: request.Inputs}

	if request.DryRun {
		return response, nil
	}

	// get client
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	// perform create
	if err = pxc.NewRole(ctx, request.Inputs.Name, utils.SliceToString(request.Inputs.Privileges)); err != nil {
		return response, fmt.Errorf("failed to create role %s: %w", request.Inputs.Name, err)
	}

	// fetch created resource to confirm
	if _, err = pxc.Role(ctx, request.Inputs.Name); err != nil {
		return response, fmt.Errorf("failed to fetch created role %s: %w", request.Inputs.Name, err)
	}

	l.Debugf("Successfully created role %s", request.Inputs.Name)

	return response, nil
}

// Delete is used to delete a role resource
func (role *Role) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	response, err = utils.DeleteResource(utils.DeletedResource{
		Ctx:          ctx,
		ResourceID:   request.State.Name,
		URL:          "/access/roles/" + request.State.Name,
		ResourceType: "role",
	})
	return response, err
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

	// if resource does not exist, pulumi will invoke Create
	if request.ID == "" {
		return response, nil
	}

	// get client
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	// fetch existing resource's permissions/privileges from server
	var existingRolePrivs api.Permission
	if existingRolePrivs, err = pxc.Role(ctx, request.ID); err != nil {
		if utils.IsNotFound(err) {
			response.ID = ""
			return response, nil
		}
		return response, fmt.Errorf("failed to get role %s: %w", request.ID, err)
	}

	l.Debugf("Successfully fetched role: %+v", request.ID)

	// set state from fetched resource
	response.State = Outputs{
		Inputs: Inputs{
			Name:       request.ID,
			Privileges: utils.MapToStringSlice(existingRolePrivs),
		},
	}

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
