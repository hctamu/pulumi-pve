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

	px2 "github.com/hctamu/pulumi-pve/provider/px"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Ensure Ha implements the required interfaces
var _ = (infer.CustomResource[Input, Output])((*Ha)(nil))
var _ = (infer.CustomDelete[Output])((*Ha)(nil))
var _ = (infer.CustomUpdate[Input, Output])((*Ha)(nil))
var _ = (infer.CustomRead[Input, Output])((*Ha)(nil))
var _ = (infer.CustomDiff[Input, Output])((*Ha)(nil))

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

// Input represents the input properties for the HA resource
type Input struct {
	Group      string `pulumi:"group,optional"`
	State      State  `pulumi:"state,optional"`
	ResourceID int    `pulumi:"resourceId"`
}

// Output represents the output properties for the HA resource
type Output struct {
	Input
}

// Create creates a new HA resource
func (ha *Ha) Create(
	ctx context.Context,
	id string,
	inputs Input,
	preview bool,
) (idRet string, output Output, err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Creating ha resource: %v", inputs)
	output = Output{Input: inputs}
	idRet = id

	if preview {
		return idRet, output, nil
	}

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return idRet, output, nil
	}

	err = pxc.CreateHA(ctx, &px2.HaResource{
		Group: inputs.Group,
		State: string(inputs.State),
		Sid:   strconv.Itoa(inputs.ResourceID),
	})

	return idRet, output, err
}

// Delete deletes an existing HA resource
func (ha *Ha) Delete(ctx context.Context, id string, output Output) (err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting ha resource: %v", output)

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return err
	}

	if err := pxc.DeleteHA(ctx, output.ResourceID); err != nil {
		return err
	}
	logger.Debugf("HA resource %v deleted", id)

	return nil
}

// Update updates an existing HA resource
func (ha *Ha) Update(ctx context.Context, id string, haOutput Output, haInput Input, preview bool) (
	haOutputRet Output, err error) {

	logger := p.GetLogger(ctx)
	logger.Debugf("Updating ha resource: %v", id)
	haOutputRet = haOutput

	if preview {
		return haOutputRet, nil
	}

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return haOutputRet, err
	}

	haOutputRet.Input = haInput

	haResource := px2.HaResource{
		State: string(haInput.State),
	}

	if haInput.Group == "" && haOutput.Group != "" {
		haResource.Delete = []string{"group"}
	} else if haInput.Group != "" {
		haResource.Group = haInput.Group
	}
	err = pxc.UpdateHA(ctx, haOutput.ResourceID, &haResource)

	return haOutputRet, err
}

// Read reads the current state of an HA resource
func (ha *Ha) Read(ctx context.Context, id string, inputs Input, output Output) (
	canonicalID string, normalizedInputs Input, normalizedOutputs Output, err error) {

	logger := p.GetLogger(ctx)
	logger.Debugf("Reading ha resource: %v", id)
	normalizedInputs = inputs
	normalizedOutputs = output
	canonicalID = id

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return canonicalID, normalizedInputs, normalizedOutputs, err
	}

	var haResource *px2.HaResource

	if haResource, err = pxc.GetHA(ctx, inputs.ResourceID); err != nil {
		return canonicalID, normalizedInputs, normalizedOutputs, err
	}

	normalizedInputs.Group = haResource.Group
	normalizedInputs.State = State(haResource.State)
	normalizedInputs.ResourceID = output.ResourceID

	normalizedOutputs.Group = haResource.Group
	normalizedOutputs.State = State(haResource.State)
	normalizedOutputs.ResourceID = output.ResourceID

	return canonicalID, normalizedInputs, normalizedOutputs, nil
}

// Diff computes the difference between the desired and actual state of an HA resource
func (ha *Ha) Diff(ctx context.Context, id string, olds Output, news Input) (response p.DiffResponse, err error) {
	diff := map[string]p.PropertyDiff{}
	if olds.ResourceID != news.ResourceID {
		diff["resourceId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if olds.Group != news.Group {
		diff["group"] = p.PropertyDiff{Kind: p.Update}
	}

	if olds.State != news.State {
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
	a.Describe(ha, "A Proxmox HA resource that manages the HA configuration of a virtual machine in the Proxmox VE.")
}

// Annotate adds descriptions to the Input properties for documentation and schema generation.
func (inputs *Input) Annotate(a infer.Annotator) {
	a.Describe(&inputs.Group, "The HA group identifier.")
	a.Describe(&inputs.ResourceID, "The ID of the virtual machine that will be managed by HA (required).")
	a.Describe(&inputs.State, "The state of the HA resource (default: started).")
	a.SetDefault(&inputs.State, "started")
}
