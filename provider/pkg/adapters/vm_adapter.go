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

package adapters

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	api "github.com/luthermonson/go-proxmox"
	"golang.org/x/exp/slices"

	p "github.com/pulumi/pulumi-go-provider"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

// Ensure VMAdapter implements the VMOperations interface.
var _ proxmox.VMOperations = (*VMAdapter)(nil)

// VMAdapter implements proxmox.VMOperations using the Proxmox API client.
type VMAdapter struct {
	client *ProxmoxAdapter
}

// NewVMAdapter creates a new VMAdapter backed by the given ProxmoxAdapter.
func NewVMAdapter(client *ProxmoxAdapter) *VMAdapter {
	return &VMAdapter{client: client}
}

// findVM locates a virtual machine by ID and optional node.
func (adapter *VMAdapter) findVM(ctx context.Context, vmID int, node *string) (*api.VirtualMachine, error) {
	vm, _, _, err := adapter.client.FindVirtualMachine(ctx, vmID, node)
	if err != nil {
		return nil, fmt.Errorf("failed to find VM %d: %w", vmID, err)
	}
	return vm, nil
}

// CreateVM creates a new (non-clone) virtual machine.
// inputs.Node and inputs.VMID must already be populated by the caller.
func (adapter *VMAdapter) CreateVM(ctx context.Context, inputs proxmox.VMInputs) error {
	l := p.GetLogger(ctx)

	if inputs.Node == nil {
		return errors.New("inputs.Node must be set before calling CreateVM")
	}
	if inputs.VMID == nil {
		return errors.New("inputs.VMID must be set before calling CreateVM")
	}

	nodeName := *inputs.Node
	vmID := *inputs.VMID

	l.Infof("Create VM '%v(%v)' on '%v'", inputs.Name, vmID, nodeName)
	options := BuildVMOptions(inputs, vmID)

	node, err := adapter.client.Node(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	createTask, err := node.NewVirtualMachine(ctx, vmID, options...)
	if err != nil {
		return fmt.Errorf("failed to create VM %d: %w", vmID, err)
	}

	l.Debugf("Create VM Task: %v", createTask)

	if err = adapter.client.WaitForTask(ctx, createTask, 60*time.Second, 0); err != nil {
		return fmt.Errorf("failed to wait for VM creation task: %w", err)
	}

	return nil
}

// CloneVM clones a source VM to create a new virtual machine.
// inputs.Clone, inputs.Node, and inputs.VMID must already be populated by the caller.
func (adapter *VMAdapter) CloneVM(ctx context.Context, inputs proxmox.VMInputs) error {
	l := p.GetLogger(ctx)

	if inputs.Clone == nil {
		return errors.New("inputs.Clone must be set before calling CloneVM")
	}
	if inputs.Node == nil {
		return errors.New("inputs.Node must be set before calling CloneVM")
	}
	if inputs.VMID == nil {
		return errors.New("inputs.VMID must be set before calling CloneVM")
	}

	vmID := *inputs.VMID

	sourceVM, _, _, err := adapter.client.FindVirtualMachine(ctx, inputs.Clone.VMID, nil)
	if err != nil {
		return fmt.Errorf("error finding source VM %d for clone: %w", inputs.Clone.VMID, err)
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

	_, cloneTask, err := sourceVM.Clone(ctx, &cloneOptions)
	if err != nil {
		return fmt.Errorf("error cloning VM %d: %w", inputs.Clone.VMID, err)
	}

	l.Debugf("Clone VM Task: %v", cloneTask)

	cloneTimeout := time.Duration(inputs.Clone.Timeout) * time.Second
	if err = adapter.client.WaitForTask(ctx, cloneTask, cloneTimeout, 0); err != nil {
		return fmt.Errorf("failed to wait for VM clone task: %w", err)
	}

	return nil
}

// Get retrieves the current state of a virtual machine.
func (adapter *VMAdapter) Get(
	ctx context.Context,
	vmID int,
	node *string,
	userDisks []*proxmox.Disk,
) (proxmox.VMInputs, error) {
	virtualMachine, _, _, err := adapter.client.FindVirtualMachine(ctx, vmID, node)
	if err != nil {
		return proxmox.VMInputs{}, fmt.Errorf("failed to find VM %v: %v", vmID, err)
	}

	stateInputs, err := ConvertVMConfigToInputs(virtualMachine, userDisks)
	if err != nil {
		return proxmox.VMInputs{}, fmt.Errorf("failed to convert VM config to inputs: %v", err)
	}

	return stateInputs, nil
}

// UpdateConfig applies configuration differences to an existing virtual machine.
func (adapter *VMAdapter) UpdateConfig(
	ctx context.Context,
	vmID int,
	node *string,
	inputs proxmox.VMInputs,
	stateInputs proxmox.VMInputs,
) error {
	l := p.GetLogger(ctx)

	virtualMachine, err := adapter.findVM(ctx, vmID, node)
	if err != nil {
		return err
	}

	l.Debugf("VM: %v", virtualMachine)
	options := BuildVMOptionsDiff(inputs, vmID, &stateInputs)
	l.Debugf("Update options: %+v", options)

	// Only call Config if there are options to apply; Proxmox returns 500 otherwise.
	if len(options) == 0 {
		l.Debugf("No VM config options to apply; skipping Config call")
		return nil
	}

	task, err := virtualMachine.Config(ctx, options...)
	if err != nil {
		return fmt.Errorf("failed to update VM %d: %w", vmID, err)
	}

	if err = adapter.client.WaitForTask(ctx, task, 60*time.Second, 0); err != nil {
		return fmt.Errorf("failed to wait for VM %d update: %w", vmID, err)
	}

	l.Debugf("Update VM Task: %v", task)
	return nil
}

// Delete deletes an existing virtual machine.
func (adapter *VMAdapter) Delete(ctx context.Context, vmID int, node *string) error {
	l := p.GetLogger(ctx)

	virtualMachine, _, _, err := adapter.client.FindVirtualMachine(ctx, vmID, node)
	if err != nil {
		return err
	}

	task, err := virtualMachine.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete VM %d: %v", vmID, err)
	}

	l.Debugf("Delete VM Task: %v", task)
	return nil
}

// ApplyConfig applies full configuration to an existing VM.
func (adapter *VMAdapter) ApplyConfig(
	ctx context.Context,
	vmID int,
	node *string,
	inputs proxmox.VMInputs,
	timeout time.Duration,
) error {
	l := p.GetLogger(ctx)

	virtualMachine, err := adapter.findVM(ctx, vmID, node)
	if err != nil {
		return fmt.Errorf("failed to find VM %d: %w", vmID, err)
	}

	options := BuildVMOptions(inputs, vmID)
	if len(options) == 0 {
		l.Debugf("No VM config options to apply; skipping Config call")
		return nil
	}

	task, err := virtualMachine.Config(ctx, options...)
	if err != nil {
		return fmt.Errorf("failed to apply config to VM %d: %w", vmID, err)
	}

	if err = adapter.client.WaitForTask(ctx, task, timeout, 0); err != nil {
		return fmt.Errorf("failed to wait for VM %d config task: %w", vmID, err)
	}

	return nil
}

// GetCurrentDisks retrieves the current disk configuration from a live VM.
// Returns regular disks keyed by interface name and the EFI disk (nil if none).
func (adapter *VMAdapter) GetCurrentDisks(
	ctx context.Context,
	vmID int,
	node *string,
) (map[string]proxmox.Disk, *proxmox.EfiDisk, error) {
	virtualMachine, err := adapter.findVM(ctx, vmID, node)
	if err != nil {
		return nil, nil, err
	}

	diskMap := virtualMachine.VirtualMachineConfig.MergeDisks()
	result := make(map[string]proxmox.Disk, len(diskMap))
	for iface, config := range diskMap {
		disk := proxmox.Disk{Interface: iface}
		if err := ParseDiskConfig(&disk, config); err != nil {
			return nil, nil, fmt.Errorf("failed to parse disk %s: %w", iface, err)
		}
		result[iface] = disk
	}

	var efiDisk *proxmox.EfiDisk
	if virtualMachine.VirtualMachineConfig.EFIDisk0 != "" {
		efiDisk = &proxmox.EfiDisk{}
		if err := ParseEfiDiskConfig(efiDisk, virtualMachine.VirtualMachineConfig.EFIDisk0); err != nil {
			return nil, nil, fmt.Errorf("failed to parse EFI disk: %w", err)
		}
	}

	return result, efiDisk, nil
}

// ResizeDisk resizes a specific disk on a VM.
func (adapter *VMAdapter) ResizeDisk(
	ctx context.Context,
	vmID int,
	node *string,
	diskInterface string,
	sizeGB int,
) error {
	virtualMachine, err := adapter.findVM(ctx, vmID, node)
	if err != nil {
		return err
	}

	sizeStr := fmt.Sprintf("%dG", sizeGB)
	if _, err := virtualMachine.ResizeDisk(ctx, diskInterface, sizeStr); err != nil {
		return fmt.Errorf("failed to resize disk %s on VM %d: %w", diskInterface, vmID, err)
	}

	return nil
}

// RemoveDisk unlinks/removes a specific disk from a VM.
func (adapter *VMAdapter) RemoveDisk(
	ctx context.Context,
	vmID int,
	node *string,
	diskInterface string,
) error {
	virtualMachine, err := adapter.findVM(ctx, vmID, node)
	if err != nil {
		return err
	}

	if _, err := virtualMachine.UnlinkDisk(ctx, diskInterface, true); err != nil {
		return fmt.Errorf("failed to unlink disk %s on VM %d: %w", diskInterface, vmID, err)
	}

	return nil
}

// RemoveEfiDisk removes the EFI disk from a VM.
func (adapter *VMAdapter) RemoveEfiDisk(ctx context.Context, vmID int, node *string) error {
	virtualMachine, err := adapter.findVM(ctx, vmID, node)
	if err != nil {
		return err
	}

	unlinkTask, err := virtualMachine.UnlinkDisk(ctx, proxmox.EfiDiskID, true)
	if err != nil {
		return fmt.Errorf("failed to unlink EFI disk on VM %d: %w", vmID, err)
	}

	if err = adapter.client.WaitForTask(ctx, unlinkTask, 60*time.Second, 0); err != nil {
		return fmt.Errorf("failed to wait for EFI disk removal task on VM %d: %w", vmID, err)
	}

	return nil
}

// ConvertVMConfigToInputs converts a VirtualMachine API response to VMInputs (state).
// userDisks is used solely as an ordering hint: disks present in userDisks are listed
// first (in user-specified order), followed by any additional disks found in the API.
// Input preservation (clearing computed fields the user did not supply) is the
// responsibility of the caller.
func ConvertVMConfigToInputs(
	vm *api.VirtualMachine,
	userDisks []*proxmox.Disk,
) (stateInputs proxmox.VMInputs, err error) {
	// go-proxmox does not populate TagsSlice from the JSON tags field during HTTP responses.
	// Call SplitTags to ensure TagsSlice is derived from the Tags string when not already set.
	if vm.VirtualMachineConfig.TagsSlice == nil && vm.VirtualMachineConfig.Tags != "" {
		vm.SplitTags()
	}

	vmConfig := vm.VirtualMachineConfig
	diskMap := vmConfig.MergeDisks()

	parsedCPU, err := parseCPUFromVMConfig(vmConfig)
	if err != nil {
		return stateInputs, err
	}

	stateDisks := []*proxmox.Disk{}
	checkedDisks := make([]string, 0, len(userDisks))

	// First: process disks in user-specified order
	for _, userDisk := range userDisks {
		if userDisk == nil || userDisk.Interface == "" {
			continue
		}
		if _, exists := diskMap[userDisk.Interface]; !exists {
			continue // disk no longer present in API
		}
		disk := &proxmox.Disk{Interface: userDisk.Interface}
		checkedDisks = append(checkedDisks, userDisk.Interface)
		if err := ParseDiskConfig(disk, diskMap[userDisk.Interface]); err != nil {
			return stateInputs, err
		}
		stateDisks = append(stateDisks, disk)
	}

	// Then: append any disks from API not covered by user's input
	for diskInterface, diskParams := range diskMap {
		if slices.Contains(checkedDisks, diskInterface) {
			continue
		}
		disk := proxmox.Disk{Interface: diskInterface}
		if err := ParseDiskConfig(&disk, diskParams); err != nil {
			return stateInputs, err
		}
		stateDisks = append(stateDisks, &disk)
	}

	var efiDisk *proxmox.EfiDisk
	if vmConfig.EFIDisk0 != "" {
		efiDisk = &proxmox.EfiDisk{}
		if err := ParseEfiDiskConfig(efiDisk, vmConfig.EFIDisk0); err != nil {
			return stateInputs, err
		}
	}

	var vmID int
	if vm.VMID > math.MaxInt {
		return stateInputs, fmt.Errorf("VMID %d overflows int", vm.VMID)
	}
	vmID = int(vm.VMID) // #nosec G115 - overflow checked above

	stateInputs = proxmox.VMInputs{
		Name:        strOrNil(vmConfig.Name),
		Description: strOrNil(vmConfig.Description),
		VMID:        &vmID,
		Hotplug:     strOrNil(vmConfig.Hotplug),
		Template:    intOrNil(vmConfig.Template),
		Autostart:   intOrNil(vmConfig.Autostart),
		EfiDisk:     efiDisk,
		OSType:      strOrNil(vmConfig.OSType),
		Machine:     strOrNil(vmConfig.Machine),
		CPU:         parsedCPU,
		Memory:      intOrNil(int(vmConfig.Memory)),
		Balloon:     intOrNil(vmConfig.Balloon),
		Disks:       stateDisks,
		Node:        strOrNil(vm.Node),
		Tags:        vmConfig.TagsSlice,
	}

	return stateInputs, nil
}

// BuildVMOptions builds a list of VirtualMachineOption from the VMInputs.
func BuildVMOptions(inputs proxmox.VMInputs, vmID int) []api.VirtualMachineOption {
	options := []api.VirtualMachineOption{}

	addOption("name", &options, inputs.Name)
	addOption("memory", &options, inputs.Memory)
	addOption("description", &options, inputs.Description)
	addOption("autostart", &options, inputs.Autostart)
	addOption("balloon", &options, inputs.Balloon)

	tags := strings.Join(inputs.Tags, ",")
	addOption("tags", &options, &tags)

	if inputs.CPU != nil {
		if cpuStr := CPUToProxmoxString(inputs.CPU); cpuStr != "" {
			options = append(options, api.VirtualMachineOption{Name: "cpu", Value: cpuStr})
		}
		addOption("cores", &options, inputs.CPU.Cores)
		addOption("sockets", &options, inputs.CPU.Sockets)
		addOption("cpulimit", &options, inputs.CPU.Limit)
		addOption("cpuunits", &options, inputs.CPU.Units)
		addOption("vcpus", &options, inputs.CPU.Vcpus)

		if inputs.CPU.Numa != nil {
			numaValue := 0
			if *inputs.CPU.Numa {
				numaValue = 1
			}
			options = append(options, api.VirtualMachineOption{Name: "numa", Value: numaValue})
		}

		for i, node := range inputs.CPU.NumaNodes {
			numaKey := fmt.Sprintf("numa%d", i)
			options = append(options, api.VirtualMachineOption{Name: numaKey, Value: ToProxmoxNumaString(node)})
		}
	}

	if inputs.EfiDisk != nil {
		options = append(
			options,
			api.VirtualMachineOption{Name: proxmox.EfiDiskID, Value: ToProxmoxEfiDiskConfig(*inputs.EfiDisk)},
		)
	}

	for _, disk := range inputs.Disks {
		diskKey, diskConfig := ToProxmoxDiskKeyConfig(*disk)
		options = append(options, api.VirtualMachineOption{Name: diskKey, Value: diskConfig})
	}

	return options
}

// BuildVMOptionsDiff builds a list of VirtualMachineOption representing the diff between inputs and currentInputs.
func BuildVMOptionsDiff(inputs proxmox.VMInputs, vmID int, currentInputs *proxmox.VMInputs) []api.VirtualMachineOption {
	options := []api.VirtualMachineOption{}
	compareAndAddOption("name", &options, inputs.Name, currentInputs.Name)
	compareAndAddOption("memory", &options, inputs.Memory, currentInputs.Memory)
	compareAndAddOption("description", &options, inputs.Description, currentInputs.Description)
	compareAndAddOption("autostart", &options, inputs.Autostart, currentInputs.Autostart)
	compareAndAddOption("balloon", &options, inputs.Balloon, currentInputs.Balloon)
	compareAndAddOption("ostype", &options, inputs.OSType, currentInputs.OSType)
	compareAndAddTags("tags", &options, inputs.Tags, currentInputs.Tags)
	addCPUDiff(&options, &inputs, currentInputs)
	if !reflect.DeepEqual(inputs.EfiDisk, currentInputs.EfiDisk) {
		if inputs.EfiDisk != nil {
			options = append(
				options,
				api.VirtualMachineOption{Name: "efidisk0", Value: ToProxmoxEfiDiskConfig(*inputs.EfiDisk)},
			)
		}
	}
	return options
}

// parseCPUFromVMConfig parses CPU configuration from VirtualMachineConfig.
func parseCPUFromVMConfig(vmConfig *api.VirtualMachineConfig) (*proxmox.CPU, error) {
	var parsedCPU *proxmox.CPU
	if vmConfig.CPU != "" {
		cpuCfg, err := ParseCPU(vmConfig.CPU)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CPU config '%s': %w", vmConfig.CPU, err)
		}
		parsedCPU = cpuCfg
	}
	if parsedCPU == nil {
		parsedCPU = &proxmox.CPU{}
	}

	if vmConfig.Cores > 0 {
		c := vmConfig.Cores
		parsedCPU.Cores = &c
	}
	if vmConfig.Sockets > 0 {
		s := vmConfig.Sockets
		parsedCPU.Sockets = &s
	}
	if vmConfig.CPULimit > 0 {
		limit := float64(vmConfig.CPULimit)
		parsedCPU.Limit = &limit
	}
	if vmConfig.CPUUnits > 0 {
		parsedCPU.Units = &vmConfig.CPUUnits
	}
	if vmConfig.Vcpus > 0 {
		parsedCPU.Vcpus = &vmConfig.Vcpus
	}
	if vmConfig.Numa > 0 {
		numaEnabled := true
		parsedCPU.Numa = &numaEnabled
	}

	numaStrings := []string{
		vmConfig.Numa0, vmConfig.Numa1, vmConfig.Numa2, vmConfig.Numa3, vmConfig.Numa4,
		vmConfig.Numa5, vmConfig.Numa6, vmConfig.Numa7, vmConfig.Numa8, vmConfig.Numa9,
	}
	var numaNodes []proxmox.NumaNode
	for _, numaStr := range numaStrings {
		if numaStr != "" {
			node, err := ParseNumaNode(numaStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse NUMA node '%s': %w", numaStr, err)
			}
			if node != nil {
				numaNodes = append(numaNodes, *node)
			}
		}
	}
	if len(numaNodes) > 0 {
		parsedCPU.NumaNodes = numaNodes
	}

	return parsedCPU, nil
}

// addCPUDiff appends VirtualMachineOption entries for CPU differences.
func addCPUDiff(options *[]api.VirtualMachineOption, newInputs, currentInputs *proxmox.VMInputs) {
	if newInputs.CPU == nil && currentInputs.CPU == nil {
		return
	}

	var newCPU, oldCPU string
	if newInputs.CPU != nil {
		newCPU = CPUToProxmoxString(newInputs.CPU)
	}
	if currentInputs.CPU != nil {
		oldCPU = CPUToProxmoxString(currentInputs.CPU)
	}
	if newCPU != oldCPU && newCPU != "" {
		*options = append(*options, api.VirtualMachineOption{Name: "cpu", Value: newCPU})
	}

	var newCores, oldCores *int
	if newInputs.CPU != nil {
		newCores = newInputs.CPU.Cores
	}
	if currentInputs.CPU != nil {
		oldCores = currentInputs.CPU.Cores
	}
	compareAndAddOption("cores", options, newCores, oldCores)

	var newSockets, oldSockets *int
	if newInputs.CPU != nil {
		newSockets = newInputs.CPU.Sockets
	}
	if currentInputs.CPU != nil {
		oldSockets = currentInputs.CPU.Sockets
	}
	compareAndAddOption("sockets", options, newSockets, oldSockets)

	var newLimit, oldLimit *float64
	if newInputs.CPU != nil {
		newLimit = newInputs.CPU.Limit
	}
	if currentInputs.CPU != nil {
		oldLimit = currentInputs.CPU.Limit
	}
	compareAndAddOption("cpulimit", options, newLimit, oldLimit)

	var newUnits, oldUnits *int
	if newInputs.CPU != nil {
		newUnits = newInputs.CPU.Units
	}
	if currentInputs.CPU != nil {
		oldUnits = currentInputs.CPU.Units
	}
	compareAndAddOption("cpuunits", options, newUnits, oldUnits)

	var newVcpus, oldVcpus *int
	if newInputs.CPU != nil {
		newVcpus = newInputs.CPU.Vcpus
	}
	if currentInputs.CPU != nil {
		oldVcpus = currentInputs.CPU.Vcpus
	}
	compareAndAddOption("vcpus", options, newVcpus, oldVcpus)

	var newNuma, oldNuma *bool
	if newInputs.CPU != nil {
		newNuma = newInputs.CPU.Numa
	}
	if currentInputs.CPU != nil {
		oldNuma = currentInputs.CPU.Numa
	}
	if utils.DifferPtr(newNuma, oldNuma) && newNuma != nil {
		numaValue := 0
		if *newNuma {
			numaValue = 1
		}
		*options = append(*options, api.VirtualMachineOption{Name: "numa", Value: numaValue})
	}

	var newNodes, oldNodes []proxmox.NumaNode
	if newInputs.CPU != nil {
		newNodes = newInputs.CPU.NumaNodes
	}
	if currentInputs.CPU != nil {
		oldNodes = currentInputs.CPU.NumaNodes
	}
	if !NumaNodesEqual(newNodes, oldNodes) {
		for i, node := range newNodes {
			numaKey := fmt.Sprintf("numa%d", i)
			*options = append(*options, api.VirtualMachineOption{Name: numaKey, Value: ToProxmoxNumaString(node)})
		}
	}
}

// strOrNil returns a pointer to s if non-empty, else nil.
func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// intOrNil returns a pointer to v if non-zero, else nil.
func intOrNil(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

// compareAndAddOption adds name→newValue to options when newValue differs from currentValue.
func compareAndAddOption[T comparable](name string, options *[]api.VirtualMachineOption, newValue, currentValue *T) {
	if utils.DifferPtr(newValue, currentValue) && newValue != nil {
		*options = append(*options, api.VirtualMachineOption{Name: name, Value: newValue})
	}
}

// compareAndAddTags adds a "tags" option if the new and current tag sets differ.
// Comparison is order-insensitive: ["prod","web"] and ["web","prod"] are treated as equal
// because Proxmox returns tags sorted alphabetically regardless of submission order.
// When a change is detected, the new tags are sent in the user-specified order.
func compareAndAddTags(name string, options *[]api.VirtualMachineOption, newTags, currentTags []string) {
	if !utils.StringSliceChanged(newTags, currentTags) {
		return
	}
	newTagsStr := strings.Join(newTags, ",")
	*options = append(*options, api.VirtualMachineOption{Name: name, Value: &newTagsStr})
}

// addOption adds name→*value to options if value is non-nil (dereferenced).
func addOption[T comparable](name string, options *[]api.VirtualMachineOption, value *T) {
	if value != nil {
		*options = append(*options, api.VirtualMachineOption{Name: name, Value: value})
	}
}

// parsedDiskBase holds the common result of parsing a Proxmox disk config string.
type parsedDiskBase struct {
	proxmox.DiskBase
	Size   *int              // nil for EFI disks (size is ignored by the API)
	Extras map[string]string // additional key-value pairs (efitype, pre-enrolled-keys, …)
}

// parseDiskBase parses the common fields shared by regular disks and EFI disks.
func parseDiskBase(diskConfig string) (parsedDiskBase, error) {
	result := parsedDiskBase{
		Extras: make(map[string]string),
	}

	for _, part := range strings.Split(diskConfig, ",") {
		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			// Handle bare storage:fileID token (no key)
			if strings.Contains(kv[0], ":") {
				diskFile := strings.Split(kv[0], ":")
				result.Storage = diskFile[0]
				if len(diskFile) > 1 {
					f := diskFile[1]
					result.FileID = &f
				}
			}
			continue
		}

		key, value := kv[0], kv[1]
		switch key {
		case "file":
			diskFile := strings.Split(value, ":")
			result.Storage = diskFile[0]
			if len(diskFile) > 1 {
				f := diskFile[1]
				result.FileID = &f
			}
		case "size":
			size, err := parseDiskSize(value)
			if err != nil {
				return parsedDiskBase{}, err
			}
			result.Size = &size
		default:
			result.Extras[key] = value
		}
	}

	if result.Storage == "" {
		return parsedDiskBase{}, fmt.Errorf("failed to parse disk config: missing storage in %s", diskConfig)
	}

	return result, nil
}

// parseDiskSize parses a Proxmox size string (e.g. "32G", "512M") and returns the size in GiB.
func parseDiskSize(value string) (int, error) {
	if utils.EndsWithLetter(value) {
		unit := value[len(value)-1]
		size, err := strconv.Atoi(value[:len(value)-1])
		if err != nil {
			return 0, fmt.Errorf("failed to parse disk size: %v", value)
		}
		switch string(unit) {
		case "G", "g":
			return size, nil
		case "M", "m":
			return size / 1024, nil
		case "T", "t":
			return size * 1024, nil
		default:
			return 0, fmt.Errorf("unknown size unit: %v", unit)
		}
	}
	return strconv.Atoi(value)
}

// ParseDiskConfig parses a Proxmox disk config string into disk, setting Storage, FileID and Size.
func ParseDiskConfig(disk *proxmox.Disk, diskConfig string) error {
	parsed, err := parseDiskBase(diskConfig)
	if err != nil {
		return err
	}
	if parsed.Size == nil {
		return fmt.Errorf("size is required for disk: %s", diskConfig)
	}
	disk.DiskBase = parsed.DiskBase
	disk.Size = *parsed.Size
	return nil
}

// ParseEfiDiskConfig parses a Proxmox EFI disk config string into efi.
func ParseEfiDiskConfig(efi *proxmox.EfiDisk, diskConfig string) error {
	parsed, err := parseDiskBase(diskConfig)
	if err != nil {
		return err
	}
	efi.DiskBase = parsed.DiskBase
	if efitype, ok := parsed.Extras["efitype"]; ok {
		efi.EfiType = proxmox.EfiType(efitype)
	}
	if v, ok := parsed.Extras["pre-enrolled-keys"]; ok {
		b := v == "1"
		efi.PreEnrolledKeys = &b
	}
	return nil
}

// ParseNumaNode parses a Proxmox NUMA node config string into a NumaNode.
func ParseNumaNode(value string) (*proxmox.NumaNode, error) {
	if value == "" {
		return nil, nil
	}

	node := &proxmox.NumaNode{}
	for _, seg := range strings.Split(value, ",") {
		if seg == "" {
			continue
		}
		kv := strings.SplitN(seg, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "cpus":
			node.Cpus = val
		case "hostnodes":
			node.HostNodes = &val
		case "memory":
			mem, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid memory value '%s': %w", val, err)
			}
			node.Memory = &mem
		case "policy":
			node.Policy = &val
		}
	}
	if node.Cpus == "" {
		return nil, errors.New("NUMA node missing required 'cpus' field")
	}
	return node, nil
}

// ParseCPU parses a Proxmox CPU config string into a CPU.
func ParseCPU(value string) (*proxmox.CPU, error) {
	if value == "" {
		return nil, nil
	}

	cfg := &proxmox.CPU{}
	for i, seg := range strings.Split(value, ",") {
		if seg == "" {
			continue
		}
		kv := strings.SplitN(seg, "=", 2)
		if len(kv) != 2 {
			if i == 0 {
				cfg.Type = &seg
			}
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "cputype":
			cfg.Type = &val
		case "flags":
			for _, f := range strings.Split(val, ";") {
				if f == "" {
					continue
				}
				switch f[0] {
				case '+':
					cfg.FlagsEnabled = append(cfg.FlagsEnabled, f[1:])
				case '-':
					cfg.FlagsDisabled = append(cfg.FlagsDisabled, f[1:])
				default:
					cfg.FlagsEnabled = append(cfg.FlagsEnabled, f)
				}
			}
		case "hidden":
			switch val {
			case "1":
				b := true
				cfg.Hidden = &b
			case "0":
				b := false
				cfg.Hidden = &b
			}
		case "hv-vendor-id":
			cfg.HVVendorID = &val
		case "phys-bits":
			cfg.PhysBits = &val
		}
	}
	return cfg, nil
}

// NumaNodesEqual checks if two NumaNode slices are equal.
func NumaNodesEqual(a, b []proxmox.NumaNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Cpus != b[i].Cpus {
			return false
		}
		if !utils.PtrEqual(a[i].HostNodes, b[i].HostNodes) {
			return false
		}
		if !utils.PtrEqual(a[i].Memory, b[i].Memory) {
			return false
		}
		if !utils.PtrEqual(a[i].Policy, b[i].Policy) {
			return false
		}
	}
	return true
}

// CPUToProxmoxString converts the CPU config to Proxmox format.
func CPUToProxmoxString(cpu *proxmox.CPU) string {
	if cpu == nil {
		return ""
	}

	parts := make([]string, 0, 6)

	if cpu.Type != nil && *cpu.Type != "" {
		parts = append(parts, *cpu.Type)
	}

	if len(cpu.FlagsEnabled) > 0 || len(cpu.FlagsDisabled) > 0 {
		flags := make([]string, 0, len(cpu.FlagsEnabled)+len(cpu.FlagsDisabled))

		for _, f := range cpu.FlagsEnabled {
			if f == "" {
				continue
			}
			flags = append(flags, "+"+f)
		}

		for _, f := range cpu.FlagsDisabled {
			if f == "" {
				continue
			}
			flags = append(flags, "-"+f)
		}

		if len(flags) > 0 {
			parts = append(parts, "flags="+strings.Join(flags, ";"))
		}
	}

	if cpu.Hidden != nil {
		if *cpu.Hidden {
			parts = append(parts, "hidden=1")
		} else {
			parts = append(parts, "hidden=0")
		}
	}

	if cpu.HVVendorID != nil {
		parts = append(parts, "hv-vendor-id="+*cpu.HVVendorID)
	}

	if cpu.PhysBits != nil {
		parts = append(parts, "phys-bits="+*cpu.PhysBits)
	}

	return strings.Join(parts, ",")
}

// ToProxmoxEfiDiskConfig converts the EfiDisk struct to Proxmox EFI disk config string.
func ToProxmoxEfiDiskConfig(efi proxmox.EfiDisk) string {
	var fullDiskPath string
	if efi.FileID == nil || *efi.FileID == "" {
		// No file Id means we are creating the disk now, so we use the storage:size format to create the disk
		fullDiskPath = fmt.Sprintf("%v:%v", efi.Storage, proxmox.EfiDiskSize)
	} else {
		// We already have a disk file, so we use the storage:file_id format
		fullDiskPath = fmt.Sprintf("%v:%v", efi.Storage, *efi.FileID)
	}

	config := fmt.Sprintf("file=%v", fullDiskPath)
	if efi.EfiType != "" {
		config += fmt.Sprintf(",efitype=%v", efi.EfiType)
	}
	if efi.PreEnrolledKeys != nil {
		if *efi.PreEnrolledKeys {
			config += ",pre-enrolled-keys=1"
		} else {
			config += ",pre-enrolled-keys=0"
		}
	}
	return config
}

// ToProxmoxNumaString converts a NumaNode to Proxmox format.
func ToProxmoxNumaString(n proxmox.NumaNode) string {
	parts := make([]string, 0, 4)
	parts = append(parts, "cpus="+n.Cpus)
	if n.HostNodes != nil {
		parts = append(parts, "hostnodes="+*n.HostNodes)
	}
	if n.Memory != nil {
		parts = append(parts, fmt.Sprintf("memory=%d", *n.Memory))
	}
	if n.Policy != nil {
		parts = append(parts, "policy="+*n.Policy)
	}
	return strings.Join(parts, ",")
}

// ToProxmoxDiskKeyConfig converts the Disk struct to Proxmox disk key and config strings.
func ToProxmoxDiskKeyConfig(disk proxmox.Disk) (diskKey, diskConfig string) {
	var fullDiskPath string

	if disk.FileID == nil || *disk.FileID == "" {
		// No file Id means we are creating the disk now, so we use the storage:size format to create the disk
		fullDiskPath = fmt.Sprintf("%v:%v", disk.Storage, disk.Size)
	} else {
		// We already have a disk file, so we use the storage:file_id format
		fullDiskPath = fmt.Sprintf("%v:%v", disk.Storage, *disk.FileID)
	}

	diskKey = disk.Interface
	diskConfig = fmt.Sprintf("file=%v,size=%v", fullDiskPath, disk.Size)
	return
}
