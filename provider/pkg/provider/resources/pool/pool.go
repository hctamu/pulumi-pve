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
	"fmt"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Pool represents a Proxmox pool resource.
type Pool struct{}

var (
	_ = (infer.CustomResource[PoolInputs, PoolOutputs])((*Pool)(nil))
	_ = (infer.CustomDelete[PoolOutputs])((*Pool)(nil))
	_ = (infer.CustomRead[PoolInputs, PoolOutputs])((*Pool)(nil))
	_ = (infer.CustomUpdate[PoolInputs, PoolOutputs])((*Pool)(nil))
	_ = (infer.CustomDiff[PoolInputs, PoolOutputs])((*Pool)(nil))
)

// PoolInputs defines the input properties for a Proxmox pool resource.
type PoolInputs struct {
	Name    string `pulumi:"name"`
	Comment string `pulumi:"comment,optional"`
}

// Annotate is used to annotate the input and output properties of the resource.
func (args *PoolInputs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "The name of the Proxmox pool.")
	a.SetDefault(&args.Comment, "Default pool comment")
	a.Describe(&args.Comment, "An optional comment for the pool. If not provided, defaults to 'Default pool comment'.")
}

// PoolOutputs defines the output properties for a Proxmox pool resource.
type PoolOutputs struct {
	PoolInputs
}

// Create is used to create a new pool resource
func (pool *Pool) Create(ctx context.Context, request infer.CreateRequest[PoolInputs]) (response infer.CreateResponse[PoolOutputs], err error) {
	response.ID = request.Name
	response.Output = PoolOutputs{PoolInputs: request.Inputs}
	l := p.GetLogger(ctx)
	l.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)
	if request.DryRun {
		return response, nil
	}

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return response, err
	}

	err = pxc.NewPool(ctx, request.Inputs.Name, request.Inputs.Comment)

	return response, err
}

// Read is used to read the state of a pool resource
func (pool *Pool) Read(ctx context.Context, request infer.ReadRequest[PoolInputs, PoolOutputs]) (response infer.ReadResponse[PoolInputs, PoolOutputs], err error) {
	response.ID = request.ID
	response.Inputs = request.Inputs
	l := p.GetLogger(ctx)
	l.Debugf("Read called for Pool with ID: %s, Inputs: %+v, State: %+v", request.ID, request.Inputs, request.State)

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return response, err
	}

	if request.ID == "" {
		l.Warningf("Missing Pool ID")
		err = errors.New("missing pool ID")
		return response, err
	}

	var existingPool *api.Pool
	if existingPool, err = pxc.Pool(ctx, response.Inputs.Name); err != nil {
		err = fmt.Errorf("failed to get pool %s: %v", response.Inputs.Name, err)
		return response, err
	}

	poolName := existingPool.PoolID

	l.Debugf("Successfully fetched pool: %+v", poolName)

	response.State = PoolOutputs{
		PoolInputs: PoolInputs{
			Name:    poolName,
			Comment: existingPool.Comment,
		},
	}

	response.Inputs = response.State.PoolInputs

	l.Debugf("Returning updated state: %+v", response.State)
	return response, nil
}

// Delete is used to delete a pool resource
func (pool *Pool) Delete(ctx context.Context, request infer.DeleteRequest[PoolOutputs]) (response infer.DeleteResponse, err error) {
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return response, err
	}

	l := p.GetLogger(ctx)
	l.Debugf("Deleting pool %v", request.State.Name)

	var existingPool *api.Pool
	if existingPool, err = pxc.Pool(ctx, request.State.Name); err != nil {
		err = fmt.Errorf("failed to get pool %s: %v", request.State.Name, err)
		l.Error(err.Error())
		return response, err
	}

	if err = existingPool.Delete(ctx); err != nil {
		err = fmt.Errorf("failed to delete pool %s: %v", request.State.Name, err)
		l.Error(err.Error())
		return response, err
	}

	return response, nil
}

// Update is used to update a pool resource
func (pool *Pool) Update(ctx context.Context, request infer.UpdateRequest[PoolInputs, PoolOutputs]) (response infer.UpdateResponse[PoolOutputs], err error) {
	response.Output = request.State
	l := p.GetLogger(ctx)
	l.Debugf("Updating pool: %v", request.State.Name)

	if request.DryRun {
		return response, nil
	}

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return response, err
	}

	var existingPool *api.Pool
	if existingPool, err = pxc.Pool(ctx, request.State.Name); err != nil {
		err = fmt.Errorf("failed to get pool %s: %v", request.State.Name, err)
		return response, err
	}

	poolUpdateOption := &api.PoolUpdateOption{
		Comment: request.Inputs.Comment,
	}

	if err = existingPool.Update(ctx, poolUpdateOption); err != nil {
		err = fmt.Errorf("failed to update pool %s: %v", request.State.Name, err)
		return response, err
	}

	response.Output = PoolOutputs{request.Inputs}
	return response, nil
}

// Diff is used to compute the difference between the current state and the desired state of a pool resource
func (pool *Pool) Diff(_ context.Context, request infer.DiffRequest[PoolInputs, PoolOutputs]) (response infer.DiffResponse, err error) {
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
	a.Describe(pool, "A Proxmox pool resource that groups virtual machines under a common pool in the Proxmox VE.")
}
