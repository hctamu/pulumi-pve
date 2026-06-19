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

package sdn

import (
	"context"
	"errors"
	"time"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// Ensure Apply implements the required interfaces
var (
	_ = infer.CustomResource[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs]((*Apply)(nil))
	_ = infer.CustomDelete[proxmox.SDNApplyOutputs]((*Apply)(nil))
	_ = infer.CustomUpdate[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs]((*Apply)(nil))
	_ = infer.CustomRead[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs]((*Apply)(nil))
	_ = infer.Annotated((*Apply)(nil))
)

// Apply represents a Proxmox SDN apply resource.
type Apply struct {
	SDNOps proxmox.SDNOperations
}

// applyWithLock acquires the SDN lock with retries, then applies pending changes.
func (sdnApply *Apply) applyWithLock(ctx context.Context, inputs proxmox.SDNApplyInputs) error {
	token, err := sdnApply.SDNOps.Lock(
		ctx,
		time.Duration(inputs.LockTimeoutSeconds)*time.Second,
	)
	if err != nil {
		return err
	}

	return sdnApply.SDNOps.Apply(ctx, token, time.Duration(inputs.ApplyTimeoutSeconds)*time.Second)
}

// Create applies pending SDN configuration changes via PUT /cluster/sdn.
func (sdnApply *Apply) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.SDNApplyInputs],
) (infer.CreateResponse[proxmox.SDNApplyOutputs], error) {
	inputs := request.Inputs
	logger := p.GetLogger(ctx)
	logger.Debugf("Applying SDN changes")

	response := infer.CreateResponse[proxmox.SDNApplyOutputs]{
		ID:     request.Name,
		Output: proxmox.SDNApplyOutputs{SDNApplyInputs: inputs},
	}

	if request.DryRun {
		return response, nil
	}

	if sdnApply.SDNOps == nil {
		return response, errors.New("SDNOperations not configured")
	}

	if err := sdnApply.applyWithLock(ctx, inputs); err != nil {
		return response, err
	}

	return response, nil
}

// Delete is a no-op: SDN apply has no server-side delete.
func (sdnApply *Apply) Delete(
	ctx context.Context,
	_ infer.DeleteRequest[proxmox.SDNApplyOutputs],
) (infer.DeleteResponse, error) {
	p.GetLogger(ctx).Debugf("SDN apply delete (no-op)")
	return infer.DeleteResponse{}, nil
}

// Update re-applies SDN configuration changes whenever any trigger value changes.
func (sdnApply *Apply) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs],
) (infer.UpdateResponse[proxmox.SDNApplyOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Re-applying SDN changes")

	response := infer.UpdateResponse[proxmox.SDNApplyOutputs]{
		Output: proxmox.SDNApplyOutputs{SDNApplyInputs: request.Inputs},
	}

	if request.DryRun {
		return response, nil
	}

	if sdnApply.SDNOps == nil {
		return response, errors.New("SDNOperations not configured")
	}

	if err := sdnApply.applyWithLock(ctx, request.Inputs); err != nil {
		return response, err
	}

	return response, nil
}

// Read is a no-op: returns current state unchanged.
func (sdnApply *Apply) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs],
) (infer.ReadResponse[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs], error) {
	p.GetLogger(ctx).Debugf("SDN apply read (no-op)")
	return infer.ReadResponse[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs](request), nil
}

// Annotate adds a description to the Apply resource.
func (sdnApply *Apply) Annotate(a infer.Annotator) {
	a.Describe(
		sdnApply,
		"Applies pending SDN configuration changes in Proxmox VE via PUT /cluster/sdn. "+
			"Re-runs whenever any trigger value changes.",
	)
}
