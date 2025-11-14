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
	"fmt"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Group represents a Proxmox group resource.
type Group struct{}

var (
	_ = (infer.CustomResource[Inputs, Outputs])((*Group)(nil))
	_ = (infer.CustomDelete[Outputs])((*Group)(nil))
	_ = (infer.CustomRead[Inputs, Outputs])((*Group)(nil))
	_ = (infer.CustomUpdate[Inputs, Outputs])((*Group)(nil))
)

// Inputs defines the input properties for a Proxmox group resource.
type Inputs struct {
	Name    string `pulumi:"name"             provider:"replaceOnChanges"`
	Comment string `pulumi:"comment,optional"`
}

// Annotate is used to annotate the input and output properties of the resource.
func (args *Inputs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "The name of the Proxmox group.")
	a.SetDefault(&args.Comment, "Default group comment")
	a.Describe(
		&args.Comment,
		"An optional comment for the group. If not provided, defaults to 'Default group comment'.",
	)
}

// Outputs defines the output properties for a Proxmox group resource.
type Outputs struct {
	Inputs
}

// Create is used to create a new group resource
func (group *Group) Create(
	ctx context.Context,
	request infer.CreateRequest[Inputs],
) (response infer.CreateResponse[Outputs], err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)

	// set provider ID to resource primary key
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
	if err = pxc.NewGroup(ctx, request.Inputs.Name, request.Inputs.Comment); err != nil {
		return response, fmt.Errorf("failed to create group %s: %w", request.Inputs.Name, err)
	}

	// fetch created resource to confirm
	if _, err = pxc.Group(ctx, request.Inputs.Name); err != nil {
		return response, fmt.Errorf("failed to fetch group %s: %v", request.Inputs.Name, err)
	}

	l.Debugf("Successfully created group %s", response.ID)

	return response, nil
}

// Delete is used to delete a group resource
func (group *Group) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	response, err = utils.DeleteResource(utils.DeletedResource{
		Ctx:          ctx,
		ResourceID:   request.State.Name,
		URL:          "/access/groups/" + request.State.Name,
		ResourceType: "group",
	})
	return response, err
}

// Read is used to read the state of a group resource
func (group *Group) Read(
	ctx context.Context,
	request infer.ReadRequest[Inputs, Outputs],
) (response infer.ReadResponse[Inputs, Outputs], err error) {
	response.ID = request.ID
	response.Inputs = request.Inputs

	l := p.GetLogger(ctx)
	l.Debugf(
		"Read called for Group with ID: %s, Inputs: %+v, State: %+v",
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

	// fetch existing resource from server
	var existingGroup *api.Group
	if existingGroup, err = pxc.Group(ctx, request.ID); err != nil {
		if utils.IsNotFound(err) {
			response.ID = ""
			return response, nil
		}
		err = fmt.Errorf("failed to get group %s: %w", request.ID, err)
		return response, err
	}

	l.Debugf("Successfully fetched group: %+v", existingGroup.GroupID)

	// set state from fetched resource
	response.State = Outputs{
		Inputs: Inputs{
			Name:    existingGroup.GroupID,
			Comment: existingGroup.Comment,
		},
	}

	// update inputs to match state
	response.Inputs = response.State.Inputs

	l.Debugf("Returning updated state: %+v", response.State)
	return response, nil
}

// Update is used to update a group resource
func (group *Group) Update(
	ctx context.Context,
	request infer.UpdateRequest[Inputs, Outputs],
) (response infer.UpdateResponse[Outputs], err error) {
	response.Output = request.State

	l := p.GetLogger(ctx)
	l.Debugf("Update called for Group with ID: %s, Inputs: %+v, State: %+v",
		request.State.Name,
		request.Inputs,
		request.State,
	)

	if request.DryRun {
		return response, nil
	}

	// compare and update fields
	if request.Inputs.Comment != request.State.Comment {
		l.Infof("Updating comment from %q to %q", request.State.Comment, request.Inputs.Comment)
		response.Output.Comment = request.Inputs.Comment
	}

	// prepare updated resource
	updatedGroup := &api.Group{
		GroupID: response.Output.Name,
		Comment: response.Output.Comment,
	}

	// get client
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	// perform update
	if err = pxc.Put(ctx, "/access/groups/"+updatedGroup.GroupID, updatedGroup, nil); err != nil {
		return response, fmt.Errorf("failed to update group %s: %w", request.State.Name, err)
	}

	l.Debugf("Successfully updated group %s", request.State.Name)
	return response, nil
}
