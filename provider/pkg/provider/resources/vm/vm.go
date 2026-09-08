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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

// VM represents a Proxmox virtual machine resource.
type VM struct {
	Client proxmox.Client
	VMOps  proxmox.VMOperations
}

var (
	_ = infer.CustomResource[proxmox.VMInputs, proxmox.VMOutputs]((*VM)(nil))
	_ = infer.CustomDelete[proxmox.VMOutputs]((*VM)(nil))
	_ = infer.CustomRead[proxmox.VMInputs, proxmox.VMOutputs]((*VM)(nil))
	_ = infer.CustomUpdate[proxmox.VMInputs, proxmox.VMOutputs]((*VM)(nil))
	_ = infer.CustomDiff[proxmox.VMInputs, proxmox.VMOutputs]((*VM)(nil))
	_ = infer.CustomCheck[proxmox.VMInputs]((*VM)(nil))
	_ = infer.CustomStateMigrations[proxmox.VMOutputs]((*VM)(nil))
	_ = infer.Annotated((*proxmox.VMInputs)(nil))
)

type legacyVMInputs struct {
	Name        string           `pulumi:"name"`
	Description *string          `pulumi:"description,optional"`
	Node        *string          `pulumi:"node,optional"`
	VMID        *int             `pulumi:"vmId,optional"        provider:"replaceOnChanges"`
	Hotplug     *string          `pulumi:"hotplug,optional"`
	Template    *int             `pulumi:"template,optional"`
	Autostart   *int             `pulumi:"autostart,optional"`
	Tags        proxmox.TagList  `pulumi:"tags,optional"`
	OSType      *string          `pulumi:"ostype,optional"`
	Machine     *string          `pulumi:"machine,optional"`
	EfiDisk     *proxmox.EfiDisk `pulumi:"efidisk,optional"`
	CPU         *proxmox.CPU     `pulumi:"cpu,optional"`
	Memory      *int             `pulumi:"memory,optional"`
	Balloon     *int             `pulumi:"balloon,optional"`
	Disks       any              `pulumi:"disks"`
	Clone       *proxmox.Clone   `pulumi:"clone,optional"`
}

type legacyVMOutputs struct {
	legacyVMInputs
}

// StateMigrations upgrades legacy VM state where disks were stored as a list.
func (vm *VM) StateMigrations(context.Context) []infer.StateMigrationFunc[proxmox.VMOutputs] {
	return []infer.StateMigrationFunc[proxmox.VMOutputs]{
		infer.StateMigration(migrateLegacyVMOutputsFromPropertyMap),
		infer.StateMigration(migrateLegacyVMOutputs),
	}
}

func migrateLegacyVMOutputsFromPropertyMap(
	ctx context.Context,
	state property.Map,
) (infer.MigrationResult[proxmox.VMOutputs], error) {
	rawState := resource.ToResourcePropertyMap(state).Mappable()
	rawDisks, hasDisks := rawState["disks"]
	if !hasDisks {
		return infer.MigrationResult[proxmox.VMOutputs]{}, nil
	}

	if _, isLegacyList := rawDisks.([]any); !isLegacyList {
		return infer.MigrationResult[proxmox.VMOutputs]{}, nil
	}

	payload, err := json.Marshal(rawState)
	if err != nil {
		return infer.MigrationResult[proxmox.VMOutputs]{}, fmt.Errorf("marshal legacy VM state: %w", err)
	}

	var legacy legacyVMOutputs
	if err := json.Unmarshal(payload, &legacy); err != nil {
		return infer.MigrationResult[proxmox.VMOutputs]{}, fmt.Errorf("decode legacy VM state: %w", err)
	}

	return migrateLegacyVMOutputs(ctx, legacy)
}

func migrateLegacyVMOutputs(
	_ context.Context,
	legacy legacyVMOutputs,
) (infer.MigrationResult[proxmox.VMOutputs], error) {
	legacyDisks, shouldMigrate, err := decodeLegacyDiskList(legacy.Disks)
	if err != nil {
		return infer.MigrationResult[proxmox.VMOutputs]{}, err
	}
	if !shouldMigrate {
		return infer.MigrationResult[proxmox.VMOutputs]{}, nil
	}

	migrated := proxmox.VMOutputs{
		VMInputs: proxmox.VMInputs{
			Name:        legacy.Name,
			Description: legacy.Description,
			Node:        legacy.Node,
			VMID:        legacy.VMID,
			Hotplug:     legacy.Hotplug,
			Template:    legacy.Template,
			Autostart:   legacy.Autostart,
			Tags:        legacy.Tags,
			OSType:      legacy.OSType,
			Machine:     legacy.Machine,
			EfiDisk:     legacy.EfiDisk,
			CPU:         legacy.CPU,
			Memory:      legacy.Memory,
			Balloon:     legacy.Balloon,
			Disks:       proxmox.DiskMapFromSliceByIdentity(legacyDisks),
			Clone:       legacy.Clone,
		},
	}

	return infer.MigrationResult[proxmox.VMOutputs]{Result: &migrated}, nil
}

func decodeLegacyDiskList(rawDisks any) (proxmox.DiskList, bool, error) {
	if rawDisks == nil {
		return nil, false, nil
	}

	switch disks := rawDisks.(type) {
	case proxmox.DiskList:
		return disks, true, nil
	case []any:
		payload, err := json.Marshal(disks)
		if err != nil {
			return nil, false, fmt.Errorf("marshal legacy disk list: %w", err)
		}
		var decoded proxmox.DiskList
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return nil, false, fmt.Errorf("decode legacy disk list: %w", err)
		}
		return decoded, true, nil
	case map[string]any:
		return nil, false, nil
	default:
		payload, err := json.Marshal(disks)
		if err != nil {
			return nil, false, fmt.Errorf("marshal legacy disks value: %w", err)
		}

		var asList proxmox.DiskList
		if err := json.Unmarshal(payload, &asList); err == nil {
			return asList, true, nil
		}

		var asMap map[string]*proxmox.Disk
		if err := json.Unmarshal(payload, &asMap); err == nil {
			return nil, false, nil
		}

		return nil, false, errors.New("unsupported disks shape for state migration")
	}
}

// Check validates VM inputs before any API calls are made. It runs on both
// `pulumi preview` and `pulumi up`, so invalid inputs are rejected early.
func (vm *VM) Check(
	ctx context.Context,
	req infer.CheckRequest,
) (infer.CheckResponse[proxmox.VMInputs], error) {
	inputs, failures, err := infer.DefaultCheck[proxmox.VMInputs](ctx, req.NewInputs)
	if err != nil || len(failures) > 0 {
		return infer.CheckResponse[proxmox.VMInputs]{Inputs: inputs, Failures: failures}, err
	}

	// Parent cpu object defaults are materialized here because infer doesn't apply
	// nested defaults when the parent object is omitted by the user.
	if inputs.CPU == nil {
		inputs.CPU = proxmox.DefaultCPU()
	}

	for _, disk := range inputs.Disks {
		if validationErr := proxmox.ValidateDiskFlags(disk); validationErr != nil {
			failures = append(failures, p.CheckFailure{
				Property: "disks",
				Reason:   validationErr.Error(),
			})
		}
	}

	failures = append(failures, checkDiskNames(inputs.Disks)...)
	failures = append(failures, checkDuplicateDiskInterfaces(inputs.Disks)...)

	oldInputs, _, _ := infer.DefaultCheck[proxmox.VMInputs](context.Background(), req.OldInputs)
	failures = append(failures, checkDiskShrink(inputs.Disks, oldInputs.Disks)...)
	failures = append(failures, checkDiskFileIDConflict(inputs.Disks, oldInputs.Disks)...)
	failures = append(failures, checkDiskInterfaceMoves(inputs.Disks, oldInputs.Disks)...)

	return infer.CheckResponse[proxmox.VMInputs]{Inputs: inputs, Failures: failures}, nil
}

// checkDiskNames validates logical disk names (map keys) are non-empty.
func checkDiskNames(disks proxmox.DiskMap) []p.CheckFailure {
	var failures []p.CheckFailure
	for diskName := range disks {
		if diskName != "" {
			continue
		}
		failures = append(failures, p.CheckFailure{
			Property: "disks",
			Reason:   "disk logical names must be non-empty",
		})
	}
	return failures
}

// checkDuplicateDiskInterfaces returns a CheckFailure for every disk whose Interface
// repeats one already seen. Duplicate interfaces would silently collide when keyed by
// interface during diffing and reconciliation (only the last one survives).
func checkDuplicateDiskInterfaces(disks proxmox.DiskMap) []p.CheckFailure {
	seen := make(map[string]struct{}, len(disks))
	var failures []p.CheckFailure
	for _, disk := range disks {
		if disk == nil {
			continue
		}
		if _, ok := seen[disk.Interface]; ok {
			failures = append(failures, p.CheckFailure{
				Property: "disks",
				Reason: fmt.Sprintf(
					"duplicate disk interface %q: each disk must have a unique interface",
					disk.Interface,
				),
			})
			continue
		}
		seen[disk.Interface] = struct{}{}
	}
	return failures
}

// checkDiskFileIDConflict returns a CheckFailure for every disk whose explicit FileID
// differs from its current (state) FileID. Changing a disk's underlying volume binding
// is not supported; the disk must be recreated instead.
func checkDiskFileIDConflict(desired, current proxmox.DiskMap) []p.CheckFailure {
	var failures []p.CheckFailure
	for diskName, desiredDisk := range desired {
		currentDisk, exists := current[diskName]
		if !exists || desiredDisk == nil || currentDisk == nil {
			continue
		}
		if desiredDisk.FileID != nil && currentDisk.FileID != nil && *desiredDisk.FileID != *currentDisk.FileID {
			failures = append(failures, p.CheckFailure{
				Property: "disks",
				Reason: fmt.Sprintf(
					"disk %s: changing the volume binding (fileID) is not supported; "+
						"remove the fileId field or recreate the disk",
					diskName,
				),
			})
		}
	}
	return failures
}

// checkDiskShrink returns a CheckFailure for every disk whose desired size is smaller than
// its current size. Proxmox does not support shrinking a disk in place.
func checkDiskShrink(desired, current proxmox.DiskMap) []p.CheckFailure {
	var failures []p.CheckFailure
	for diskName, desiredDisk := range desired {
		currentDisk, exists := current[diskName]
		if !exists || desiredDisk == nil || currentDisk == nil {
			continue
		}
		if desiredDisk.Size < currentDisk.Size {
			failures = append(failures, p.CheckFailure{
				Property: "disks",
				Reason: fmt.Sprintf(
					"disk %s: shrinking disks is not supported by Proxmox; "+
						"increase the size or replace the resource",
					diskName,
				),
			})
		}
	}
	return failures
}

// checkDiskInterfaceMoves returns a CheckFailure for every disk where the interface
// changed in an unsafe or unsupported way relative to its current (state) value.
func checkDiskInterfaceMoves(desired, current proxmox.DiskMap) []p.CheckFailure {
	var failures []p.CheckFailure
	for diskName, desiredDisk := range desired {
		currentDisk, exists := current[diskName]
		if !exists || desiredDisk == nil || currentDisk == nil {
			continue
		}
		if desiredDisk.Interface == currentDisk.Interface {
			continue
		}
		// Reject unsafe moves before apply. Update expects the same logical disk to be
		// moved in place, not recreated behind the user's back.
		if err := proxmox.CheckDiskInterfaceMove(
			diskName, currentDisk.Interface, desiredDisk.Interface, desired,
		); err != nil {
			failures = append(failures, p.CheckFailure{
				Property: "disks",
				Reason:   err.Error(),
			})
		}
	}
	return failures
}

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

	currentByName := proxmox.DiskMapFromCurrentInterfaces(currentDisks, inputs.Disks)

	if err := vm.reconcileDisks(ctx, vmID, node, inputs.Disks, currentByName); err != nil {
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

// reconcileDisks reconciles desired disks against current state: removes absent disks,
// resizes grown disks, and propagates FileIDs. Matching is keyed by disk Interface.
type diskMoveAction struct {
	diskName        string
	fromInterface   string
	targetInterface string
}

type diskResizeAction struct {
	diskName    string
	interfaceID string
	sizeGB      int
}

type diskRemoveAction struct {
	diskName    string
	interfaceID string
}

func (vm *VM) reconcileDisks(
	ctx context.Context,
	vmID int,
	node *string,
	desired proxmox.DiskMap,
	current proxmox.DiskMap,
) error {
	currentByName := current

	// Collect actions first so the API calls stay in a Proxmox-safe order.
	var moveActions []diskMoveAction
	var resizeActions []diskResizeAction
	var removeActions []diskRemoveAction

	for diskName, currentDisk := range currentByName {
		desiredDisk, exists := desired[diskName]
		if !exists && currentDisk != nil {
			removeActions = append(removeActions, diskRemoveAction{
				diskName:    diskName,
				interfaceID: currentDisk.Interface,
			})
			continue
		}

		if desiredDisk == nil || currentDisk == nil {
			continue
		}

		if currentDisk.FileID != nil {
			if desiredDisk.FileID == nil {
				desiredDisk.FileID = currentDisk.FileID
			} else if *desiredDisk.FileID != *currentDisk.FileID {
				return fmt.Errorf(
					"disk %s: changing the volume binding (fileID) is not supported; "+
						"remove the fileId field or recreate the disk",
					diskName,
				)
			}
		}

		if desiredDisk.Size < currentDisk.Size {
			return fmt.Errorf(
				"disk %s: shrinking disks is not supported by Proxmox; increase the size or replace the resource",
				diskName,
			)
		}

		if desiredDisk.Storage != currentDisk.Storage {
			return fmt.Errorf(
				"disk %s: storage migration is not supported yet; recreate the disk on the target storage",
				diskName,
			)
		}

		if desiredDisk.Interface != currentDisk.Interface {
			moveActions = append(moveActions, diskMoveAction{
				diskName:        diskName,
				fromInterface:   currentDisk.Interface,
				targetInterface: desiredDisk.Interface,
			})
		}

		if desiredDisk.Size > currentDisk.Size {
			resizeActions = append(resizeActions, diskResizeAction{
				diskName:    diskName,
				interfaceID: desiredDisk.Interface,
				sizeGB:      desiredDisk.Size,
			})
		}
	}

	for _, moveAction := range moveActions {
		// Moves run before deletes so slot changes keep the underlying volume.
		if err := vm.VMOps.MoveDisk(ctx, vmID, node, moveAction.fromInterface, moveAction.targetInterface); err != nil {
			return fmt.Errorf(
				"failed to move disk %s (%s -> %s): %w",
				moveAction.diskName,
				moveAction.fromInterface,
				moveAction.targetInterface,
				err,
			)
		}
	}

	for _, removeAction := range removeActions {
		// Deletes only run after moves, so rename/re-slot cases do not drop data.
		if err := vm.VMOps.RemoveDisk(ctx, vmID, node, removeAction.interfaceID); err != nil {
			return fmt.Errorf("failed to remove disk %s: %w", removeAction.diskName, err)
		}
	}

	for _, resizeAction := range resizeActions {
		// Resizes are last because they only apply to disks still present after moves.
		if err := vm.VMOps.ResizeDisk(ctx, vmID, node, resizeAction.interfaceID, resizeAction.sizeGB); err != nil {
			return fmt.Errorf("failed to resize disk %s: %w", resizeAction.diskName, err)
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

	stateInputs, err := vm.VMOps.Get(ctx, *vmID, request.Inputs.Node, request.State.Disks)
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

// preserveInputs clears computed fields (VMID, Node, FileIDs) that the user omitted.
func preserveInputs(state, userInputs proxmox.VMInputs) proxmox.VMInputs {
	return applyPreservation(state, userInputs, true)
}

// preserveCreateState keeps full API state including computed FileIDs.
func preserveCreateState(state, userInputs proxmox.VMInputs) proxmox.VMInputs {
	return applyPreservation(state, userInputs, false)
}

// applyPreservation is shared by preserveInputs (clearComputed=true) and preserveCreateState (false).
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

	userByInterface := proxmox.DiskMapByInterface(userInputs.Disks)
	preservedDisks := make(proxmox.DiskMap, len(state.Disks))
	for diskName, disk := range state.Disks {
		if disk == nil {
			continue
		}
		preservedDisk := *disk
		if clearComputed {
			if userDisk, ok := userByInterface[disk.Interface]; ok && userDisk.FileID == nil {
				preservedDisk.FileID = nil
			}
		}
		// Preserve format when the API didn't return it (block-based storage such as
		// LVM and Ceph omits the format key from the disk config string). Without this
		// a user who sets format=raw on local-lvm would see a phantom diff on every plan.
		if userDisk, ok := userByInterface[disk.Interface]; ok &&
			userDisk.Format != nil && preservedDisk.Format == nil {
			preservedDisk.Format = userDisk.Format
		}
		preservedDisks[diskName] = &preservedDisk
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

// copyMissingDiskFileIDs copies FileIDs from state to inputs when user omitted them,
// preventing unnecessary disk recreation during Update. Matching by disk Interface.
func copyMissingDiskFileIDs(inputs *proxmox.VMInputs, state proxmox.VMInputs) {
	// Regular disks
	if len(inputs.Disks) > 0 && len(state.Disks) > 0 {
		stateByInterface := proxmox.DiskMapByInterface(state.Disks)

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

// buildOutputWithComputedFromState constructs VMOutputs by copying computed values (VMID, Node, FileIDs)
// from prior state when omitted by user.
func buildOutputWithComputedFromState(newInputs, oldState proxmox.VMInputs) proxmox.VMOutputs {
	out := proxmox.VMOutputs{VMInputs: newInputs}

	if out.VMID == nil && oldState.VMID != nil {
		out.VMID = oldState.VMID
	}
	if out.Node == nil && oldState.Node != nil {
		out.Node = oldState.Node
	}

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

	// Only reconcile disks if they changed; use state disks as baseline to avoid re-fetching.
	disksChanged := disksNeedReconciliation(request.Inputs, request.State.VMInputs)
	if disksChanged {
		if err := vm.reconcileDisks(ctx, *vmID, request.Inputs.Node, request.Inputs.Disks, request.State.Disks); err != nil {
			return response, err
		}
	}

	efiAdded := request.Inputs.EfiDisk != nil && request.State.EfiDisk == nil

	// Remove EFI disk if the user removed it from inputs but it exists in state.
	if request.Inputs.EfiDisk == nil && request.State.EfiDisk != nil {
		if err := vm.VMOps.RemoveEfiDisk(ctx, *vmID, request.Inputs.Node); err != nil {
			return response, fmt.Errorf("failed to remove EFI disk: %w", err)
		}
	}

	if err := vm.VMOps.UpdateConfig(ctx, *vmID, request.Inputs.Node, request.Inputs, request.State.VMInputs); err != nil {
		return response, err
	}

	if disksChanged || efiAdded {
		stateInputs, err := vm.VMOps.Get(ctx, *vmID, request.Inputs.Node, request.Inputs.Disks)
		if err != nil {
			l.Errorf("error reading VM %v after update: %v", *vmID, err)
			return response, err
		}
		response.Output = proxmox.VMOutputs{VMInputs: preserveCreateState(stateInputs, request.Inputs)}
	}

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

// disksNeedReconciliation reports whether the desired disk list differs from the
// prior state in any way that requires live Proxmox API calls (add, remove, resize,
// storage change, shrink, or FileID change). When it returns false, GetCurrentDisks
// and reconcileDisks can be skipped safely during Update.
func disksNeedReconciliation(inputs, state proxmox.VMInputs) bool {
	current := state.Disks
	for diskName, desiredDisk := range inputs.Disks {
		currentDisk, exists := current[diskName]
		if !exists {
			return true
		}

		changes := proxmox.CompareDisksByInterface([]*proxmox.Disk{desiredDisk}, []*proxmox.Disk{currentDisk})
		for _, ifaceChanges := range changes {
			for _, change := range ifaceChanges {
				if change.Type != proxmox.DiskUnchanged {
					return true
				}
			}
		}
	}

	for diskName := range current {
		if _, exists := inputs.Disks[diskName]; !exists {
			return true
		}
	}

	return false
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

		if differ, ok := inField.Interface().(proxmox.FieldDiffer); ok {
			for key, propertyDiff := range differ.DiffFrom(name, stateField.Interface()) {
				diff[key] = propertyDiff
			}
			continue
		}

		var propertyDiff *p.PropertyDiff
		switch {
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
