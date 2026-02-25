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

// Package user provides resources for managing Proxmox users.
package user

import (
	"context"
	"errors"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Ensure User implements the required interfaces
var (
	_ = (infer.CustomResource[proxmox.UserInputs, proxmox.UserOutputs])((*User)(nil))
	_ = (infer.CustomDelete[proxmox.UserOutputs])((*User)(nil))
	_ = (infer.CustomUpdate[proxmox.UserInputs, proxmox.UserOutputs])((*User)(nil))
	_ = (infer.CustomRead[proxmox.UserInputs, proxmox.UserOutputs])((*User)(nil))
	_ = (infer.CustomDiff[proxmox.UserInputs, proxmox.UserOutputs])((*User)(nil))
	_ = infer.Annotated((*User)(nil))
)

// User represents a Proxmox user resource
type User struct {
	UserOps proxmox.UserOperations
}

// Create is used to create a new user resource
func (user *User) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.UserInputs],
) (infer.CreateResponse[proxmox.UserOutputs], error) {
	inputs := request.Inputs
	preview := request.DryRun

	logger := p.GetLogger(ctx)
	logger.Debugf("Creating user resource: %v", inputs)

	response := infer.CreateResponse[proxmox.UserOutputs]{
		ID:     inputs.Name,
		Output: proxmox.UserOutputs{UserInputs: inputs},
	}

	if preview {
		return response, nil
	}

	if user.UserOps == nil {
		return response, errors.New("UserOperations not configured")
	}

	if err := user.UserOps.Create(ctx, inputs); err != nil {
		return response, err
	}

	return response, nil
}

// Read is used to read the state of a user resource
func (user *User) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.UserInputs, proxmox.UserOutputs],
) (infer.ReadResponse[proxmox.UserInputs, proxmox.UserOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf(
		"Read called for User with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	response := infer.ReadResponse[proxmox.UserInputs, proxmox.UserOutputs](request)

	if user.UserOps == nil {
		return response, errors.New("UserOperations not configured")
	}

	// If resource does not exist yet, Pulumi will invoke Create.
	if request.ID == "" {
		return response, nil
	}

	outputs, err := user.UserOps.Get(ctx, request.ID)
	if err != nil {
		if utils.IsNotFound(err) {
			response.ID = ""
			response.State = proxmox.UserOutputs{}
			return response, nil
		}
		return response, err
	}

	// Proxmox does not allow retrieving passwords; preserve the value from inputs.
	state := *outputs
	state.Password = request.Inputs.Password

	response.Inputs = state.UserInputs
	response.State = state

	logger.Debugf("Returning updated state: %+v", response.State)
	return response, nil
}

// Update is used to update a user resource
func (user *User) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.UserInputs, proxmox.UserOutputs],
) (infer.UpdateResponse[proxmox.UserOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Updating user resource: %v", request.ID)

	response := infer.UpdateResponse[proxmox.UserOutputs]{
		Output: request.State,
	}

	if request.DryRun {
		return response, nil
	}

	if user.UserOps == nil {
		return response, errors.New("UserOperations not configured")
	}

	// Merge desired changes over the last-known state to avoid unintentionally
	// zeroing fields and to preserve old behavior.
	newState := request.State.UserInputs

	if request.Inputs.Comment != request.State.Comment {
		logger.Infof("Updating comment from %q to %q", request.State.Comment, request.Inputs.Comment)
		newState.Comment = request.Inputs.Comment
	}
	if request.Inputs.Email != request.State.Email {
		logger.Infof("Updating email from %q to %q", request.State.Email, request.Inputs.Email)
		newState.Email = request.Inputs.Email
	}
	if request.Inputs.Enable != request.State.Enable {
		logger.Infof("Updating enable from %v to %v", request.State.Enable, request.Inputs.Enable)
		newState.Enable = request.Inputs.Enable
	}
	if request.Inputs.Expire != request.State.Expire {
		logger.Infof("Updating expire from %d to %d", request.State.Expire, request.Inputs.Expire)
		newState.Expire = request.Inputs.Expire
	}
	if request.Inputs.Firstname != request.State.Firstname {
		logger.Infof("Updating firstname from %q to %q", request.State.Firstname, request.Inputs.Firstname)
		newState.Firstname = request.Inputs.Firstname
	}
	if utils.SliceToString(request.Inputs.Groups) != utils.SliceToString(request.State.Groups) {
		logger.Infof("Updating groups from %q to %q", request.State.Groups, request.Inputs.Groups)
		newState.Groups = request.Inputs.Groups
	}
	if utils.SliceToString(request.Inputs.Keys) != utils.SliceToString(request.State.Keys) {
		logger.Infof("Updating keys from %q to %q", request.State.Keys, request.Inputs.Keys)
		newState.Keys = request.Inputs.Keys
	}
	if request.Inputs.Lastname != request.State.Lastname {
		logger.Infof("Updating lastname from %q to %q", request.State.Lastname, request.Inputs.Lastname)
		newState.Lastname = request.Inputs.Lastname
	}

	response.Output.UserInputs = newState

	err := user.UserOps.Update(ctx, request.State.Name, newState)

	return response, err
}

// Delete is used to delete a user resource
func (user *User) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.UserOutputs],
) (infer.DeleteResponse, error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting user resource: %v", request.State)

	var response infer.DeleteResponse

	if user.UserOps == nil {
		return response, errors.New("UserOperations not configured")
	}

	if err := user.UserOps.Delete(ctx, request.State.Name); err != nil {
		return response, err
	}
	logger.Debugf("User resource %v deleted", request.State.Name)

	return response, nil
}

// Diff is used to avoid phantom updates due to ordering changes in list-like properties
func (user *User) Diff(
	ctx context.Context,
	request infer.DiffRequest[proxmox.UserInputs, proxmox.UserOutputs],
) (p.DiffResponse, error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Diff called for User with ID: %s", request.ID)

	diff := map[string]p.PropertyDiff{}

	// Replace-on-change properties
	if request.Inputs.Name != request.State.Name {
		diff["userid"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if request.Inputs.Password != request.State.Password {
		diff["password"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	// Regular update properties
	if request.Inputs.Comment != request.State.Comment {
		diff["comment"] = p.PropertyDiff{Kind: p.Update}
	}
	if request.Inputs.Email != request.State.Email {
		diff["email"] = p.PropertyDiff{Kind: p.Update}
	}
	if request.Inputs.Enable != request.State.Enable {
		diff["enable"] = p.PropertyDiff{Kind: p.Update}
	}
	if request.Inputs.Expire != request.State.Expire {
		diff["expire"] = p.PropertyDiff{Kind: p.Update}
	}
	if request.Inputs.Firstname != request.State.Firstname {
		diff["firstname"] = p.PropertyDiff{Kind: p.Update}
	}
	if request.Inputs.Lastname != request.State.Lastname {
		diff["lastname"] = p.PropertyDiff{Kind: p.Update}
	}

	// Treat lists as sets for diffing (order-insensitive; nil/empty-insensitive).
	if utils.SliceToString(request.Inputs.Groups) != utils.SliceToString(request.State.Groups) {
		diff["groups"] = p.PropertyDiff{Kind: p.Update}
	}
	if utils.SliceToString(request.Inputs.Keys) != utils.SliceToString(request.State.Keys) {
		diff["keys"] = p.PropertyDiff{Kind: p.Update}
	}

	response := p.DiffResponse{
		HasChanges:   len(diff) > 0,
		DetailedDiff: diff,
	}
	return response, nil
}

// Annotate is used to annotate the user resource
// This is used to provide documentation for the resource in the Pulumi schema
// and to provide default values for the resource properties.
func (user *User) Annotate(a infer.Annotator) {
	a.Describe(
		user,
		"A Proxmox user resource that represents a user in the Proxmox VE.",
	)
}
