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

// Package sdn provides resources for managing Proxmox SDN (Software-Defined Network) configurations.
package sdn

import (
	"context"
	"errors"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// Ensure Vnet implements the required interfaces.
var (
	_ = (infer.CustomResource[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs])((*Vnet)(nil))
	_ = (infer.CustomDelete[proxmox.SdnVnetOutputs])((*Vnet)(nil))
	_ = (infer.CustomUpdate[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs])((*Vnet)(nil))
	_ = (infer.CustomRead[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs])((*Vnet)(nil))
	_ = (infer.CustomCheck[proxmox.SdnVnetInputs])((*Vnet)(nil))
	_ = infer.Annotated((*Vnet)(nil))
)

// Vnet represents a Proxmox SDN VNet resource.
type Vnet struct {
	SdnVnetOps proxmox.SdnVnetOperations
}

// Check validates the inputs for a VNet resource. Name constraints are enforced here
// so that pulumi preview catches invalid values before any API call is made.
func (vnet *Vnet) Check(
	ctx context.Context,
	req infer.CheckRequest,
) (infer.CheckResponse[proxmox.SdnVnetInputs], error) {
	inputs, failures, err := infer.DefaultCheck[proxmox.SdnVnetInputs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[proxmox.SdnVnetInputs]{}, err
	}
	if nameErr := proxmox.ValidateVnetName(inputs.Vnet); nameErr != nil {
		failures = append(failures, p.CheckFailure{Property: "vnet", Reason: nameErr.Error()})
	}
	return infer.CheckResponse[proxmox.SdnVnetInputs]{
		Inputs:   inputs,
		Failures: failures,
	}, nil
}

// Create creates a new VNet resource.
func (vnet *Vnet) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.SdnVnetInputs],
) (infer.CreateResponse[proxmox.SdnVnetOutputs], error) {
	inputs := request.Inputs
	logger := p.GetLogger(ctx)
	logger.Debugf("Creating SDN VNet resource: %v", inputs)

	response := infer.CreateResponse[proxmox.SdnVnetOutputs]{
		ID:     request.Name,
		Output: proxmox.SdnVnetOutputs{SdnVnetInputs: inputs},
	}

	if request.DryRun {
		return response, nil
	}

	if vnet.SdnVnetOps == nil {
		return response, errors.New("SdnVnetOperations not configured")
	}

	if err := vnet.SdnVnetOps.Create(ctx, inputs); err != nil {
		return response, err
	}

	outputs, err := vnet.SdnVnetOps.Get(ctx, inputs.Vnet)
	if err != nil {
		return response, err
	}
	response.Output = *outputs
	return response, nil
}

// Read reads the current state of a VNet resource.
func (vnet *Vnet) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs],
) (infer.ReadResponse[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Reading SDN VNet resource: %v", request.ID)

	response := infer.ReadResponse[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs](request)

	if vnet.SdnVnetOps == nil {
		return response, errors.New("SdnVnetOperations not configured")
	}

	// During import, Inputs.Vnet is empty; fall back to the resource ID.
	vnetName := request.Inputs.Vnet
	if vnetName == "" {
		vnetName = request.ID
	}

	outputs, err := vnet.SdnVnetOps.Get(ctx, vnetName)
	if err != nil {
		return response, err
	}

	response.Inputs = outputs.SdnVnetInputs
	response.State = *outputs
	return response, nil
}

// Update updates an existing VNet resource.
func (vnet *Vnet) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs],
) (infer.UpdateResponse[proxmox.SdnVnetOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Updating SDN VNet resource: %v", request.ID)

	response := infer.UpdateResponse[proxmox.SdnVnetOutputs]{Output: request.State}

	if request.DryRun {
		return response, nil
	}

	if vnet.SdnVnetOps == nil {
		return response, errors.New("SdnVnetOperations not configured")
	}

	response.Output.SdnVnetInputs = request.Inputs

	if err := vnet.SdnVnetOps.Update(ctx, request.State.Vnet, request.Inputs, request.State); err != nil {
		return response, err
	}

	outputs, err := vnet.SdnVnetOps.Get(ctx, request.State.Vnet)
	if err != nil {
		return response, err
	}
	response.Output = *outputs
	return response, nil
}

// Delete deletes an existing VNet resource.
func (vnet *Vnet) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.SdnVnetOutputs],
) (infer.DeleteResponse, error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting SDN VNet resource: %v", request.State.Vnet)

	var response infer.DeleteResponse

	if vnet.SdnVnetOps == nil {
		return response, errors.New("SdnVnetOperations not configured")
	}

	if err := vnet.SdnVnetOps.Delete(ctx, request.State.Vnet); err != nil {
		return response, err
	}
	return response, nil
}

// Annotate adds a description to the VNet resource.
func (vnet *Vnet) Annotate(a infer.Annotator) {
	a.Describe(
		vnet,
		"A Proxmox SDN VNet resource that defines a per-zone virtual network "+
			"materialized as a bridge on each node after SdnApply runs.",
	)
}
