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

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Ensure HA implements the required interfaces
var (
	_ = (infer.CustomResource[proxmox.HAInputs, proxmox.HAOutputs])((*HA)(nil))
	_ = (infer.CustomDelete[proxmox.HAOutputs])((*HA)(nil))
	_ = (infer.CustomUpdate[proxmox.HAInputs, proxmox.HAOutputs])((*HA)(nil))
	_ = (infer.CustomRead[proxmox.HAInputs, proxmox.HAOutputs])((*HA)(nil))
	_ = infer.Annotated((*HA)(nil))
)

// HA represents a Proxmox HA resource
type HA struct {
	HAOps proxmox.HAOperations
}

// Create creates a new HA resource
func (ha *HA) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.HAInputs],
) (response infer.CreateResponse[proxmox.HAOutputs], err error) {
	inputs := request.Inputs
	preview := request.DryRun

	logger := p.GetLogger(ctx)
	logger.Debugf("Creating ha resource: %v", inputs)
	response = infer.CreateResponse[proxmox.HAOutputs]{
		ID:     request.Name,
		Output: proxmox.HAOutputs{HAInputs: inputs},
	}

	if preview {
		return response, nil
	}

	if ha.HAOps == nil {
		err = fmt.Errorf("HAOperations not configured")
		return response, err
	}

	err = ha.HAOps.Create(ctx, inputs)

	return response, err
}

// Delete deletes an existing HA resource
func (ha *HA) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.HAOutputs],
) (response infer.DeleteResponse, err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting ha resource: %v", request.State)

	if ha.HAOps == nil {
		return response, fmt.Errorf("HAOperations not configured")
	}

	id := request.State.ResourceID
	if err := ha.HAOps.Delete(ctx, id); err != nil {
		return response, err
	}
	logger.Debugf("HA resource %v deleted", id)

	return response, nil
}

// Update updates an existing HA resource
func (ha *HA) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.HAInputs, proxmox.HAOutputs],
) (response infer.UpdateResponse[proxmox.HAOutputs], err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Updating ha resource: %v", request.ID)
	response.Output = request.State

	if request.DryRun {
		return response, nil
	}

	if ha.HAOps == nil {
		err = fmt.Errorf("HAOperations not configured")
		return response, err
	}

	response.Output.HAInputs = request.Inputs

	err = ha.HAOps.Update(ctx, request.State.ResourceID, request.Inputs, request.State)

	return response, err
}

// Read reads the current state of an HA resource
func (ha *HA) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.HAInputs, proxmox.HAOutputs],
) (response infer.ReadResponse[proxmox.HAInputs, proxmox.HAOutputs], err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Reading ha resource: %v", request.ID)
	response.Inputs = request.Inputs
	response.State = request.State
	response.ID = request.ID

	if ha.HAOps == nil {
		err = fmt.Errorf("HAOperations not configured")
		return response, err
	}

	var outputs *proxmox.HAOutputs

	if outputs, err = ha.HAOps.Get(ctx, request.Inputs.ResourceID); err != nil {
		return response, err
	}

	response.Inputs = outputs.HAInputs
	response.State = *outputs

	return response, nil
}

// Annotate adds descriptions to the HA resource and its properties
func (ha *HA) Annotate(a infer.Annotator) {
	a.Describe(
		ha,
		"A Proxmox HA resource that manages the HA configuration of a virtual machine in the Proxmox VE.",
	)
}
