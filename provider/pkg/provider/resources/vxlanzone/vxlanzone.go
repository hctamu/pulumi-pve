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

// Package sdnvxlanzone provides a resource for managing Proxmox SDN VXLAN zones.
package vxlanzone

import (
	"context"
	"errors"
	"reflect"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

// Ensure VxlanZone implements the required interfaces.
var (
	_ = infer.CustomResource[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]((*VxlanZone)(nil))
	_ = infer.CustomDelete[proxmox.VxlanZoneOutputs]((*VxlanZone)(nil))
	_ = infer.CustomUpdate[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]((*VxlanZone)(nil))
	_ = infer.CustomRead[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]((*VxlanZone)(nil))
	_ = infer.Annotated((*VxlanZone)(nil))
)

// VxlanZone represents a Proxmox SDN VXLAN zone resource.
type VxlanZone struct {
	VxlanZoneOps proxmox.VxlanZoneOperations
}

func hasConfiguredVxlanZoneOps(ops proxmox.VxlanZoneOperations) bool {
	if ops == nil {
		return false
	}

	value := reflect.ValueOf(ops)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return false
	}

	return true
}

// Create creates a new VXLAN zone.
func (sdnVxlanZone *VxlanZone) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.VxlanZoneInputs],
) (infer.CreateResponse[proxmox.VxlanZoneOutputs], error) {
	inputs := request.Inputs
	p.GetLogger(ctx).Debugf("Creating SDN VXLAN zone: %s", inputs.Name)

	response := infer.CreateResponse[proxmox.VxlanZoneOutputs]{
		ID:     inputs.Name,
		Output: proxmox.VxlanZoneOutputs{VxlanZoneInputs: inputs},
	}

	if request.DryRun {
		return response, nil
	}

	if !hasConfiguredVxlanZoneOps(sdnVxlanZone.VxlanZoneOps) {
		return response, errors.New("VxlanZoneOperations not configured")
	}

	if err := sdnVxlanZone.VxlanZoneOps.Create(ctx, inputs); err != nil {
		return response, err
	}

	return response, nil
}

// Delete deletes an existing VXLAN zone.
func (sdnVxlanZone *VxlanZone) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.VxlanZoneOutputs],
) (infer.DeleteResponse, error) {
	p.GetLogger(ctx).Debugf("Deleting SDN VXLAN zone: %s", request.State.Name)

	if !hasConfiguredVxlanZoneOps(sdnVxlanZone.VxlanZoneOps) {
		return infer.DeleteResponse{}, errors.New("VxlanZoneOperations not configured")
	}

	name := request.State.Name
	if name == "" {
		name = request.ID
	}

	if err := sdnVxlanZone.VxlanZoneOps.Delete(ctx, name); err != nil {
		return infer.DeleteResponse{}, err
	}

	return infer.DeleteResponse{}, nil
}

// Update updates an existing VXLAN zone.
func (sdnVxlanZone *VxlanZone) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs],
) (infer.UpdateResponse[proxmox.VxlanZoneOutputs], error) {
	p.GetLogger(ctx).Debugf("Updating SDN VXLAN zone: %s", request.State.Name)

	response := infer.UpdateResponse[proxmox.VxlanZoneOutputs]{
		Output: proxmox.VxlanZoneOutputs{VxlanZoneInputs: request.Inputs},
	}

	if request.DryRun {
		return response, nil
	}

	if !hasConfiguredVxlanZoneOps(sdnVxlanZone.VxlanZoneOps) {
		return response, errors.New("VxlanZoneOperations not configured")
	}

	if err := sdnVxlanZone.VxlanZoneOps.Update(ctx, request.State.Name, request.Inputs, request.State); err != nil {
		return response, err
	}

	return response, nil
}

// Read reads the current state of an existing VXLAN zone.
func (sdnVxlanZone *VxlanZone) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs],
) (infer.ReadResponse[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs], error) {
	p.GetLogger(ctx).Debugf("Reading SDN VXLAN zone: %s", request.ID)

	response := infer.ReadResponse[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs](request)

	if !hasConfiguredVxlanZoneOps(sdnVxlanZone.VxlanZoneOps) {
		return response, errors.New("VxlanZoneOperations not configured")
	}

	zoneName := request.Inputs.Name
	if zoneName == "" {
		zoneName = request.ID
	}

	if zoneName == "" {
		return response, nil
	}

	outputs, err := sdnVxlanZone.VxlanZoneOps.Get(ctx, zoneName)
	if err != nil {
		if utils.IsNotFound(err) || strings.Contains(err.Error(), "404 Not Found") {
			response.ID = ""
			response.State = proxmox.VxlanZoneOutputs{}
			return response, nil
		}

		return response, err
	}

	// Proxmox does not return peers on GET, so preserve previous-state peers.
	outputs.Peers = request.State.Peers

	response.Inputs = outputs.VxlanZoneInputs
	response.State = *outputs
	return response, nil
}

// Annotate adds a description to the VxlanZone resource.
func (sdnVxlanZone *VxlanZone) Annotate(a infer.Annotator) {
	a.Describe(
		sdnVxlanZone,
		"A Proxmox SDN VXLAN zone resource managed via /cluster/sdn/zones.",
	)
}
