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

// Package pool provides resources for managing Proxmox pools.
package pool

import (
	"context"
	"errors"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Ensure Pool implements the required interfaces
var (
	_ = (infer.CustomResource[proxmox.PoolInputs, proxmox.PoolOutputs])((*Pool)(nil))
	_ = (infer.CustomDelete[proxmox.PoolOutputs])((*Pool)(nil))
	_ = (infer.CustomUpdate[proxmox.PoolInputs, proxmox.PoolOutputs])((*Pool)(nil))
	_ = (infer.CustomRead[proxmox.PoolInputs, proxmox.PoolOutputs])((*Pool)(nil))
	_ = (infer.CustomDiff[proxmox.PoolInputs, proxmox.PoolOutputs])((*Pool)(nil))
	_ = infer.Annotated((*Pool)(nil))
)

// Pool represents a Proxmox pool resource
type Pool struct {
	PoolOps proxmox.PoolOperations
}

// Create is used to create a new pool resource
func (pool *Pool) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.PoolInputs],
) (response infer.CreateResponse[proxmox.PoolOutputs], err error) {
	inputs := request.Inputs
	preview := request.DryRun

	logger := p.GetLogger(ctx)
	logger.Debugf("Creating pool resource: %v", inputs)

	response = infer.CreateResponse[proxmox.PoolOutputs]{
		ID:     request.Name,
		Output: proxmox.PoolOutputs{PoolInputs: inputs},
	}

	if preview {
		return response, nil
	}

	if pool.PoolOps == nil {
		err = errors.New("PoolOperations not configured")
		return response, err
	}

	err = pool.PoolOps.Create(ctx, inputs)

	return response, err
}

// Read is used to read the state of a pool resource
func (pool *Pool) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.PoolInputs, proxmox.PoolOutputs],
) (response infer.ReadResponse[proxmox.PoolInputs, proxmox.PoolOutputs], err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf(
		"Read called for Pool with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	response.ID = request.ID
	response.Inputs = request.Inputs
	response.State = request.State

	if pool.PoolOps == nil {
		err = errors.New("PoolOperations not configured")
		return response, err
	}

	if request.ID == "" {
		logger.Warningf("Missing Pool ID")
		err = errors.New("missing pool ID")
		return response, err
	}

	var outputs *proxmox.PoolOutputs

	if outputs, err = pool.PoolOps.Get(ctx, request.Inputs.Name); err != nil {
		return response, err
	}

	response.Inputs = outputs.PoolInputs
	response.State = *outputs

	logger.Debugf("Returning updated state: %+v", response.State)
	return response, nil
}

// Delete is used to delete a pool resource
func (pool *Pool) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.PoolOutputs],
) (response infer.DeleteResponse, err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting pool resource: %v", request.State)

	if pool.PoolOps == nil {
		return response, errors.New("PoolOperations not configured")
	}

	if err := pool.PoolOps.Delete(ctx, request.State.Name); err != nil {
		return response, err
	}
	logger.Debugf("Pool resource %v deleted", request.State.Name)

	return response, nil
}

// Update is used to update a pool resource
func (pool *Pool) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.PoolInputs, proxmox.PoolOutputs],
) (response infer.UpdateResponse[proxmox.PoolOutputs], err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Updating pool resource: %v", request.ID)

	response.Output = request.State

	if request.DryRun {
		return response, nil
	}

	if pool.PoolOps == nil {
		err = errors.New("PoolOperations not configured")
		return response, err
	}

	response.Output.PoolInputs = request.Inputs

	err = pool.PoolOps.Update(ctx, request.State.Name, request.Inputs)

	return response, err
}

// Diff is used to compute the difference between the current state and the desired state of a pool resource
func (pool *Pool) Diff(
	_ context.Context,
	request infer.DiffRequest[proxmox.PoolInputs, proxmox.PoolOutputs],
) (response infer.DiffResponse, err error) {
	diff := map[string]p.PropertyDiff{}
	if request.Inputs.Name != request.State.Name {
		diff["name"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if request.Inputs.Comment != request.State.Comment {
		diff["comment"] = p.PropertyDiff{Kind: p.Update}
	}

	response = p.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}

	return response, nil
}

// Annotate is used to annotate the pool resource
// This is used to provide documentation for the resource in the Pulumi schema
// and to provide default values for the resource properties.
func (pool *Pool) Annotate(a infer.Annotator) {
	a.Describe(
		pool,
		"A Proxmox pool resource that groups virtual machines under a common pool in the Proxmox VE.",
	)
}
