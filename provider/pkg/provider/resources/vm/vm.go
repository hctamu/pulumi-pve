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

package vm

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// VM represents a Proxmox virtual machine resource.
type VM struct {
	Client proxmox.Client
	VMOps  proxmox.VMOperations
}

var (
	_ = (infer.CustomResource[proxmox.VMInputs, proxmox.VMOutputs])((*VM)(nil))
	_ = (infer.CustomDelete[proxmox.VMOutputs])((*VM)(nil))
	_ = (infer.CustomRead[proxmox.VMInputs, proxmox.VMOutputs])((*VM)(nil))
	_ = (infer.CustomUpdate[proxmox.VMInputs, proxmox.VMOutputs])((*VM)(nil))
	_ = (infer.CustomDiff[proxmox.VMInputs, proxmox.VMOutputs])((*VM)(nil))
	_ = infer.Annotated((*proxmox.VMInputs)(nil))
)

// Create creates a new virtual machine based on the provided inputs.
func (vm *VM) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.VMInputs],
) (infer.CreateResponse[proxmox.VMOutputs], error) {
	l := p.GetLogger(ctx)
	l.Debugf("Create VM: %v", request.Inputs.VMID)

	response := infer.CreateResponse[proxmox.VMOutputs]{
		ID:     request.Name,
		Output: proxmox.VMOutputs{VMInputs: request.Inputs},
	}

	if request.DryRun {
		return response, nil
	}

	if vm.VMOps == nil {
		return response, errors.New("VMOperations not configured")
	}

	if vm.Client == nil {
		return response, errors.New("Client not configured")
	}

	nodeName, err := vm.Client.ResolveNode(ctx, request.Inputs.Node)
	if err != nil {
		l.Errorf("error resolving node: %v", err)
		return response, err
	}
	request.Inputs.Node = &nodeName

	if request.Inputs.VMID == nil {
		vmID, err := vm.Client.NextVMID(ctx)
		if err != nil {
			l.Errorf("error getting next VM ID: %v", err)
			return response, err
		}
		request.Inputs.VMID = &vmID
	}

	vmID, node, err := vm.VMOps.Create(ctx, request.Inputs)
	if err != nil {
		l.Errorf("error creating VM: %v", err)
		return response, err
	}

	request.Inputs.VMID = &vmID
	request.Inputs.Node = &node

	if response.Output, err = readCurrentOutput(ctx, vm, &request, vmID); err != nil {
		l.Errorf("error reading VM after creation: %v", err)
		return response, err
	}

	return response, nil
}

// readCurrentOutput reads the current state of the VM after creation.
func readCurrentOutput(
	ctx context.Context,
	vm *VM,
	request *infer.CreateRequest[proxmox.VMInputs],
	vmID int,
) (proxmox.VMOutputs, error) {
	state := proxmox.VMOutputs{VMInputs: request.Inputs}
	state.VMID = &vmID
	readRequest := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID:     request.Name,
		Inputs: request.Inputs,
		State:  state,
	}

	readResponse, err := vm.Read(ctx, readRequest)
	if err != nil {
		return proxmox.VMOutputs{}, fmt.Errorf("failed to read VM after creation: %v", err)
	}

	currentOutput := readResponse.State
	// Preserve clone configuration (not returned by Read) if user supplied it.
	if request.Inputs.Clone != nil && currentOutput.Clone == nil {
		currentOutput.Clone = request.Inputs.Clone
	}

	return currentOutput, nil
}

// Read reads the state of the virtual machine.
func (vm *VM) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs],
) (infer.ReadResponse[proxmox.VMInputs, proxmox.VMOutputs], error) {
	l := p.GetLogger(ctx)

	// Determine which VMID to use: inputs.vmid if not nil, otherwise state.vmid
	var vmID *int
	switch {
	case request.Inputs.VMID != nil:
		vmID = request.Inputs.VMID
		l.Debugf("Read VM with ID from inputs: %v", *vmID)
	case request.State.VMID != nil:
		vmID = request.State.VMID
		l.Debugf("Read VM with ID from state: %v", *vmID)
	default:
		err := errors.New("VMID is required for reading VM state but is nil in both inputs and state")
		l.Errorf("VMID is nil in both inputs and state during read operation")
		return infer.ReadResponse[proxmox.VMInputs, proxmox.VMOutputs]{}, err
	}

	if vm.VMOps == nil {
		return infer.ReadResponse[proxmox.VMInputs, proxmox.VMOutputs]{}, errors.New("VMOperations not configured")
	}

	stateInputs, preservedInputs, err := vm.VMOps.Get(ctx, *vmID, request.Inputs.Node, request.Inputs)
	if err != nil {
		l.Errorf("Error reading VM %v: %v", *vmID, err)
		return infer.ReadResponse[proxmox.VMInputs, proxmox.VMOutputs]{}, err
	}

	response := infer.ReadResponse[proxmox.VMInputs, proxmox.VMOutputs]{
		ID:     request.ID,
		Inputs: preservedInputs,
		State:  proxmox.VMOutputs{VMInputs: stateInputs},
	}

	// Preserve clone info from prior state (not derivable from VM config).
	if request.State.Clone != nil && response.State.Clone == nil {
		response.State.Clone = request.State.Clone
	}

	l.Debugf("VM read complete: %v", stateInputs.VMID)
	return response, nil
}

// copyMissingDiskFileIDs propagates FileID values from the current state into the
// user inputs when the user omitted them. This prevents unnecessary disk recreation
// during Update operations when the intention is to keep existing disks.
// Matching is done by disk Interface. EFI disk handled separately.
func copyMissingDiskFileIDs(inputs *proxmox.VMInputs, state proxmox.VMInputs) {
	// Regular disks
	if len(inputs.Disks) > 0 && len(state.Disks) > 0 {
		stateByInterface := make(map[string]*proxmox.Disk, len(state.Disks))
		for _, stateDisk := range state.Disks {
			if stateDisk != nil && stateDisk.Interface != "" {
				stateByInterface[stateDisk.Interface] = stateDisk
			}
		}

		for _, inputDisk := range inputs.Disks {
			if inputDisk == nil || inputDisk.Interface == "" {
				continue
			}
			if inputDisk.FileID == nil {
				// Only copy when user did not supply a value
				if stateDisk, ok := stateByInterface[inputDisk.Interface]; ok && stateDisk.FileID != nil {
					inputDisk.FileID = stateDisk.FileID
				}
			}
		}
	}

	// EFI disk
	if inputs.EfiDisk != nil && state.EfiDisk != nil {
		if inputs.EfiDisk.FileID == nil && state.EfiDisk.FileID != nil {
			inputs.EfiDisk.FileID = state.EfiDisk.FileID
		}
	}
}

// buildOutputWithComputedFromState constructs proxmox.VMOutputs for Update by starting from new inputs
// and copying computed values (VMID, Node, disk and EFI FileIDs) from the prior state when
// the user omitted them. proxmox.VMInputs remain as provided by the user; proxmox.VMOutputs carry computed values.
func buildOutputWithComputedFromState(newInputs, oldState proxmox.VMInputs) proxmox.VMOutputs {
	out := proxmox.VMOutputs{VMInputs: newInputs}

	// VMID and Node: copy from state if user did not provide
	if out.VMID == nil && oldState.VMID != nil {
		out.VMID = oldState.VMID
	}
	if out.Node == nil && oldState.Node != nil {
		out.Node = oldState.Node
	}

	// Disks and EFI FileIDs: copy from state when omitted in inputs
	merged := out.VMInputs
	copyMissingDiskFileIDs(&merged, oldState)
	out.VMInputs = merged

	return out
}

// Update updates the state of the virtual machine.
func (vm *VM) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs],
) (infer.UpdateResponse[proxmox.VMOutputs], error) {
	l := p.GetLogger(ctx)
	l.Debugf("Update VM with ID: %v", request.ID)

	vmID := request.State.VMID
	if request.Inputs.VMID == nil {
		request.Inputs.VMID = vmID
	}

	nodeID := request.State.Node
	if request.Inputs.Node == nil {
		request.Inputs.Node = nodeID
	}

	// Propagate missing FileIDs from state to inputs to avoid recreating disks/efi disk
	copyMissingDiskFileIDs(&request.Inputs, request.State.VMInputs)

	// Build outputs by copying computed fields from prior state where inputs omit them
	response := infer.UpdateResponse[proxmox.VMOutputs]{
		Output: buildOutputWithComputedFromState(request.Inputs, request.State.VMInputs),
	}

	if request.DryRun {
		return response, nil
	}

	if vm.VMOps == nil {
		return response, errors.New("VMOperations not configured")
	}

	err := vm.VMOps.Update(ctx, *vmID, request.Inputs.Node, request.Inputs, request.State.VMInputs)
	return response, err
}

// Delete deletes the virtual machine.
func (vm *VM) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.VMOutputs],
) (infer.DeleteResponse, error) {
	l := p.GetLogger(ctx)
	l.Debugf("Deleting VM: %v", request.ID)

	var response infer.DeleteResponse

	if vm.VMOps == nil {
		return response, errors.New("VMOperations not configured")
	}

	err := vm.VMOps.Delete(ctx, *request.State.VMID, request.State.Node)
	return response, err
}

// disksChanged compares two disk slices and returns true if they have meaningful changes.
// FileID differences are ignored when the input FileID is nil (computed field).
func disksChanged(inputDisks, stateDisks []*proxmox.Disk) bool {
	if len(inputDisks) != len(stateDisks) {
		return true
	}

	for i := range inputDisks {
		if i >= len(stateDisks) {
			return true
		}

		input := inputDisks[i]
		state := stateDisks[i]

		// Compare non-FileID fields
		if input.Storage != state.Storage {
			return true
		}
		if input.Size != state.Size {
			return true
		}
		if input.Interface != state.Interface {
			return true
		}

		// Only compare FileID if input explicitly set it (not nil)
		if input.FileID != nil && state.FileID != nil {
			if *input.FileID != *state.FileID {
				return true
			}
		} else if input.FileID != nil && state.FileID == nil {
			// Input has FileID but state doesn't - this is a change
			return true
		}
		// If input.FileID is nil but state.FileID has value, ignore it (computed field)
	}

	return false
}

// compareEfiDiskFields compares two EfiDisk instances and returns a map of property diffs
// for each changed field. This provides granular diff information instead of treating
// the entire efidisk as a single changed property.
func compareEfiDiskFields(inputEfi, stateEfi *proxmox.EfiDisk) map[string]p.PropertyDiff {
	diffs := make(map[string]p.PropertyDiff)

	// Compare Storage
	if inputEfi.Storage != stateEfi.Storage {
		diffs["efidisk.storage"] = p.PropertyDiff{Kind: p.Update}
	}

	// Compare EfiType
	if inputEfi.EfiType != stateEfi.EfiType {
		diffs["efidisk.efitype"] = p.PropertyDiff{Kind: p.Update}
	}

	// Compare PreEnrolledKeys
	switch {
	case inputEfi.PreEnrolledKeys != nil && stateEfi.PreEnrolledKeys != nil:
		if *inputEfi.PreEnrolledKeys != *stateEfi.PreEnrolledKeys {
			diffs["efidisk.preEnrolledKeys"] = p.PropertyDiff{Kind: p.Update}
		}
	case inputEfi.PreEnrolledKeys != nil && stateEfi.PreEnrolledKeys == nil:
		diffs["efidisk.preEnrolledKeys"] = p.PropertyDiff{Kind: p.Update}
	case inputEfi.PreEnrolledKeys == nil && stateEfi.PreEnrolledKeys != nil:
		diffs["efidisk.preEnrolledKeys"] = p.PropertyDiff{Kind: p.Update}
	}

	// Only compare FileID if input explicitly set it (not nil)
	if inputEfi.FileID != nil && stateEfi.FileID != nil {
		if *inputEfi.FileID != *stateEfi.FileID {
			diffs["efidisk.fileId"] = p.PropertyDiff{Kind: p.Update}
		}
	} else if inputEfi.FileID != nil && stateEfi.FileID == nil {
		diffs["efidisk.fileId"] = p.PropertyDiff{Kind: p.Update}
	}
	// If input.FileID is nil but state.FileID has value, ignore it (computed field)

	return diffs
}

// handleEfiDiskDiff processes EfiDisk field comparison.
// Returns a map of property diffs.
func handleEfiDiskDiff(inField, stateField reflect.Value) (map[string]p.PropertyDiff, error) {
	inNil := inField.IsNil()
	stateNil := stateField.IsNil()
	efiDiffs := make(map[string]p.PropertyDiff)

	// EfiDisk added or removed
	if inNil != stateNil {
		efiDiffs[proxmox.EfiDiskInputName] = p.PropertyDiff{Kind: p.Update}
		return efiDiffs, nil
	}

	// Both non-nil: compare with granular diffs
	if !inNil && !stateNil {
		inputEfi, okIn := inField.Interface().(*proxmox.EfiDisk)
		stateEfi, okState := stateField.Interface().(*proxmox.EfiDisk)
		if !okIn || !okState {
			return nil, errors.New("failed to assert EfiDisk types during diff")
		}

		efiDiffs = compareEfiDiskFields(inputEfi, stateEfi)
	}

	return efiDiffs, nil
}

// Diff implements a custom diff so that computed fields like vmId (and node when auto-selected)
// do not force spurious updates when they were not explicitly set by the user. All other
// properties follow a pointer/value comparison semantics: changed value -> Update; for vmId a
// change triggers Replace. Clearing a property (state non-nil, input nil) counts as an update
// unless the property is computed.
func (vm *VM) Diff(
	ctx context.Context,
	request infer.DiffRequest[proxmox.VMInputs, proxmox.VMOutputs],
) (infer.DiffResponse, error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Diff VM: id=%s", request.ID)

	diff := map[string]p.PropertyDiff{}

	// Properties considered computed when absent in user inputs.
	computed := map[string]struct{}{"vmId": {}, "node": {}}

	inVal := reflect.ValueOf(request.Inputs)
	stateVal := reflect.ValueOf(request.State.VMInputs)
	inType := inVal.Type()

	for i := 0; i < inType.NumField(); i++ {
		field := inType.Field(i)
		tag := field.Tag.Get("pulumi")
		if tag == "" {
			continue
		}

		name := getPulumiPropertyName(tag)
		if name == "" {
			continue
		}

		inField := inVal.Field(i)
		stateField := stateVal.Field(i)

		var propertyDiff *p.PropertyDiff

		switch {
		case name == proxmox.EfiDiskInputName:
			var efiDiff map[string]p.PropertyDiff
			efiDiff, diffErr := handleEfiDiskDiff(inField, stateField)
			if diffErr != nil {
				return p.DiffResponse{}, diffErr
			}
			// Handle EfiDisk with granular diff support
			maps.Copy(diff, efiDiff)
		case inField.Kind() == reflect.Slice || stateField.Kind() == reflect.Slice:
			// Handle slices (like Disks []*Disk)
			propertyDiff = compareSliceFields(name, inField, stateField)
		case inField.Kind() == reflect.Pointer || stateField.Kind() == reflect.Pointer:
			// Handle pointer fields with special cases
			propertyDiff = comparePointerFields(name, inField, stateField, computed)
		}

		if propertyDiff != nil {
			diff[name] = *propertyDiff
		}
	}

	response := p.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}
	return response, nil
}

// comparePointerFields compares pointer fields and returns a PropertyDiff if they differ.
// Handles computed fields and special cases. Returns nil if no difference.
func comparePointerFields(
	name string,
	inField, stateField reflect.Value,
	computed map[string]struct{},
) *p.PropertyDiff {
	inNil := inField.IsNil()
	stateNil := stateField.IsNil()

	// Skip diff for computed property when user didn't provide (input nil) but state has a value
	if _, isComputed := computed[name]; isComputed && inNil && !stateNil {
		return nil
	}

	// Clearing property or setting property (nil mismatch) -> update
	if inNil != stateNil {
		return &p.PropertyDiff{Kind: p.Update}
	}

	// Both nil => no change
	if inNil && stateNil {
		return nil
	}

	// Both non-nil: compare underlying values
	if !reflect.DeepEqual(inField.Interface(), stateField.Interface()) {
		// vmId changes require replacement
		kind := p.Update
		if name == "vmId" {
			kind = p.UpdateReplace
		}
		return &p.PropertyDiff{Kind: kind}
	}

	return nil
}

// compareSliceFields compares slice fields and returns a PropertyDiff if they differ.
// Returns nil if no difference found.
func compareSliceFields(name string, inField, stateField reflect.Value) *p.PropertyDiff {
	// Special handling for Disks slice - ignore FileID differences when input FileID is nil
	if name == "disks" {
		inputDisks, okIn := inField.Interface().([]*proxmox.Disk)
		stateDisks, okState := stateField.Interface().([]*proxmox.Disk)
		if okIn && okState && disksChanged(inputDisks, stateDisks) {
			return &p.PropertyDiff{Kind: p.Update}
		}
		if okIn && okState {
			return nil // No changes
		}
	}

	// Compare other slices with DeepEqual
	if !reflect.DeepEqual(inField.Interface(), stateField.Interface()) {
		return &p.PropertyDiff{Kind: p.Update}
	}
	return nil
}

// getPulumiPropertyName extracts the property name from a pulumi struct tag.
// Tags are formatted like "name" or "name,optional".
func getPulumiPropertyName(tag string) string {
	if tag == "" {
		return ""
	}
	// Extract the name before the first comma
	if idx := indexRune(tag, ','); idx != -1 {
		return tag[:idx]
	}
	return tag
}

// indexRune returns the index of the first occurrence of a rune in a string.
func indexRune(s string, r rune) int {
	for i, c := range s {
		if c == r {
			return i
		}
	}
	return -1
}
