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

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// Ensure Role implements the required interfaces
var (
	_ = (infer.CustomResource[proxmox.RoleInputs, proxmox.RoleOutputs])((*Role)(nil))
	_ = (infer.CustomDelete[proxmox.RoleOutputs])((*Role)(nil))
	_ = (infer.CustomUpdate[proxmox.RoleInputs, proxmox.RoleOutputs])((*Role)(nil))
	_ = (infer.CustomRead[proxmox.RoleInputs, proxmox.RoleOutputs])((*Role)(nil))
	_ = (infer.CustomDiff[proxmox.RoleInputs, proxmox.RoleOutputs])((*Role)(nil))
	_ = infer.Annotated((*Role)(nil))
)

// Role represents a Proxmox role resource
type Role struct {
	RoleOps proxmox.RoleOperations
}

// Create is used to create a new role resource
func (role *Role) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.RoleInputs],
) (infer.CreateResponse[proxmox.RoleOutputs], error) {
	inputs := request.Inputs
	preview := request.DryRun

	logger := p.GetLogger(ctx)
	logger.Debugf("Creating role resource: %v", inputs)

	response := infer.CreateResponse[proxmox.RoleOutputs]{
		ID:     inputs.Name,
		Output: proxmox.RoleOutputs{RoleInputs: inputs},
	}

	if preview {
		return response, nil
	}

	if role.RoleOps == nil {
		return response, errors.New("RoleOperations not configured")
	}

	if err := role.RoleOps.Create(ctx, inputs); err != nil {
		return response, err
	}

	return response, nil
}

// Read is used to read the state of a role resource
func (role *Role) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.RoleInputs, proxmox.RoleOutputs],
) (infer.ReadResponse[proxmox.RoleInputs, proxmox.RoleOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf(
		"Read called for Role with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	response := infer.ReadResponse[proxmox.RoleInputs, proxmox.RoleOutputs](request)

	if role.RoleOps == nil {
		return response, errors.New("RoleOperations not configured")
	}

	// If resource does not exist yet, Pulumi will invoke Create.
	if request.ID == "" {
		return response, nil
	}

	outputs, err := role.RoleOps.Get(ctx, request.ID)
	if err != nil {
		if utils.IsNotFound(err) {
			response.ID = ""
			response.State = proxmox.RoleOutputs{}
			return response, nil
		}
		return response, err
	}

	state := *outputs
	response.Inputs = state.RoleInputs
	response.State = state

	logger.Debugf("Returning updated state: %+v", response.State)
	return response, nil
}

// Update is used to update a role resource
func (role *Role) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.RoleInputs, proxmox.RoleOutputs],
) (infer.UpdateResponse[proxmox.RoleOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Updating role resource: %v", request.ID)

	response := infer.UpdateResponse[proxmox.RoleOutputs]{
		Output: request.State,
	}

	if request.DryRun {
		return response, nil
	}

	if role.RoleOps == nil {
		return response, errors.New("RoleOperations not configured")
	}

	// Merge desired changes over the last-known state to avoid unintentionally
	// zeroing fields and to preserve old behavior.
	newState := request.State.RoleInputs

	if utils.SliceToString(request.Inputs.Privileges) != utils.SliceToString(request.State.Privileges) {
		logger.Infof("Updating privileges from %q to %q", request.State.Privileges, request.Inputs.Privileges)
		newState.Privileges = request.Inputs.Privileges
	}

	response.Output.RoleInputs = newState

	err := role.RoleOps.Update(ctx, request.State.Name, newState)

	return response, err
}

// Delete is used to delete a role resource
func (role *Role) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.RoleOutputs],
) (infer.DeleteResponse, error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting role resource: %v", request.State)

	var response infer.DeleteResponse

	if role.RoleOps == nil {
		return response, errors.New("RoleOperations not configured")
	}

	if err := role.RoleOps.Delete(ctx, request.State.Name); err != nil {
		return response, err
	}
	logger.Debugf("Role resource %v deleted", request.State.Name)

	return response, nil
}

// Diff is used to avoid phantom updates due to ordering changes in list-like properties
func (role *Role) Diff(
	ctx context.Context,
	request infer.DiffRequest[proxmox.RoleInputs, proxmox.RoleOutputs],
) (p.DiffResponse, error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Diff called for Role with ID: %s", request.ID)

	diff := map[string]p.PropertyDiff{}

	// Replace-on-change properties
	if request.Inputs.Name != request.State.Name {
		diff["name"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// Regular update properties
	if utils.SliceToString(request.Inputs.Privileges) != utils.SliceToString(request.State.Privileges) {
		diff["privileges"] = p.PropertyDiff{Kind: p.Update}
	}

	response := p.DiffResponse{
		HasChanges:   len(diff) > 0,
		DetailedDiff: diff,
	}
	return response, nil
}

// Annotate is used to annotate the role resource
func (role *Role) Annotate(a infer.Annotator) {
	a.Describe(
		role,
		"A Proxmox role resource that represents a role in the Proxmox VE.",
	)
}
