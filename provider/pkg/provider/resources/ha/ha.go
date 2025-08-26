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

// Package ha provides resources for managing Proxmox HA (High Availability) configurations.
package ha

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	px2 "github.com/hctamu/pulumi-pve/provider/px"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Ensure Ha implements the required interfaces
var (
	_ = (infer.CustomResource[Inputs, Outputs])((*Ha)(nil))
	_ = (infer.CustomDelete[Outputs])((*Ha)(nil))
	_ = (infer.CustomUpdate[Inputs, Outputs])((*Ha)(nil))
	_ = (infer.CustomRead[Inputs, Outputs])((*Ha)(nil))
	_ = (infer.CustomDiff[Inputs, Outputs])((*Ha)(nil))
)

// Ha represents a Proxmox HA resource
type Ha struct{}

// State represents the state of the HA resource
type State string

const (
	// StateEnabled represents the "ignored" state for HA.
	StateEnabled State = "ignored"
	// StateDisabled represents the "started" state for HA.
	StateDisabled State = "started"
	// StateUnknown represents the "stopped" state for HA.
	StateUnknown State = "stopped"
)

// ValidateState validates the HA state
func (state State) ValidateState(ctx context.Context) (err error) {
	switch state {
	case StateEnabled, StateDisabled, StateUnknown:
		return nil
	default:
		err = fmt.Errorf("invalid state: %s", state)
		p.GetLogger(ctx).Error(err.Error())
		return err
	}
}

// Inputs represents the input properties for the HA resource
type Inputs struct {
	Group      string `pulumi:"group,optional"`
	State      State  `pulumi:"state,optional"`
	ResourceID int    `pulumi:"resourceId"`
}

// Outputs represents the output properties for the HA resource
type Outputs struct {
	Inputs
}

// Create creates a new HA resource
func (ha *Ha) Create(
	ctx context.Context,
	request infer.CreateRequest[Inputs],
) (response infer.CreateResponse[Outputs], err error) {
	inputs := request.Inputs
	preview := request.DryRun

	logger := p.GetLogger(ctx)
	logger.Debugf("Creating ha resource: %v", inputs)
	response = infer.CreateResponse[Outputs]{
		ID:     request.Name,
		Output: Outputs{Inputs: inputs},
	}

	if preview {
		return response, nil
	}

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return response, nil
	}

	err = pxc.CreateHA(ctx, &px2.HaResource{
		Group: inputs.Group,
		State: string(inputs.State),
		Sid:   strconv.Itoa(inputs.ResourceID),
	})

	return response, err
}

// Delete deletes an existing HA resource
func (ha *Ha) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting ha resource: %v", request.State)

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return response, err
	}

	id := request.State.ResourceID
	if err := pxc.DeleteHA(ctx, id); err != nil {
		return response, err
	}
	logger.Debugf("HA resource %v deleted", id)

	return response, nil
}

// Update updates an existing HA resource
func (ha *Ha) Update(
	ctx context.Context,
	request infer.UpdateRequest[Inputs, Outputs],
) (response infer.UpdateResponse[Outputs], err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Updating ha resource: %v", request.ID)
	response.Output = request.State

	if request.DryRun {
		return response, nil
	}

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return response, err
	}

	response.Output.Inputs = request.Inputs

	haResource := px2.HaResource{
		State: string(request.Inputs.State),
	}

	if request.Inputs.Group == "" && request.State.Group != "" {
		haResource.Delete = []string{"group"}
	} else if request.Inputs.Group != "" {
		haResource.Group = request.Inputs.Group
	}
	err = pxc.UpdateHA(ctx, request.State.ResourceID, &haResource)

	return response, err
}

// Read reads the current state of an HA resource
func (ha *Ha) Read(
	ctx context.Context,
	request infer.ReadRequest[Inputs, Outputs],
) (response infer.ReadResponse[Inputs, Outputs], err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Reading ha resource: %v", request.ID)
	response.Inputs = request.Inputs
	response.State = request.State
	response.ID = request.ID

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return response, err
	}

	var haResource *px2.HaResource

	if haResource, err = pxc.GetHA(ctx, request.Inputs.ResourceID); err != nil {
		return response, err
	}

	response.Inputs.Group = haResource.Group
	response.Inputs.State = State(haResource.State)
	response.Inputs.ResourceID = request.State.ResourceID

	response.State.Group = haResource.Group
	response.State.State = State(haResource.State)
	response.State.ResourceID = request.State.ResourceID

	return response, nil
}

// Diff computes the difference between the desired and actual state of an HA resource
func (ha *Ha) Diff(
	ctx context.Context,
	request infer.DiffRequest[Inputs, Outputs],
) (response infer.DiffResponse, err error) {
	diff := map[string]p.PropertyDiff{}
	if request.State.ResourceID != request.Inputs.ResourceID {
		diff["resourceId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if request.State.Group != request.Inputs.Group {
		diff["group"] = p.PropertyDiff{Kind: p.Update}
	}

	if request.State.State != request.Inputs.State {
		diff["state"] = p.PropertyDiff{Kind: p.Update}
	}

	response = p.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}

	return response, nil
}

// Annotate adds descriptions to the HA resource and its properties
func (ha *Ha) Annotate(a infer.Annotator) {
	a.Describe(
		ha,
		"A Proxmox HA resource that manages the HA configuration of a virtual machine in the Proxmox VE.",
	)
}

// Annotate adds descriptions to the Input properties for documentation and schema generation.
func (inputs *Inputs) Annotate(a infer.Annotator) {
	a.Describe(&inputs.Group, "The HA group identifier.")
	a.Describe(
		&inputs.ResourceID,
		"The ID of the virtual machine that will be managed by HA (required).",
	)
	a.Describe(&inputs.State, "The state of the HA resource (default: started).")
	a.SetDefault(&inputs.State, "started")
}
