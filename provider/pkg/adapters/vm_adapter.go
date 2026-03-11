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

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"
	"golang.org/x/exp/slices"

	p "github.com/pulumi/pulumi-go-provider"
)

// Ensure VMAdapter implements the VMOperations interface.
var _ proxmox.VMOperations = (*VMAdapter)(nil)

// VMAdapter implements proxmox.VMOperations using the Proxmox API client.
type VMAdapter struct{}

// NewVMAdapter creates a new VMAdapter.
func NewVMAdapter() *VMAdapter {
	return &VMAdapter{}
}

func (a *VMAdapter) pxClient(ctx context.Context) (*px.Client, error) {
	return client.GetProxmoxClientFn(ctx)
}

// Create creates a new virtual machine and returns its assigned VM ID and node name.
// inputs.Node and inputs.VMID must already be populated by the caller.
func (a *VMAdapter) Create(ctx context.Context, inputs proxmox.VMInputs) (int, string, error) {
	l := p.GetLogger(ctx)

	if inputs.Node == nil {
		return 0, "", errors.New("inputs.Node must be set before calling Create")
	}
	if inputs.VMID == nil {
		return 0, "", errors.New("inputs.VMID must be set before calling Create")
	}

	nodeName := *inputs.Node
	vmID := *inputs.VMID

	l.Infof("Create VM '%v(%v)' on '%v'", inputs.Name, vmID, nodeName)
	options := BuildVMOptions(inputs, vmID)

	createTask, timeout, err := vmCreateTask(ctx, inputs, vmID, options)
	if err != nil {
		l.Errorf("error creating VM task: %v", err)
		return 0, "", err
	}

	interval := 5 * time.Second
	if err = createTask.Wait(ctx, interval, timeout); err != nil {
		l.Errorf("error waiting for VM creation task: %v", err)
		return 0, "", err
	}

	if inputs.Clone != nil {
		pxc, err := a.pxClient(ctx)
		if err != nil {
			return 0, "", err
		}
		if err = vmFinalizeClone(ctx, pxc, inputs, vmID, options); err != nil {
			l.Errorf("error finalizing clone: %v", err)
			return 0, "", err
		}
	}

	return vmID, nodeName, nil
}

// Get retrieves the current state of a virtual machine.
// Returns computed inputs (full API state) and preserved inputs (user-visible state).
func (a *VMAdapter) Get(
	ctx context.Context,
	vmID int,
	node *string,
	existingInputs proxmox.VMInputs,
) (proxmox.VMInputs, proxmox.VMInputs, error) {
	pxClient, err := a.pxClient(ctx)
	if err != nil {
		return proxmox.VMInputs{}, proxmox.VMInputs{}, fmt.Errorf("failed to get Proxmox client: %v", err)
	}

	virtualMachine, _, _, err := pxClient.FindVirtualMachine(ctx, vmID, node)
	if err != nil {
		return proxmox.VMInputs{}, proxmox.VMInputs{}, fmt.Errorf("failed to find VM %v: %v", vmID, err)
	}

	stateInputs, preservedInputs, err := ConvertVMConfigToInputs(virtualMachine, existingInputs)
	if err != nil {
		return proxmox.VMInputs{}, proxmox.VMInputs{}, fmt.Errorf("failed to convert VM config to inputs: %v", err)
	}

	return stateInputs, preservedInputs, nil
}

// Update applies configuration changes to an existing virtual machine.
func (a *VMAdapter) Update(
	ctx context.Context,
	vmID int,
	node *string,
	inputs proxmox.VMInputs,
	stateInputs proxmox.VMInputs,
) error {
	l := p.GetLogger(ctx)

	pxClient, err := a.pxClient(ctx)
	if err != nil {
		return err
	}

	virtualMachine, _, _, err := pxClient.FindVirtualMachine(ctx, vmID, node)
	if err != nil {
		return err
	}

	if inputs.EfiDisk == nil {
		if err := vmRemoveEfiDisk(ctx, virtualMachine); err != nil {
			return err
		}
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
		return fmt.Errorf("failed to update VM %d: %v", vmID, err)
	}

	interval := 5 * time.Second
	timeout := 60 * time.Second
	if err = task.Wait(ctx, interval, time.Duration(timeout)); err != nil {
		return fmt.Errorf("failed to wait for VM %d update: %v", vmID, err)
	}

	if task.IsFailed {
		return fmt.Errorf("update task for VM %d failed: %v", vmID, task.ExitStatus)
	}

	l.Debugf("Update VM Task: %v", task)
	return nil
}

// Delete deletes an existing virtual machine.
func (a *VMAdapter) Delete(ctx context.Context, vmID int, node *string) error {
	l := p.GetLogger(ctx)

	pxClient, err := a.pxClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Proxmox client: %v", err)
	}

	virtualMachine, _, _, err := pxClient.FindVirtualMachine(ctx, vmID, node)
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

// --- Internal helper functions ---

// vmGetNextID retrieves the next available VM ID from the cluster.
func vmGetNextID(ctx context.Context, cluster *api.Cluster) (int, error) {
	vmIDInt, err := cluster.NextID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get next VM ID: %v", err)
	}
	return vmIDInt, nil
}

// vmCreateTask creates a task for creating a new VM or cloning an existing one.
func vmCreateTask(
	ctx context.Context,
	inputs proxmox.VMInputs,
	vmID int,
	options []api.VirtualMachineOption,
) (*api.Task, time.Duration, error) {
	var (
		createTask *api.Task
		timeout    time.Duration
		err        error
	)
	if inputs.Clone != nil {
		createTask, err = vmHandleClone(ctx, inputs, vmID)
		timeout = time.Duration(inputs.Clone.Timeout) * time.Second
	} else {
		createTask, err = vmHandleNewVM(ctx, inputs, vmID, options)
		timeout = 60 * time.Second
	}

	return createTask, timeout, err
}

// vmFinalizeClone finalizes the cloning process by updating the disks and configuration.
func vmFinalizeClone(
	ctx context.Context,
	pxClient *px.Client,
	inputs proxmox.VMInputs,
	vmID int,
	options []api.VirtualMachineOption,
) error {
	virtualMachine, _, _, err := pxClient.FindVirtualMachine(ctx, vmID, inputs.Node)
	if err != nil {
		return fmt.Errorf("failed to find cloned VM: %v", err)
	}

	// Update disks after clone based on the inputs
	if err := vmUpdateDisksAfterClone(ctx, options, virtualMachine); err != nil {
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

// vmHandleClone handles the cloning of a virtual machine.
func vmHandleClone(ctx context.Context, inputs proxmox.VMInputs, vmID int) (*api.Task, error) {
	pxc, err := client.GetProxmoxClientFn(ctx)
	if err != nil {
		return nil, err
	}

	sourceVM, _, _, err := pxc.FindVirtualMachine(ctx, inputs.Clone.VMID, nil)
	if err != nil {
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

// vmHandleNewVM handles the creation of a new virtual machine.
func vmHandleNewVM(
	ctx context.Context,
	inputs proxmox.VMInputs,
	vmID int,
	options []api.VirtualMachineOption,
) (*api.Task, error) {
	pxc, err := client.GetProxmoxClientFn(ctx)
	if err != nil {
		return nil, err
	}

	node, err := pxc.Node(ctx, *inputs.Node)
	if err != nil {
		return nil, err
	}

	createTask, err := node.NewVirtualMachine(ctx, vmID, options...)
	if err != nil {
		return nil, err
	}

	p.GetLogger(ctx).Debugf("Create VM Task: %v", createTask)
	return createTask, nil
}

// vmUpdateDisksAfterClone updates the disks of the virtual machine after a clone operation.
// It updates the file and size of the disks and resizes them if necessary.
// It also removes disks that are not present in the new configuration.
func vmUpdateDisksAfterClone(
	ctx context.Context,
	options []api.VirtualMachineOption,
	virtualMachine *api.VirtualMachine,
) error {
	disks := virtualMachine.VirtualMachineConfig.MergeDisks()

	// Update disks based on the new configuration
	for diskInterface, currentDiskStr := range disks {
		diskOption := getDiskOption(options, diskInterface)
		if diskOption != nil {
			disk := proxmox.Disk{Interface: diskInterface}
			if err := ParseDiskConfig(&disk, diskOption.Value.(string)); err != nil {
				return fmt.Errorf("failed to parse disk config: %v", err)
			}

			currentDisk := proxmox.Disk{Interface: diskInterface}
			if err := ParseDiskConfig(&currentDisk, currentDiskStr); err != nil {
				return fmt.Errorf("failed to parse current disk config: %v", err)
			}

			disk.FileID = currentDisk.FileID
			_, diskConfig := ToProxmoxDiskKeyConfig(disk)
			diskOption.Value = diskConfig

			// Resize disk if necessary
			if disk.Size != currentDisk.Size {
				sizeStr := fmt.Sprintf("%dG", disk.Size)
				if _, err := virtualMachine.ResizeDisk(ctx, diskInterface, sizeStr); err != nil {
					return fmt.Errorf("failed to resize disk %v: %v", diskInterface, err)
				}
			}
		} else {
			// Remove not needed disk
			if _, err := virtualMachine.UnlinkDisk(ctx, diskInterface, true); err != nil {
				return fmt.Errorf("failed to unlink disk %v: %v", diskInterface, err)
			}
		}
	}

	efiDiskOption := getDiskOption(options, proxmox.EfiDiskID)
	if efiDiskOption == nil && virtualMachine.VirtualMachineConfig.EFIDisk0 != "" {
		// Remove not needed EFI disk
		if err := vmRemoveEfiDisk(ctx, virtualMachine); err != nil {
			return err
		}
	}

	return nil
}

// vmRemoveEfiDisk removes the EFI disk from a virtual machine.
func vmRemoveEfiDisk(ctx context.Context, virtualMachine *api.VirtualMachine) error {
	var unlinkTask *api.Task
	var err error
	if unlinkTask, err = virtualMachine.UnlinkDisk(ctx, proxmox.EfiDiskID, true); err != nil {
		return fmt.Errorf("failed to unlink EFI disk: %v", err)
	}

	// Some Proxmox operations may not return a task (nil) if no-op or immediate.
	if unlinkTask == nil {
		return nil
	}

	interval := 5 * time.Second
	timeout := 60 * time.Second
	if err = unlinkTask.Wait(ctx, interval, time.Duration(timeout)); err != nil {
		return fmt.Errorf("failed to wait for EFI disk removal task: %v", err)
	}

	if unlinkTask.IsFailed {
		return fmt.Errorf("EFI disk removal task failed: %v", unlinkTask.ExitStatus)
	}

	return nil
}

// --- Functions moved from proxmox/vm.go: these depend on the go-proxmox API types ---

// ConvertVMConfigToInputs converts a VirtualMachine configuration to VMInputs.
// It returns two VMInputs:
//   - stateInputs: fully computed values from API (for Outputs/State)
//   - preservedInputs: values adjusted to preserve user-omitted computed fields (for Inputs)
func ConvertVMConfigToInputs(
	vm *api.VirtualMachine,
	currentInput proxmox.VMInputs,
) (stateInputs, preservedInputs proxmox.VMInputs, err error) {
	vmConfig := vm.VirtualMachineConfig
	diskMap := vmConfig.MergeDisks()

	parsedCPU, err := parseCPUFromVMConfig(vmConfig)
	if err != nil {
		return stateInputs, preservedInputs, err
	}

	stateDisks := []*proxmox.Disk{}
	preservedDisks := []*proxmox.Disk{}
	checkedDisks := make([]string, 0, len(currentInput.Disks))

	prevByInterface := make(map[string]*proxmox.Disk, len(currentInput.Disks))
	for _, d := range currentInput.Disks {
		if d != nil && d.Interface != "" {
			prevByInterface[d.Interface] = d
		}
	}

	for _, currentDisk := range currentInput.Disks {
		if _, exists := diskMap[currentDisk.Interface]; !exists {
			continue
		}
		disk := &proxmox.Disk{Interface: currentDisk.Interface}
		checkedDisks = append(checkedDisks, currentDisk.Interface)
		if err := ParseDiskConfig(disk, diskMap[currentDisk.Interface]); err != nil {
			return stateInputs, preservedInputs, err
		}
		stateDisks = append(stateDisks, disk)
		preservedDisk := *disk
		if prev, ok := prevByInterface[disk.Interface]; ok {
			if prev.FileID == nil {
				preservedDisk.FileID = nil
			}
		}
		preservedDisks = append(preservedDisks, &preservedDisk)
	}

	for diskInterface, diskParams := range diskMap {
		if slices.Contains(checkedDisks, diskInterface) {
			continue
		}
		disk := proxmox.Disk{Interface: diskInterface}
		if err := ParseDiskConfig(&disk, diskParams); err != nil {
			return stateInputs, preservedInputs, err
		}
		stateDisks = append(stateDisks, &disk)
		preservedDisks = append(preservedDisks, &disk)
	}

	var efiDisk *proxmox.EfiDisk
	var preservedEfi *proxmox.EfiDisk
	if vmConfig.EFIDisk0 != "" {
		efiDisk = &proxmox.EfiDisk{}
		if err := ParseEfiDiskConfig(efiDisk, vmConfig.EFIDisk0); err != nil {
			return stateInputs, preservedInputs, err
		}
		preservedEfiDisk := *efiDisk
		if currentInput.EfiDisk != nil && currentInput.EfiDisk.FileID == nil {
			preservedEfiDisk.FileID = nil
		}
		preservedEfi = &preservedEfiDisk
	}

	var vmID int
	if vm.VMID > math.MaxInt {
		return stateInputs, preservedInputs, fmt.Errorf("VMID %d overflows int", vm.VMID)
	}
	vmID = int(vm.VMID) // #nosec G115 - overflow checked above

	stateInputs = proxmox.VMInputs{
		Name:        strOrNil(vmConfig.Name),
		Description: strOrNil(vmConfig.Description),
		VMID:        &vmID,
		Hookscript:  strOrNil(vmConfig.Hookscript),
		Hotplug:     strOrNil(vmConfig.Hotplug),
		Template:    intOrNil(vmConfig.Template),
		Autostart:   intOrNil(vmConfig.Autostart),
		Tablet:      intOrNil(vmConfig.Tablet),
		KVM:         intOrNil(vmConfig.KVM),
		Protection:  intOrNil(vmConfig.Protection),
		Lock:        strOrNil(vmConfig.Lock),

		EfiDisk: efiDisk,

		OSType:  strOrNil(vmConfig.OSType),
		Machine: strOrNil(vmConfig.Machine),
		Bio:     strOrNil(vmConfig.Bios),

		Acpi: intOrNil(vmConfig.Acpi),

		CPU:       parsedCPU,
		Memory:    intOrNil(int(vmConfig.Memory)),
		Hugepages: strOrNil(vmConfig.Hugepages),
		Balloon:   intOrNil(vmConfig.Balloon),

		VGA:       strOrNil(vmConfig.VGA),
		TPMState0: strOrNil(vmConfig.TPMState0),
		Rng0:      strOrNil(vmConfig.Rng0),
		Audio0:    strOrNil(vmConfig.Audio0),

		Disks: stateDisks,

		HostPCI0:  strOrNil(vmConfig.HostPCI0),
		Serial0:   strOrNil(vmConfig.Serial0),
		USB0:      strOrNil(vmConfig.USB0),
		Parallel0: strOrNil(vmConfig.Parallel0),

		CIType:       strOrNil(vmConfig.CIType),
		CIUser:       strOrNil(vmConfig.CIUser),
		CIPassword:   strOrNil(vmConfig.CIPassword),
		Nameserver:   strOrNil(vmConfig.Nameserver),
		Searchdomain: strOrNil(vmConfig.Searchdomain),
		SSHKeys:      strOrNil(vmConfig.SSHKeys),
		CICustom:     strOrNil(vmConfig.CICustom),
		CIUpgrade:    intOrNil(vmConfig.CIUpgrade),

		IPConfig0: strOrNil(vmConfig.IPConfig0),
		Node:      strOrNil(vm.Node),
	}

	preservedInputs = stateInputs
	if currentInput.VMID == nil {
		preservedInputs.VMID = nil
	}
	if currentInput.Node == nil {
		preservedInputs.Node = nil
	}
	preservedInputs.Disks = preservedDisks
	preservedInputs.EfiDisk = preservedEfi

	return stateInputs, preservedInputs, nil
}

// BuildVMOptions builds a list of VirtualMachineOption from the VMInputs.
func BuildVMOptions(inputs proxmox.VMInputs, vmID int) []api.VirtualMachineOption {
	options := []api.VirtualMachineOption{}

	addOption("name", &options, inputs.Name)
	addOption("memory", &options, inputs.Memory)
	addOption("description", &options, inputs.Description)
	addOption("autostart", &options, inputs.Autostart)
	addOption("protection", &options, inputs.Protection)
	addOption("lock", &options, inputs.Lock)
	addOption("hugepages", &options, inputs.Hugepages)
	addOption("balloon", &options, inputs.Balloon)
	addOption("vga", &options, inputs.VGA)
	addOption("ostype", &options, inputs.OSType)
	addOption("citype", &options, inputs.CIType)
	addOption("ciuser", &options, inputs.CIUser)
	addOption("cipassword", &options, inputs.CIPassword)
	addOption("nameserver", &options, inputs.Nameserver)
	addOption("searchdomain", &options, inputs.Searchdomain)
	addOption("sshkeys", &options, inputs.SSHKeys)
	addOption("cicustom", &options, inputs.CICustom)
	addOption("ciupgrade", &options, inputs.CIUpgrade)

	if inputs.CPU != nil {
		if cpuStr := CPUToProxmoxString(inputs.CPU); cpuStr != "" {
			options = append(options, api.VirtualMachineOption{Name: "cpu", Value: cpuStr})
		}
		addOptionPtr("cores", &options, inputs.CPU.Cores)
		addOptionPtr("sockets", &options, inputs.CPU.Sockets)
		addOptionPtr("cpulimit", &options, inputs.CPU.Limit)
		addOptionPtr("cpuunits", &options, inputs.CPU.Units)
		addOptionPtr("vcpus", &options, inputs.CPU.Vcpus)

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
	compareAndAddOption("protection", &options, inputs.Protection, currentInputs.Protection)
	compareAndAddOption("lock", &options, inputs.Lock, currentInputs.Lock)
	compareAndAddOption("hugepages", &options, inputs.Hugepages, currentInputs.Hugepages)
	compareAndAddOption("balloon", &options, inputs.Balloon, currentInputs.Balloon)
	compareAndAddOption("vga", &options, inputs.VGA, currentInputs.VGA)
	compareAndAddOption("ostype", &options, inputs.OSType, currentInputs.OSType)
	compareAndAddOption("sshkeys", &options, inputs.SSHKeys, currentInputs.SSHKeys)
	compareAndAddOption("cicustom", &options, inputs.CICustom, currentInputs.CICustom)
	compareAndAddOption("ciupgrade", &options, inputs.CIUpgrade, currentInputs.CIUpgrade)
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
	compareAndAddOptionPtr("cores", options, newCores, oldCores)

	var newSockets, oldSockets *int
	if newInputs.CPU != nil {
		newSockets = newInputs.CPU.Sockets
	}
	if currentInputs.CPU != nil {
		oldSockets = currentInputs.CPU.Sockets
	}
	compareAndAddOptionPtr("sockets", options, newSockets, oldSockets)

	var newLimit, oldLimit *float64
	if newInputs.CPU != nil {
		newLimit = newInputs.CPU.Limit
	}
	if currentInputs.CPU != nil {
		oldLimit = currentInputs.CPU.Limit
	}
	compareAndAddOptionPtr("cpulimit", options, newLimit, oldLimit)

	var newUnits, oldUnits *int
	if newInputs.CPU != nil {
		newUnits = newInputs.CPU.Units
	}
	if currentInputs.CPU != nil {
		oldUnits = currentInputs.CPU.Units
	}
	compareAndAddOptionPtr("cpuunits", options, newUnits, oldUnits)

	var newVcpus, oldVcpus *int
	if newInputs.CPU != nil {
		newVcpus = newInputs.CPU.Vcpus
	}
	if currentInputs.CPU != nil {
		oldVcpus = currentInputs.CPU.Vcpus
	}
	compareAndAddOptionPtr("vcpus", options, newVcpus, oldVcpus)

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

// getDiskOption returns the VirtualMachineOption matching the given disk interface name.
func getDiskOption(options []api.VirtualMachineOption, diskInterface string) *api.VirtualMachineOption {
	for index := range options {
		if options[index].Name == diskInterface {
			return &options[index]
		}
	}
	return nil
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

// compareAndAddOptionPtr is like compareAndAddOption but passes the pointer value directly.
func compareAndAddOptionPtr[T comparable](name string, options *[]api.VirtualMachineOption, newValue, currentValue *T) {
	if utils.DifferPtr(newValue, currentValue) && newValue != nil {
		*options = append(*options, api.VirtualMachineOption{Name: name, Value: newValue})
	}
}

// addOption adds name→*value to options if value is non-nil (dereferenced).
func addOption[T comparable](name string, options *[]api.VirtualMachineOption, value *T) {
	if value != nil {
		*options = append(*options, api.VirtualMachineOption{Name: name, Value: *value})
	}
}

// addOptionPtr adds name→value (pointer) to options if value is non-nil.
func addOptionPtr[T comparable](name string, options *[]api.VirtualMachineOption, value *T) {
	if value != nil {
		*options = append(*options, api.VirtualMachineOption{Name: name, Value: value})
	}
}

// --- Disk config parsing (Proxmox wire-format strings → domain types) ---

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
