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

// Package sdnapply provides a resource for applying pending SDN configuration changes in Proxmox VE.
package sdnapply

import (
	"context"
	"errors"
	"time"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// Ensure SDNApply implements the required interfaces
var (
	_ = infer.CustomResource[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs]((*SDNApply)(nil))
	_ = infer.CustomDelete[proxmox.SDNApplyOutputs]((*SDNApply)(nil))
	_ = infer.CustomUpdate[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs]((*SDNApply)(nil))
	_ = infer.CustomRead[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs]((*SDNApply)(nil))
	_ = infer.Annotated((*SDNApply)(nil))
)

// SDNApply represents a Proxmox SDN apply resource.
type SDNApply struct {
	SDNOps proxmox.SDNOperations
}

const (
	defaultLockRetryTimeout = 60 * time.Second
)

// applyWithLock acquires the SDN lock with retries, then applies pending changes,
// releasing the lock atomically.
func (sdnApply *SDNApply) applyWithLock(ctx context.Context, inputs proxmox.SDNApplyInputs) error {
	retryTimeout := time.Duration(inputs.RetryTimeoutSeconds) * time.Second
	if retryTimeout <= 0 {
		retryTimeout = defaultLockRetryTimeout
	}

	token, err := sdnApply.SDNOps.Lock(ctx, retryTimeout)
	if err != nil {
		return err
	}

	return sdnApply.SDNOps.Apply(ctx, token, true)
}

// Create applies pending SDN configuration changes via PUT /cluster/sdn.
func (sdnApply *SDNApply) Create(
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
func (sdnApply *SDNApply) Delete(
	ctx context.Context,
	_ infer.DeleteRequest[proxmox.SDNApplyOutputs],
) (infer.DeleteResponse, error) {
	p.GetLogger(ctx).Debugf("SDN apply delete (no-op)")
	return infer.DeleteResponse{}, nil
}

// Update re-applies SDN configuration changes whenever any trigger value changes.
func (sdnApply *SDNApply) Update(
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
func (sdnApply *SDNApply) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs],
) (infer.ReadResponse[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs], error) {
	p.GetLogger(ctx).Debugf("SDN apply read (no-op)")
	return infer.ReadResponse[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs](request), nil
}

// Annotate adds a description to the SDNApply resource.
func (sdnApply *SDNApply) Annotate(a infer.Annotator) {
	a.Describe(
		sdnApply,
		"Applies pending SDN configuration changes in Proxmox VE via PUT /cluster/sdn. "+
			"Re-runs whenever any trigger value changes.",
	)
}
