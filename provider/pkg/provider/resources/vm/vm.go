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
	"strconv"
	"time"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// VM represents a Proxmox virtual machine resource.
type VM struct{}

var (
	_ = (infer.CustomResource[Inputs, Outputs])((*VM)(nil))
	_ = (infer.CustomDelete[Outputs])((*VM)(nil))
	_ = (infer.CustomRead[Inputs, Outputs])((*VM)(nil))
	_ = (infer.CustomUpdate[Inputs, Outputs])((*VM)(nil))
	_ = (infer.CustomDiff[Inputs, Outputs])((*VM)(nil))
	_ = infer.Annotated((*Inputs)(nil))
)

// Outputs represents the output state of a Proxmox virtual machine resource.
type Outputs struct {
	Inputs
}

// Create creates a new virtual machine based on the provided inputs.
func (vm *VM) Create(
	ctx context.Context,
	request infer.CreateRequest[Inputs],
) (response infer.CreateResponse[Outputs], err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Create VM: %v", request.Inputs.VMID)

	response.Output = Outputs{request.Inputs}
	response.ID = request.Name

	if request.DryRun {
		return response, nil
	}

	pxClient, err := client.GetProxmoxClientFn(ctx)
	if err != nil {
		return response, err
	}

	cluster, err := pxClient.Cluster(ctx)
	if err != nil {
		return response, err
	}

	nodeName, err := getNodeName(request.Inputs, cluster)
	if err != nil {
		return response, err
	}

	request.Inputs.Node = &nodeName

	if request.Inputs.VMID == nil {
		if err = setNextVMId(ctx, cluster, &response.Output); err != nil {
			l.Errorf("error: %v", err)
			return response, err
		}
	}

	vmID := *response.Output.VMID

	l.Infof("Create VM '%v(%v)' on '%v'", *response.Output.Name, vmID, nodeName)
	options := request.Inputs.BuildOptions(vmID)

	var createTask *api.Task
	var timeout time.Duration

	if createTask, timeout, err = createVMTask(ctx, request.Inputs, vmID, options); err != nil {
		l.Errorf("error: %v", err)
		return response, err
	}

	interval := 5 * time.Second
	if err = createTask.Wait(ctx, interval, timeout); err != nil {
		l.Errorf("error waiting for VM creation task: %v", err)
		return response, err
	}

	if request.Inputs.Clone != nil {
		if err = finalizeClone(ctx, pxClient, request.Inputs, vmID, options); err != nil {
			l.Errorf("error: %v", err)
			return response, err
		}
	}

	// Read the current state of the VM after creation
	if response.Output, err = readCurrentOutput(ctx, vm, &request, vmID); err != nil {
		l.Errorf("error: %v", err)
		return response, err
	}

	return response, nil
}

// setNextVMId sets the next available VM ID.
func setNextVMId(ctx context.Context, cluster *api.Cluster, outputs *Outputs) error {
	vmIDInt, err := cluster.NextID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get next VM ID: %v", err)
	}
	outputs.VMID = &vmIDInt
	return nil
}

// createVMTask creates a task for creating a new VM or cloning an existing one.
func createVMTask(
	ctx context.Context,
	inputs Inputs,
	vmID int,
	options []api.VirtualMachineOption,
) (
	createTask *api.Task,
	timeout time.Duration,
	err error,
) {
	if inputs.Clone != nil {
		createTask, err = handleClone(ctx, inputs, vmID)
		timeout = time.Duration(inputs.Clone.Timeout) * time.Second
	} else {
		createTask, err = handleNewVM(ctx, inputs, vmID, options)
		timeout = 60 * time.Second
	}

	return createTask, timeout, err
}

// finalizeClone finalizes the cloning process by updating the disks and configuration.
func finalizeClone(
	ctx context.Context,
	pxClient *px.Client,
	inputs Inputs,
	vmID int,
	options []api.VirtualMachineOption,
) (err error) {
	var virtualMachine *api.VirtualMachine
	virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, vmID, inputs.Node)
	if err != nil {
		return fmt.Errorf("failed to find cloned VM: %v", err)
	}

	// Update disks after clone based on the inputs
	if err = updateDisksAfterClone(ctx, options, virtualMachine); err != nil {
		return fmt.Errorf("failed to update disks after clone: %w", err)
	}

	task, err := virtualMachine.Config(ctx, options...)
	if err != nil {
		return fmt.Errorf("failed to update cloned VM: %v", err)
	}

	interval := 5 * time.Second
	timeout := time.Duration(inputs.Clone.Timeout) * time.Second
	if err = task.Wait(ctx, interval, timeout); err != nil {
		return fmt.Errorf("failed to wait for cloned VM update: %v", err)
	}

	return nil
}

// readCurrentOutput reads the current state of the VM after creation.
func readCurrentOutput(
	ctx context.Context,
	vm *VM,
	request *infer.CreateRequest[Inputs],
	vmID int,
) (currentOutput Outputs, err error) {
	state := Outputs{Inputs: request.Inputs}
	state.VMID = &vmID
	readRequest := infer.ReadRequest[Inputs, Outputs]{
		ID:     request.Name,
		Inputs: request.Inputs,
		State:  state,
	}

	var readResponse infer.ReadResponse[Inputs, Outputs]

	if readResponse, err = vm.Read(ctx, readRequest); err != nil {
		return Outputs{}, fmt.Errorf("failed to read VM after creation: %v", err)
	}

	currentOutput = readResponse.State
	// Preserve clone configuration (not returned by Read) if user supplied it.
	if request.Inputs.Clone != nil && currentOutput.Clone == nil {
		currentOutput.Clone = request.Inputs.Clone
	}

	return currentOutput, nil
}

// getNodeName returns the node name from the inputs or selects the first node from the cluster.
func getNodeName(inputs Inputs, cluster *api.Cluster) (string, error) {
	if inputs.Node != nil {
		return *inputs.Node, nil
	}
	if len(cluster.Nodes) == 0 {
		return "", errors.New("no nodes found in the cluster")
	}
	return cluster.Nodes[0].Name, nil
}

// handleClone handles the cloning of a virtual machine.
func handleClone(ctx context.Context, inputs Inputs, vmID int) (createTask *api.Task, err error) {
	pxc, err := client.GetProxmoxClientFn(ctx)
	if err != nil {
		return nil, err
	}

	var sourceVM *api.VirtualMachine
	if sourceVM, _, _, err = pxc.FindVirtualMachine(ctx, inputs.Clone.VMID, nil); err != nil {
		return nil, fmt.Errorf("error during finding source VM for clone: %v", err)
	}

	fullClone := uint8(0)
	if inputs.Clone.FullClone != nil && *inputs.Clone.FullClone {
		fullClone = uint8(1)
	}

	cloneOptions := api.VirtualMachineCloneOptions{
		Full:   fullClone,
		Target: *inputs.Node,
		NewID:  vmID,
	}

	var cloneTask *api.Task
	if _, cloneTask, err = sourceVM.Clone(ctx, &cloneOptions); err != nil {
		return nil, fmt.Errorf("error during cloning VM %v: %v", inputs.Clone.VMID, err)
	}

	return cloneTask, nil
}

// handleNewVM handles the creation of a new virtual machine.
func handleNewVM(
	ctx context.Context,
	inputs Inputs,
	vmID int,
	options []api.VirtualMachineOption,
) (createTask *api.Task, err error) {
	pxc, err := client.GetProxmoxClientFn(ctx)
	if err != nil {
		return nil, err
	}

	var node *api.Node
	if node, err = pxc.Node(ctx, *inputs.Node); err != nil {
		return nil, err
	}

	if createTask, err = node.NewVirtualMachine(ctx, vmID, options...); err != nil {
		return nil, err
	}

	p.GetLogger(ctx).Debugf("Create VM Task: %v", createTask)
	return createTask, nil
}

// Read reads the state of the virtual machine.
func (vm *VM) Read(
	ctx context.Context,
	request infer.ReadRequest[Inputs, Outputs],
) (response infer.ReadResponse[Inputs, Outputs], err error) {
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
		err = errors.New("VMID is required for reading VM state but is nil in both inputs and state")
		l.Errorf("VMID is nil in both inputs and state during read operation")
		return response, err
	}

	var pxClient *px.Client
	if pxClient, err = client.GetProxmoxClientFn(ctx); err != nil {
		err = fmt.Errorf("failed to get Proxmox client: %v", err)
		l.Errorf("Error during getting Proxmox client: %v", err)
		return response, err
	}

	var virtualMachine *api.VirtualMachine
	virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *vmID, request.Inputs.Node)
	if err != nil {
		l.Errorf("Error during finding VM %v: %v", *vmID, err)
		return response, err

	}
	var apiInputs Inputs
	if apiInputs, err = ConvertVMConfigToInputs(virtualMachine, request.Inputs); err != nil {
		err = fmt.Errorf("failed to convert VM to inputs %v", err)
		l.Errorf("Error during converting VM to inputs for %v: %v", virtualMachine.VMID, err)
		return response, err
	}
	// State (outputs) should contain fully computed values from API
	response.State.Inputs = apiInputs
	// Build Inputs that preserve emptiness for computed fields if they were empty before
	response.Inputs = preserveComputedInputEmptiness(request.Inputs, apiInputs)
	// Preserve clone info from prior state (not derivable from VM config).
	if request.State.Clone != nil && response.State.Clone == nil {
		response.State.Clone = request.State.Clone
	}

	response.ID = request.ID

	l.Debugf("VM: %v", virtualMachine)
	return response, nil
}

// copyMissingDiskFileIDs propagates FileID values from the current state into the
// user inputs when the user omitted them. This prevents unnecessary disk recreation
// during Update operations when the intention is to keep existing disks.
// Matching is done by disk Interface. EFI disk handled separately.
func copyMissingDiskFileIDs(inputs *Inputs, state Inputs) {
	// Regular disks
	if len(inputs.Disks) > 0 && len(state.Disks) > 0 {
		stateByInterface := make(map[string]*Disk, len(state.Disks))
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

// buildOutputWithComputedFromState constructs Outputs for Update by starting from new inputs
// and copying computed values (VMID, Node, disk and EFI FileIDs) from the prior state when
// the user omitted them. Inputs remain as provided by the user; Outputs carry computed values.
func buildOutputWithComputedFromState(newInputs, oldState Inputs) Outputs {
	out := Outputs{Inputs: newInputs}

	// VMID and Node: copy from state if user did not provide
	if out.VMID == nil && oldState.VMID != nil {
		out.VMID = oldState.VMID
	}
	if out.Node == nil && oldState.Node != nil {
		out.Node = oldState.Node
	}

	// Disks and EFI FileIDs: copy from state when omitted in inputs
	merged := out.Inputs
	copyMissingDiskFileIDs(&merged, oldState)
	out.Inputs = merged

	return out
}

// Update updates the state of the virtual machine.
func (vm *VM) Update(
	ctx context.Context,
	request infer.UpdateRequest[Inputs, Outputs],
) (response infer.UpdateResponse[Outputs], err error) {
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
	copyMissingDiskFileIDs(&request.Inputs, request.State.Inputs)

	// Build outputs by copying computed fields from prior state where inputs omit them
	response.Output = buildOutputWithComputedFromState(request.Inputs, request.State.Inputs)

	if request.DryRun {
		return response, nil
	}

	var pxClient *px.Client
	if pxClient, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	var virtualMachine *api.VirtualMachine
	if virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *vmID, request.State.Node); err != nil {
		return response, err
	}

	if request.Inputs.EfiDisk == nil {
		if err := removeEfiDisk(ctx, virtualMachine); err != nil {
			return response, err
		}
	}

	l.Debugf("VM: %v", virtualMachine)
	options := request.Inputs.BuildOptionsDiff(*vmID, &request.State.Inputs)
	l.Debugf("Update options: %+v", options)
	// Only call Config if there are options to apply; Proxmox returns 500 otherwise.
	var task *api.Task
	if len(options) > 0 {
		if task, err = virtualMachine.Config(ctx, options...); err != nil {
			err = fmt.Errorf("failed to update VM %d: %v", *vmID, err)
			return response, err
		}

		// Wait for the task to complete
		interval := 5 * time.Second
		timeout := time.Duration(60) * time.Second
		if err = task.Wait(ctx, interval, timeout); err != nil {
			err = fmt.Errorf("failed to wait for VM %d update: %v", *vmID, err)
			return response, err
		}

		// Check task status and handle failure
		if task.IsFailed {
			err = fmt.Errorf("update task for VM %d failed: %v", *vmID, task.ExitStatus)
			return response, err
		}

		l.Debugf("Update VM Task: %v", task)
	} else {
		l.Debugf("No VM config options to apply; skipping Config call")
	}

	return response, nil
}

// Delete deletes the virtual machine.
func (vm *VM) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Deleting VM: %v", request.ID)

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, fmt.Errorf("failed to get Proxmox client: %v", err)
	}

	var virtualMachine *api.VirtualMachine
	if virtualMachine, _, _, err = pxc.FindVirtualMachine(ctx, *request.State.VMID, request.State.Node); err != nil {
		return response, err
	}

	var task *api.Task
	if task, err = virtualMachine.Delete(ctx); err != nil {
		return response, fmt.Errorf("failed to delete VM %d: %v", *request.State.VMID, err)
	}

	l.Debugf("Delete VM Task: %v", task)
	return response, nil
}

// disksChanged compares two disk slices and returns true if they have meaningful changes.
// FileID differences are ignored when the input FileID is nil (computed field).
func disksChanged(inputDisks, stateDisks []*Disk) bool {
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
func compareEfiDiskFields(inputEfi, stateEfi *EfiDisk) map[string]p.PropertyDiff {
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
		efiDiffs[efiDiskInputName] = p.PropertyDiff{Kind: p.Update}
		return efiDiffs, nil
	}

	// Both non-nil: compare with granular diffs
	if !inNil && !stateNil {
		inputEfi, okIn := inField.Interface().(*EfiDisk)
		stateEfi, okState := stateField.Interface().(*EfiDisk)
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
	request infer.DiffRequest[Inputs, Outputs],
) (response infer.DiffResponse, err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Diff VM: id=%s", request.ID)

	diff := map[string]p.PropertyDiff{}

	// Properties considered computed when absent in user inputs.
	computed := map[string]struct{}{"vmId": {}, "node": {}}

	inVal := reflect.ValueOf(request.Inputs)
	stateVal := reflect.ValueOf(request.State.Inputs)
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
		case name == efiDiskInputName:
			var efiDiff map[string]p.PropertyDiff
			if efiDiff, err = handleEfiDiskDiff(inField, stateField); err != nil {
				return p.DiffResponse{}, err
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

	response = p.DiffResponse{
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
		inputDisks, okIn := inField.Interface().([]*Disk)
		stateDisks, okState := stateField.Interface().([]*Disk)
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

// preserveComputedInputEmptiness returns a copy of apiInputs with computed fields
// cleared (set to nil) where they were previously empty in prev inputs. This keeps
// Inputs reflecting user intent while Outputs (state) carry the full computed values.
func preserveComputedInputEmptiness(prev, apiInputs Inputs) Inputs {
	newInputs := apiInputs

	// Computed: VMID and Node
	if prev.VMID == nil {
		newInputs.VMID = nil
	}
	if prev.Node == nil {
		newInputs.Node = nil
	}

	// Computed: Disk filenames per interface
	if len(newInputs.Disks) > 0 && len(prev.Disks) > 0 {
		prevByIf := make(map[string]*Disk, len(prev.Disks))
		for _, d := range prev.Disks {
			if d != nil && d.Interface != "" {
				prevByIf[d.Interface] = d
			}
		}
		for i, d := range newInputs.Disks {
			if d == nil || d.Interface == "" {
				continue
			}
			if prevDisk, ok := prevByIf[d.Interface]; ok {
				if prevDisk.FileID == nil {
					newInputs.Disks[i].FileID = nil
				}
			}
		}
	}

	// Computed: EFI disk filename; remove entire EFI disk if it didn't exist previously
	if prev.EfiDisk == nil {
		newInputs.EfiDisk = nil
	} else if newInputs.EfiDisk != nil && prev.EfiDisk.FileID == nil {
		newInputs.EfiDisk.FileID = nil
	}

	return newInputs
}

// UpdateDisksAfterClone updates the disks of the virtual machine after a clone operation.
// It updates the file and size of the disks and resizes them if necessary.
// It also removes disks that are not present in the new configuration.
func updateDisksAfterClone(
	ctx context.Context,
	options []api.VirtualMachineOption,
	virtualMachine *api.VirtualMachine,
) (err error) {
	disks := virtualMachine.VirtualMachineConfig.MergeDisks()

	// Update disks based on the new configuration
	for diskInterface, currentDiskStr := range disks {
		diskOption := getDiskOption(options, diskInterface)
		if diskOption != nil {
			disk := Disk{Interface: diskInterface}
			if err = disk.ParseDiskConfig(diskOption.Value.(string)); err != nil {
				return fmt.Errorf("failed to parse disk config: %v", err)
			}

			currentDisk := Disk{Interface: diskInterface}
			if err = currentDisk.ParseDiskConfig(currentDiskStr); err != nil {
				return fmt.Errorf("failed to parse current disk config: %v", err)
			}

			disk.FileID = currentDisk.FileID
			_, diskConfig := disk.ToProxmoxDiskKeyConfig()
			diskOption.Value = diskConfig

			// Resize disk if necessary
			if disk.Size != currentDisk.Size {
				if _, err = virtualMachine.ResizeDisk(ctx, diskInterface, strconv.Itoa(disk.Size)+"G"); err != nil {
					return fmt.Errorf("failed to resize disk %v: %v", diskInterface, err)
				}
			}
		} else {
			// Remove not needed disk
			if _, err = virtualMachine.UnlinkDisk(ctx, diskInterface, true); err != nil {
				return fmt.Errorf("failed to unlink disk %v: %v", diskInterface, err)
			}
		}
	}

	efiDiskOption := getDiskOption(options, efiDiskID)
	if efiDiskOption == nil && virtualMachine.VirtualMachineConfig.EFIDisk0 != "" {
		// Remove not needed EFI disk
		if err := removeEfiDisk(ctx, virtualMachine); err != nil {
			return err
		}
	}

	return nil
}

func removeEfiDisk(ctx context.Context, virtualMachine *api.VirtualMachine) error {
	var unlinkTask *api.Task
	var err error
	if unlinkTask, err = virtualMachine.UnlinkDisk(ctx, efiDiskID, true); err != nil {
		return fmt.Errorf("failed to unlink EFI disk: %v", err)
	}

	// Some Proxmox operations may not return a task (nil) if no-op or immediate.
	if unlinkTask == nil {
		return nil
	}

	interval := 5 * time.Second
	timeout := time.Duration(60) * time.Second
	if err = unlinkTask.Wait(ctx, interval, timeout); err != nil {
		return fmt.Errorf("failed to wait for EFI disk removal task: %v", err)
	}

	if unlinkTask.IsFailed {
		return fmt.Errorf("EFI disk removal task failed: %v", unlinkTask.ExitStatus)
	}

	return nil
}

// Annotate adds descriptions to the Inputs resource and its properties
func (inputs *Inputs) Annotate(a infer.Annotator) {
	a.Describe(
		inputs,
		"A Proxmox Virtual Machine (VM) resource that manages virtual machines in the Proxmox VE.",
	)
}

// Annotate provides documentation for the CPU type.
func (cpu *CPU) Annotate(a infer.Annotator) {
	a.Describe(
		&cpu,
		"CPU configuration for the virtual machine.",
	)
	a.SetDefault(&cpu.Cores, 1, "Number of CPU cores")
}

// Annotate provides documentation for the EfiDisk type.
func (efiDisk *EfiDisk) Annotate(a infer.Annotator) {
	a.Describe(
		&efiDisk,
		"EFI disk configuration for the virtual machine.",
	)
}
