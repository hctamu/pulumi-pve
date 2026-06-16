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

// Package vxlanzone provides a resource for managing Proxmox SDN VXLAN zones.
package vxlanzone

import (
	"context"
	"errors"
	"reflect"
	"regexp"
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
	_ = infer.CustomCheck[proxmox.VxlanZoneInputs]((*VxlanZone)(nil))
	_ = infer.CustomDiff[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]((*VxlanZone)(nil))
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

// Diff computes the diff between old and new inputs, treating nodes and peers as
// order-insensitive sets (Proxmox returns them in lexical order regardless of input order).
func (sdnVxlanZone *VxlanZone) Diff(
	ctx context.Context,
	request infer.DiffRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs],
) (p.DiffResponse, error) {
	p.GetLogger(ctx).Debugf("Diff SDN VXLAN zone: %s", request.ID)

	in := request.Inputs
	st := request.State.VxlanZoneInputs

	diff := map[string]p.PropertyDiff{}

	if in.Name != st.Name {
		diff["name"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if !ptrEqual(in.Fabric, st.Fabric) {
		diff["fabric"] = p.PropertyDiff{Kind: p.Update}
	}
	if utils.StringSliceChanged(in.Peers, st.Peers) {
		diff["peers"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrEqual(in.MTU, st.MTU) {
		diff["mtu"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrEqual(in.VXLANPort, st.VXLANPort) {
		diff["vxlanPort"] = p.PropertyDiff{Kind: p.Update}
	}
	if utils.StringSliceChanged(in.Nodes, st.Nodes) {
		diff["nodes"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrEqual(in.DNS, st.DNS) {
		diff["dns"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrEqual(in.DNSZone, st.DNSZone) {
		diff["dnsZone"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrEqual(in.ReverseDNS, st.ReverseDNS) {
		diff["reverseDns"] = p.PropertyDiff{Kind: p.Update}
	}
	if !ptrEqual(in.IPAM, st.IPAM) {
		diff["ipam"] = p.PropertyDiff{Kind: p.Update}
	}

	return p.DiffResponse{
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
		DeleteBeforeReplace: true,
	}, nil
}

// ptrEqual returns true when two comparable pointer values are equal (both nil, or both
// non-nil with equal dereferenced values).
func ptrEqual[T comparable](a, b *T) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// Check validates zone name format and that exactly one of fabric or peers is provided.
func (sdnVxlanZone *VxlanZone) Check(
	ctx context.Context,
	req infer.CheckRequest,
) (infer.CheckResponse[proxmox.VxlanZoneInputs], error) {
	inputs, failures, err := infer.DefaultCheck[proxmox.VxlanZoneInputs](ctx, req.NewInputs)
	if err != nil || len(failures) > 0 {
		return infer.CheckResponse[proxmox.VxlanZoneInputs]{Inputs: inputs, Failures: failures}, err
	}

	if nameFailure := validateZoneName(inputs.Name); nameFailure != nil {
		failures = append(failures, *nameFailure)
	}

	if inputs.MTU != nil && (*inputs.MTU <= 0 || *inputs.MTU >= 9000) {
		failures = append(failures, p.CheckFailure{
			Property: "mtu",
			Reason:   "mtu must be a positive number less than 9000",
		})
	}

	if inputs.VXLANPort != nil && (*inputs.VXLANPort < 1 || *inputs.VXLANPort > 65535) {
		failures = append(failures, p.CheckFailure{
			Property: "vxlanPort",
			Reason:   "vxlanPort must be a valid port number between 1 and 65535",
		})
	}

	hasFabric := inputs.Fabric != nil && *inputs.Fabric != ""
	hasPeers := len(inputs.Peers) > 0

	if hasFabric == hasPeers {
		failures = append(failures, p.CheckFailure{
			Property: "peers",
			Reason:   "exactly one of fabric or peers must be provided, but not both",
		})
	}

	return infer.CheckResponse[proxmox.VxlanZoneInputs]{Inputs: inputs, Failures: failures}, nil
}

// zoneNameRe matches a valid zone name: starts with a letter, followed by letters/digits, max 8 chars.
var zoneNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]{0,7}$`)

// validateZoneName returns a CheckFailure if name does not start with a letter,
// contains non-alphanumeric characters, or exceeds 8 characters.
func validateZoneName(name string) *p.CheckFailure {
	if !zoneNameRe.MatchString(name) {
		return &p.CheckFailure{
			Property: "name",
			Reason:   "zone name must start with a letter, contain only letters and numbers, and be at most 8 characters",
		}
	}
	return nil
}

// Annotate adds a description to the VxlanZone resource.
func (sdnVxlanZone *VxlanZone) Annotate(a infer.Annotator) {
	a.Describe(
		sdnVxlanZone,
		"A Proxmox SDN VXLAN zone resource managed via /cluster/sdn/zones.",
	)
}
