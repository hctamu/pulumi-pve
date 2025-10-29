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

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
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
	response.ID = request.Name
	response.Output = Outputs{Inputs: request.Inputs}
	l := p.GetLogger(ctx)
	l.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)
	if request.DryRun {
		return response, nil
	}

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	if err = pxc.NewGroup(ctx, request.Inputs.Name, request.Inputs.Comment); err != nil {
		err = fmt.Errorf("failed to create group: %v", err)
		l.Error(err.Error())
	}

	return response, err
}

// Delete is used to delete a group resource
func (group *Group) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	l := p.GetLogger(ctx)
	l.Debugf("Deleting group %v", request.State.Name)

	var existingGroup *api.Group
	if existingGroup, err = pxc.Group(ctx, request.State.Name); err != nil {
		err = fmt.Errorf("failed to get group %s: %v", request.State.Name, err)
		l.Error(err.Error())
		return response, err
	}

	if err = existingGroup.Delete(ctx); err != nil {
		err = fmt.Errorf("failed to delete group %s: %v", request.State.Name, err)
		l.Error(err.Error())
		return response, err
	}

	return response, nil
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

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	if request.ID == "" {
		l.Warningf("Missing Group ID")
		err = errors.New("missing group ID")
		return response, err
	}

	var existingGroup *api.Group
	if existingGroup, err = pxc.Group(ctx, response.Inputs.Name); err != nil {
		err = fmt.Errorf("failed to get group %s: %v", response.Inputs.Name, err)
		return response, err
	}

	groupName := existingGroup.GroupID

	l.Debugf("Successfully fetched group: %+v", groupName)

	response.State = Outputs{
		Inputs: Inputs{
			Name:    groupName,
			Comment: existingGroup.Comment,
		},
	}

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
	l.Debugf("Updating group: %v", request.State.Name)

	if request.DryRun {
		return response, nil
	}

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	var existingGroup *api.Group
	if existingGroup, err = pxc.Group(ctx, request.State.Name); err != nil {
		err = fmt.Errorf("failed to get group %s: %v", request.State.Name, err)
		return response, err
	}

	l.Infof("Updating comment from %q to %q", existingGroup.Comment, request.Inputs.Comment)
	existingGroup.Comment = request.Inputs.Comment

	// pxc.Group GET method gives back the Members field too, but
	// API does not support updating members directly.
	// So we clear it here to avoid sending it back in the update.
	existingGroup.Members = nil

	if err = existingGroup.Update(ctx); err != nil {
		err = fmt.Errorf("failed to update group %s: %v", request.State.Name, err)
		return response, err
	}

	response.Output = Outputs{request.Inputs}
	return response, nil
}
