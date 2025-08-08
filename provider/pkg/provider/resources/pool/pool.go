// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package pool provides resources for managing Proxmox pools.
package pool

import (
	"context"
	"errors"
	"fmt"

	"github.com/hctamu/pulumi-pve/provider/px"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	api "github.com/luthermonson/go-proxmox"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Pool represents a Proxmox pool resource.
type Pool struct{}

var _ = (infer.CustomResource[Input, PoolOutput])((*Pool)(nil))
var _ = (infer.CustomDelete[PoolOutput])((*Pool)(nil))
var _ = (infer.CustomRead[Input, PoolOutput])((*Pool)(nil))
var _ = (infer.CustomUpdate[Input, PoolOutput])((*Pool)(nil))
var _ = (infer.CustomDiff[Input, PoolOutput])((*Pool)(nil))

// Input defines the input properties for a Proxmox pool resource.
type Input struct {
	Name    string `pulumi:"name"`
	Comment string `pulumi:"comment,optional"`
}

// Annotate is used to annotate the input and output properties of the resource.
func (args *Input) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "The name of the Proxmox pool.")
	a.SetDefault(&args.Comment, "Default pool comment")
	a.Describe(&args.Comment, "An optional comment for the pool. If not provided, defaults to 'Default pool comment'.")
}

// PoolOutput defines the output properties for a Proxmox pool resource.
type PoolOutput struct {
	Input
}

// Create is used to create a new pool resource
func (pool *Pool) Create(ctx context.Context, id string, inputs Input, preview bool) (
	idRet string, state PoolOutput, err error) {

	idRet = id
	state = PoolOutput{inputs}
	l := p.GetLogger(ctx)
	l.Debugf("Create: %v, %v, %v", id, inputs, state)
	if preview {
		return idRet, state, nil
	}

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return idRet, state, err
	}

	err = pxc.NewPool(ctx, inputs.Name, inputs.Comment)

	return idRet, state, err
}

// Read is used to read the state of a pool resource
func (pool *Pool) Read(ctx context.Context, id string, inputs Input, state PoolOutput) (
	canonicalID string, normalizedInputs Input, normalizedOutput PoolOutput, err error) {

	canonicalID = id
	normalizedInputs = inputs
	l := p.GetLogger(ctx)
	l.Debugf("Read called for Pool with ID: %s, Inputs: %+v, State: %+v", id, inputs, state)

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return canonicalID, normalizedInputs, normalizedOutput, err
	}

	if id == "" {
		l.Warningf("Missing Pool ID")
		err = errors.New("missing pool ID")
		return canonicalID, normalizedInputs, normalizedOutput, err
	}

	var existingPool *api.Pool
	if existingPool, err = pxc.Pool(ctx, normalizedInputs.Name); err != nil {
		err = fmt.Errorf("failed to get pool %s: %v", normalizedInputs.Name, err)
		return canonicalID, normalizedInputs, normalizedOutput, err
	}

	poolName := existingPool.PoolID

	l.Debugf("Successfully fetched pool: %+v", poolName)

	normalizedOutput = PoolOutput{
		Input: Input{
			Name:    poolName,
			Comment: existingPool.Comment,
		},
	}

	normalizedInputs = normalizedOutput.Input

	l.Debugf("Returning updated state: %+v", normalizedOutput)
	return canonicalID, normalizedInputs, normalizedOutput, nil
}

// Delete is used to delete a pool resource
func (pool *Pool) Delete(ctx context.Context, id string, output PoolOutput) (err error) {
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return err
	}

	l := p.GetLogger(ctx)
	l.Debugf("Deleting pool %v", output.Name)

	var existingPool *api.Pool
	if existingPool, err = pxc.Pool(ctx, output.Name); err != nil {
		err = fmt.Errorf("failed to get pool %s: %v", output.Name, err)
		l.Error(err.Error())
		return err
	}

	if err = existingPool.Delete(ctx); err != nil {
		err = fmt.Errorf("failed to delete pool %s: %v", output.Name, err)
		l.Error(err.Error())
		return err
	}

	return nil
}

// Update is used to update a pool resource
func (pool *Pool) Update(ctx context.Context, name string, poolState PoolOutput, poolArgs Input, preview bool) (
	poolStateRet PoolOutput, err error) {

	poolStateRet = poolState
	l := p.GetLogger(ctx)
	l.Debugf("Updating pool: %v", name)

	if preview {
		return poolStateRet, nil
	}

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return poolStateRet, err
	}

	var existingPool *api.Pool
	if existingPool, err = pxc.Pool(ctx, poolState.Name); err != nil {
		err = fmt.Errorf("failed to get pool %s: %v", poolState.Name, err)
		return poolStateRet, err
	}

	poolUpdateOption := &api.PoolUpdateOption{
		Comment: poolArgs.Comment,
	}

	if err = existingPool.Update(ctx, poolUpdateOption); err != nil {
		err = fmt.Errorf("failed to update pool %s: %v", poolState.Name, err)
		return poolStateRet, err
	}

	poolStateRet = PoolOutput{poolArgs}
	return poolStateRet, nil
}

// Diff is used to compute the difference between the current state and the desired state of a pool resource
func (pool *Pool) Diff(
	_ context.Context,
	_ string,
	olds PoolOutput,
	news Input,
) (response p.DiffResponse, err error) {
	diff := map[string]p.PropertyDiff{}
	if news.Name != olds.Name {
		diff["name"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if news.Comment != olds.Comment {
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
