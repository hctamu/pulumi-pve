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

// Package vm implements the Pulumi resource for Proxmox virtual machines.
package vm

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"time"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

// disksInputName is the pulumi property name used for disk diff tracking.
const disksInputName = "disks"

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
		return response, errors.New("client not configured")
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

	vmID := *request.Inputs.VMID

	if request.Inputs.Clone != nil {
		// Clone flow: clone source VM, reconcile disks, then apply config.
		if err := vm.VMOps.CloneVM(ctx, request.Inputs); err != nil {
			l.Errorf("error cloning VM: %v", err)
			return response, err
		}

		reconciledInputs := request.Inputs

		if err := vm.reconcileDisksAfterClone(ctx, &reconciledInputs); err != nil {
			l.Errorf("error reconciling disks after clone: %v", err)
			return response, err
		}

		timeout := time.Duration(request.Inputs.Clone.Timeout) * time.Second
		if err := vm.VMOps.ApplyConfig(ctx, vmID, request.Inputs.Node, reconciledInputs, timeout); err != nil {
			l.Errorf("error applying config to cloned VM: %v", err)
			return response, err
		}
	} else {
		// New VM flow: create directly.
		if err := vm.VMOps.CreateVM(ctx, request.Inputs); err != nil {
			l.Errorf("error creating VM: %v", err)
			return response, err
		}
	}

	// Read back the VM from the API to capture computed fields (disk FileIDs, etc.)
	stateInputs, err := vm.VMOps.Get(ctx, *request.Inputs.VMID, request.Inputs.Node, request.Inputs.Disks)
	if err != nil {
		l.Errorf("error reading VM %v after creation: %v", *request.Inputs.VMID, err)
		return response, err
	}

	// Build Create output from full API state, preserving computed FileIDs in the stack.
	response.Output = proxmox.VMOutputs{VMInputs: preserveCreateState(stateInputs, request.Inputs)}

	return response, nil
}

// reconcileDisksAfterClone adjusts the cloned VM's disks to match the desired inputs.
// It delegates regular disk reconciliation to reconcileDisks and handles EFI disk separately.
func (vm *VM) reconcileDisksAfterClone(ctx context.Context, inputs *proxmox.VMInputs) error {
	vmID := *inputs.VMID
	node := inputs.Node

	currentDisks, currentEfi, err := vm.VMOps.GetCurrentDisks(ctx, vmID, node)
	if err != nil {
		return fmt.Errorf("failed to get current disks after clone: %w", err)
	}

	if err := vm.reconcileDisks(ctx, vmID, node, inputs.Disks, currentDisks); err != nil {
		return err
	}

	// Handle EFI disk reconciliation.
	if inputs.EfiDisk == nil && currentEfi != nil {
		if err := vm.VMOps.RemoveEfiDisk(ctx, vmID, node); err != nil {
			return fmt.Errorf("failed to remove unwanted EFI disk: %w", err)
		}
	} else if inputs.EfiDisk != nil && currentEfi != nil {
		// Copy file ID so ApplyConfig uses the existing cloned EFI disk.
		inputs.EfiDisk.FileID = currentEfi.FileID
	}

	return nil
}

// reconcileDisks reconciles the desired disk slice against the current live disk map.
// It removes disks absent from desired, resizes disks that have grown, and propagates
// current FileIDs into the desired pointers so that the subsequent config call uses
// existing disk images instead of creating new ones.
//
// DiskShrunk and DiskStorageChanged return errors: Proxmox does not support these
// operations. At Update time they are caught earlier by disksDiff during Diff.
func (vm *VM) reconcileDisks(
	ctx context.Context,
	vmID int,
	node *string,
	desired []*proxmox.Disk,
	currentMap map[string]proxmox.Disk,
) error {
	// Convert value map to pointer slice for CompareDisksByInterface.
	currentSlice := make([]*proxmox.Disk, 0, len(currentMap))
	for iface := range currentMap {
		d := currentMap[iface]
		currentSlice = append(currentSlice, &d)
	}

	changes := proxmox.CompareDisksByInterface(desired, currentSlice)
	for i := range changes {
		change := &changes[i]
		switch change.Type {
		case proxmox.DiskRemoved:
			if err := vm.VMOps.RemoveDisk(ctx, vmID, node, change.Interface); err != nil {
				return fmt.Errorf("failed to remove disk %s: %w", change.Interface, err)
			}
		case proxmox.DiskResized:
			if err := vm.VMOps.ResizeDisk(ctx, vmID, node, change.Interface, change.Desired.Size); err != nil {
				return fmt.Errorf("failed to resize disk %s: %w", change.Interface, err)
			}
			if change.Current != nil && change.Current.FileID != nil && change.Desired != nil {
				change.Desired.FileID = change.Current.FileID
			}
		case proxmox.DiskUnchanged, proxmox.DiskFileIDChanged:
			if change.Current != nil && change.Current.FileID != nil && change.Desired != nil {
				change.Desired.FileID = change.Current.FileID
			}
		case proxmox.DiskAdded:
			// No pre-action; the subsequent config call provisions the new disk.
		case proxmox.DiskShrunk:
			return fmt.Errorf(
				"disk %s: shrinking disks is not supported by Proxmox; "+
					"increase the size or replace the resource",
				change.Interface,
			)
		case proxmox.DiskStorageChanged:
			return fmt.Errorf(
				"disk %s: storage migration is not supported yet; "+
					"recreate the disk on the target storage",
				change.Interface,
			)
		}
	}
	return nil
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

	stateInputs, err := vm.VMOps.Get(ctx, *vmID, request.Inputs.Node, request.Inputs.Disks)
	if err != nil {
		l.Errorf("Error reading VM %v: %v", *vmID, err)
		return infer.ReadResponse[proxmox.VMInputs, proxmox.VMOutputs]{}, err
	}

	preservedInputs := preserveInputs(stateInputs, request.Inputs)

	response := infer.ReadResponse[proxmox.VMInputs, proxmox.VMOutputs]{
		ID:     request.ID,
		Inputs: preservedInputs,
		State:  proxmox.VMOutputs{VMInputs: stateInputs},
	}

	// Preserve clone info from prior state (not derivable from VM config).
	if request.State.Clone != nil && response.State.Clone == nil {
		response.State.Clone = request.State.Clone
	}

	// Preserve user-specified zero-value fields in state output so the state file
	// records user intent for fields the API cannot distinguish from "not set".
	if request.Inputs.Balloon != nil && response.State.Balloon == nil {
		response.State.Balloon = request.Inputs.Balloon
	}
	if request.Inputs.Autostart != nil && response.State.Autostart == nil {
		response.State.Autostart = request.Inputs.Autostart
	}
	if request.Inputs.Template != nil && response.State.Template == nil {
		response.State.Template = request.Inputs.Template
	}
	if request.Inputs.CPU != nil && response.State.CPU != nil &&
		request.Inputs.CPU.Numa != nil && response.State.CPU.Numa == nil {
		cpu := *response.State.CPU
		cpu.Numa = request.Inputs.CPU.Numa
		response.State.CPU = &cpu
	}

	l.Debugf("VM read complete: %v", stateInputs.VMID)
	return response, nil
}

// preserveInputs computes user-visible inputs from API state by clearing computed
// fields that the user did not explicitly supply.
//
//   - VMID and Node are cleared when the user omitted them (they are computed by Proxmox).
//   - Disk FileIDs are cleared for disks the user already had without a FileID.
//   - EFI disk FileID is cleared when the user supplied an EFI disk without a FileID.
//   - Newly discovered disks/EFI (not present in userInputs) retain their FileIDs.
func preserveInputs(state, userInputs proxmox.VMInputs) proxmox.VMInputs {
	return applyPreservation(state, userInputs, true)
}

// preserveCreateState builds the output state for Create by keeping the full API state
// (including computed disk/EFI FileIDs) while applying user-intent corrections for
// fields the API cannot represent (clone info, zero-value fields, tag ordering).
func preserveCreateState(state, userInputs proxmox.VMInputs) proxmox.VMInputs {
	return applyPreservation(state, userInputs, false)
}

// applyPreservation is the shared implementation for preserveInputs and preserveCreateState.
// When clearComputed is true, VMID/Node and disk FileIDs are cleared for fields the user
// did not explicitly provide (Read path). When false, all computed values are kept (Create path).
func applyPreservation(state, userInputs proxmox.VMInputs, clearComputed bool) proxmox.VMInputs {
	preserved := state

	if clearComputed {
		if userInputs.VMID == nil {
			preserved.VMID = nil
		}
		if userInputs.Node == nil {
			preserved.Node = nil
		}
	}

	userByInterface := make(map[string]*proxmox.Disk, len(userInputs.Disks))
	for _, d := range userInputs.Disks {
		if d != nil && d.Interface != "" {
			userByInterface[d.Interface] = d
		}
	}
	preservedDisks := make([]*proxmox.Disk, 0, len(state.Disks))
	for _, disk := range state.Disks {
		if disk == nil {
			continue
		}
		preservedDisk := *disk
		if clearComputed {
			if userDisk, ok := userByInterface[disk.Interface]; ok && userDisk.FileID == nil {
				preservedDisk.FileID = nil
			}
		}
		preservedDisks = append(preservedDisks, &preservedDisk)
	}
	preserved.Disks = preservedDisks

	if clearComputed && preserved.EfiDisk != nil && userInputs.EfiDisk != nil && userInputs.EfiDisk.FileID == nil {
		efi := *preserved.EfiDisk
		efi.FileID = nil
		preserved.EfiDisk = &efi
	}

	// When the API returns the same set of tags (just alphabetically reordered by Proxmox),
	// preserve the user's original ordering so that refreshes don't trigger phantom diffs.
	if !utils.StringSliceChanged(state.Tags, userInputs.Tags) {
		preserved.Tags = userInputs.Tags
	}

	// Normalize empty tags to nil so state matches what the API reports.
	if len(preserved.Tags) == 0 {
		preserved.Tags = nil
	}

	preserved.Clone = userInputs.Clone // Clone info is not returned by API, always preserve from user inputs

	// Preserve user-specified values for fields where the Proxmox API cannot
	// distinguish "explicitly set to zero/false" from "not set at all" (fields use
	// int with omitempty or the adapter's intOrNil/> 0 checks return nil for zero).
	// We only fill in the user's value when the API returned nil, so that non-zero
	// drift (e.g. someone changed balloon from 512→256) is still detected.
	if userInputs.Balloon != nil && preserved.Balloon == nil {
		preserved.Balloon = userInputs.Balloon
	}
	if userInputs.Autostart != nil && preserved.Autostart == nil {
		preserved.Autostart = userInputs.Autostart
	}
	if userInputs.Template != nil && preserved.Template == nil {
		preserved.Template = userInputs.Template
	}

	if userInputs.CPU != nil && preserved.CPU != nil {
		if userInputs.CPU.Numa != nil && preserved.CPU.Numa == nil {
			cpu := *preserved.CPU
			cpu.Numa = userInputs.CPU.Numa
			preserved.CPU = &cpu
		}
	}

	return preserved
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

	// Get current live disks and reconcile: remove deleted disks, resize grown disks,
	// and propagate live FileIDs into inputs so UpdateConfig reuses existing disks.
	currentDisks, _, err := vm.VMOps.GetCurrentDisks(ctx, *vmID, request.Inputs.Node)
	if err != nil {
		return response, fmt.Errorf("failed to get current disks: %w", err)
	}
	if err := vm.reconcileDisks(ctx, *vmID, request.Inputs.Node, request.Inputs.Disks, currentDisks); err != nil {
		return response, err
	}

	// Remove EFI disk if the user removed it from inputs but it exists in state.
	if request.Inputs.EfiDisk == nil && request.State.EfiDisk != nil {
		if err := vm.VMOps.RemoveEfiDisk(ctx, *vmID, request.Inputs.Node); err != nil {
			return response, fmt.Errorf("failed to remove EFI disk: %w", err)
		}
	}

	if err := vm.VMOps.UpdateConfig(ctx, *vmID, request.Inputs.Node, request.Inputs, request.State.VMInputs); err != nil {
		return response, err
	}

	// Re-read live state to capture computed fields (e.g. filenames for newly-added disks).
	liveOutput, err := vm.readCurrentState(ctx, request.ID, request.Inputs)
	if err != nil {
		l.Errorf("error reading VM after update: %v", err)
		return response, err
	}
	response.Output = liveOutput
	return response, nil
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

// disksDiff compares desired and current disk slices using interface-based identity.
// It returns a map of property diffs keyed by "disks.<interface>", and returns an
// error at Diff time for unsupported operations (shrink, storage migration).
func disksDiff(inputDisks, stateDisks []*proxmox.Disk) (map[string]p.PropertyDiff, error) {
	changes := proxmox.CompareDisksByInterface(inputDisks, stateDisks)
	diffs := make(map[string]p.PropertyDiff)

	for _, change := range changes {
		key := disksInputName + "." + change.Interface
		switch change.Type {
		case proxmox.DiskAdded:
			diffs[key] = p.PropertyDiff{Kind: p.Add}
		case proxmox.DiskRemoved:
			diffs[key] = p.PropertyDiff{Kind: p.Delete}
		case proxmox.DiskResized:
			diffs[key] = p.PropertyDiff{Kind: p.Update}
		case proxmox.DiskFileIDChanged:
			diffs[key] = p.PropertyDiff{Kind: p.Update}
		case proxmox.DiskShrunk:
			return nil, fmt.Errorf(
				"disk %s: shrinking disks is not supported by Proxmox; "+
					"increase the size or replace the resource",
				change.Interface,
			)
		case proxmox.DiskStorageChanged:
			return nil, fmt.Errorf(
				"disk %s: storage migration is not supported yet; "+
					"recreate the disk on the target storage",
				change.Interface,
			)
		case proxmox.DiskUnchanged:
			// no diff
		}
	}

	return diffs, nil
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
		case name == disksInputName:
			inputDisks, okIn := inField.Interface().([]*proxmox.Disk)
			stateDisks, okState := stateField.Interface().([]*proxmox.Disk)
			if okIn && okState {
				diskDiffs, err := disksDiff(inputDisks, stateDisks)
				if err != nil {
					return p.DiffResponse{}, err
				}
				maps.Copy(diff, diskDiffs)
			}
		case inField.Kind() == reflect.Slice || stateField.Kind() == reflect.Slice:
			// Handle remaining slices (e.g. Tags []string)
			propertyDiff = compareSliceFields(name, inField, stateField)
		case inField.Kind() == reflect.Pointer || stateField.Kind() == reflect.Pointer:
			// Handle pointer fields with special cases
			propertyDiff = comparePointerFields(name, inField, stateField, computed)
		default:
			// Handle plain value types (string, int, bool, …)
			if !reflect.DeepEqual(inField.Interface(), stateField.Interface()) {
				propertyDiff = &p.PropertyDiff{Kind: p.Update}
			}
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
// Disk slices are handled separately via disksDiff() in Diff(). Returns nil if no difference.
func compareSliceFields(name string, inField, stateField reflect.Value) *p.PropertyDiff {
	// Compare tags order-insensitively: Proxmox returns tags sorted alphabetically regardless
	// of the order the user specified, so ["web","prod"] and ["prod","web"] are the same set.
	if name == "tags" {
		inputTags, okIn := inField.Interface().([]string)
		stateTags, okState := stateField.Interface().([]string)
		if okIn && okState {
			if utils.StringSliceChanged(inputTags, stateTags) {
				return &p.PropertyDiff{Kind: p.Update}
			}
			return nil
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
