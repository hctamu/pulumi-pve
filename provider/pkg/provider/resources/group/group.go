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

// Package group provides resources for managing Proxmox groups.
package group

import (
	"context"
	"errors"
	"fmt"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Group represents a Proxmox group resource.
type Group struct {
	GroupOps proxmox.GroupOperations
}

var (
	_ = (infer.CustomResource[proxmox.GroupInputs, proxmox.GroupOutputs])((*Group)(nil))
	_ = (infer.CustomDelete[proxmox.GroupOutputs])((*Group)(nil))
	_ = (infer.CustomRead[proxmox.GroupInputs, proxmox.GroupOutputs])((*Group)(nil))
	_ = (infer.CustomUpdate[proxmox.GroupInputs, proxmox.GroupOutputs])((*Group)(nil))
)

// Create is used to create a new group resource
func (group *Group) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.GroupInputs],
) (response infer.CreateResponse[proxmox.GroupOutputs], err error) {
	inputs := request.Inputs
	preview := request.DryRun

	logger := p.GetLogger(ctx)
	logger.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)

	response = infer.CreateResponse[proxmox.GroupOutputs]{
		ID:     inputs.Name,
		Output: proxmox.GroupOutputs{GroupInputs: inputs},
	}

	if preview {
		return response, nil
	}

	if group.GroupOps == nil {
		err = errors.New("GroupOperations not configured")
		return response, err
	}

	err = group.GroupOps.Create(ctx, inputs)

	return response, err
}

// Delete is used to delete a group resource
func (group *Group) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.GroupOutputs],
) (response infer.DeleteResponse, err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting group resource: %v", request.State)

	if group.GroupOps == nil {
		return response, errors.New("GroupOperations not configured")
	}

	if err := group.GroupOps.Delete(ctx, request.State.Name); err != nil {
		return response, err
	}
	logger.Debugf("Group resource %v deleted", request.State.Name)

	return response, nil
}

// Read is used to read the state of a group resource
func (group *Group) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.GroupInputs, proxmox.GroupOutputs],
) (response infer.ReadResponse[proxmox.GroupInputs, proxmox.GroupOutputs], err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf(
		"Read called for Group with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	response.ID = request.ID
	response.Inputs = request.Inputs
	response.State = request.State

	if group.GroupOps == nil {
		err = errors.New("GroupOperations not configured")
		return response, err
	}

	// if resource does not exist, pulumi will invoke Create
	if request.ID == "" {
		return response, nil
	}

	var outputs *proxmox.GroupOutputs

	if outputs, err = group.GroupOps.Get(ctx, request.ID); err != nil {
		if utils.IsNotFound(err) {
			response.ID = ""
			response.State = proxmox.GroupOutputs{}
			return response, nil
		}
		err = fmt.Errorf("failed to get group %s: %w", request.ID, err)
		return response, err
	}
	existingGroup := outputs.GroupInputs
	logger.Debugf("Successfully fetched group: %+v", existingGroup.Name)

	state := *outputs
	response.State = *outputs
	response.Inputs = state.GroupInputs

	logger.Debugf("Returning updated state: %+v", response.State)
	return response, nil
}

// Update is used to update a group resource
func (group *Group) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.GroupInputs, proxmox.GroupOutputs],
) (response infer.UpdateResponse[proxmox.GroupOutputs], err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Update called for Group with ID: %s, Inputs: %+v, State: %+v",
		request.State.Name,
		request.Inputs,
		request.State,
	)

	response.Output = request.State

	if request.DryRun {
		return response, nil
	}

	if group.GroupOps == nil {
		err = errors.New("GroupOperations not configured")
		return response, err
	}

	newState := request.State.GroupInputs

	// compare and update fields
	if request.Inputs.Comment != request.State.Comment {
		logger.Infof("Updating comment from %q to %q", request.State.Comment, request.Inputs.Comment)
		newState.Comment = request.Inputs.Comment
	}

	response.Output.GroupInputs = newState

	err = group.GroupOps.Update(ctx, request.ID, newState)

	logger.Debugf("Successfully updated group %s", request.State.Name)
	return response, err
}

// Annotate is used to annotate the group resource
// This is used to provide documentation for the resource in the Pulumi schema
// and to provide default values for the resource properties.
func (group *Group) Annotate(a infer.Annotator) {
	a.Describe(group, "A Proxmox group resource that represents a group in the Proxmox VE.")
}
