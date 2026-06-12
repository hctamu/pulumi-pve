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

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// Ensure SdnApply implements the required interfaces
var (
	_ = (infer.CustomResource[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs])((*SdnApply)(nil))
	_ = (infer.CustomDelete[proxmox.SdnApplyOutputs])((*SdnApply)(nil))
	_ = (infer.CustomUpdate[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs])((*SdnApply)(nil))
	_ = (infer.CustomRead[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs])((*SdnApply)(nil))
	_ = infer.Annotated((*SdnApply)(nil))
)

// SdnApply represents a Proxmox SDN apply resource.
type SdnApply struct {
	SdnOps proxmox.SdnOperations
}

// Create applies pending SDN configuration changes via PUT /cluster/sdn.
func (sdnApply *SdnApply) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.SdnApplyInputs],
) (infer.CreateResponse[proxmox.SdnApplyOutputs], error) {
	inputs := request.Inputs
	logger := p.GetLogger(ctx)
	logger.Debugf("Applying SDN changes")

	response := infer.CreateResponse[proxmox.SdnApplyOutputs]{
		ID:     request.Name,
		Output: proxmox.SdnApplyOutputs{SdnApplyInputs: inputs},
	}

	if request.DryRun {
		return response, nil
	}

	if sdnApply.SdnOps == nil {
		return response, errors.New("SdnOperations not configured")
	}

	if err := sdnApply.SdnOps.Apply(ctx); err != nil {
		return response, err
	}

	return response, nil
}

// Delete is a no-op: SDN apply has no server-side delete.
func (sdnApply *SdnApply) Delete(
	ctx context.Context,
	_ infer.DeleteRequest[proxmox.SdnApplyOutputs],
) (infer.DeleteResponse, error) {
	p.GetLogger(ctx).Debugf("SDN apply delete (no-op)")
	return infer.DeleteResponse{}, nil
}

// Update re-applies SDN configuration changes whenever any trigger value changes.
func (sdnApply *SdnApply) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs],
) (infer.UpdateResponse[proxmox.SdnApplyOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Re-applying SDN changes")

	response := infer.UpdateResponse[proxmox.SdnApplyOutputs]{
		Output: proxmox.SdnApplyOutputs{SdnApplyInputs: request.Inputs},
	}

	if request.DryRun {
		return response, nil
	}

	if sdnApply.SdnOps == nil {
		return response, errors.New("SdnOperations not configured")
	}

	if err := sdnApply.SdnOps.Apply(ctx); err != nil {
		return response, err
	}

	return response, nil
}

// Read is a no-op: returns current state unchanged.
func (sdnApply *SdnApply) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs],
) (infer.ReadResponse[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs], error) {
	p.GetLogger(ctx).Debugf("SDN apply read (no-op)")
	return infer.ReadResponse[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs](request), nil
}

// Annotate adds a description to the SdnApply resource.
func (sdnApply *SdnApply) Annotate(a infer.Annotator) {
	a.Describe(
		sdnApply,
		"Applies pending SDN configuration changes in Proxmox VE via PUT /cluster/sdn. "+
			"Re-runs whenever any trigger value changes.",
	)
}
