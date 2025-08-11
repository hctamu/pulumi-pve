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
	_ = (infer.CustomResource[Input, Output])((*VM)(nil))
	_ = (infer.CustomDelete[Output])((*VM)(nil))
	_ = (infer.CustomRead[Input, Output])((*VM)(nil))
	_ = (infer.CustomUpdate[Input, Output])((*VM)(nil))
)

// Output represents the output state of a Proxmox virtual machine resource.
type Output struct {
	Input
}

// Create creates a new virtual machine based on the provided inputs.
func (vm *VM) Create(
	ctx context.Context,
	id string,
	inputs Input,
	preview bool,
) (idRet string, output Output, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Create VM: %v", inputs.VMID)

	if preview {
		return id, Output{Input: inputs}, nil
	}

	pxClient, err := client.GetProxmoxClient(ctx)
	if err != nil {
		return id, Output{}, err
	}

	cluster, err := pxClient.Cluster(ctx)
	if err != nil {
		return id, Output{}, err
	}

	nodeName, err := getNodeName(inputs, cluster)
	if err != nil {
		return id, Output{}, err
	}

	inputs.Node = &nodeName

	if inputs.VMID == nil {
		if err = setNextVMId(ctx, cluster, &inputs); err != nil {
			l.Errorf("error: %v", err)
			return id, Output{}, err
		}
	}

	output = Output{Input: inputs}

	l.Infof("Create VM '%v(%v)' on '%v'", *inputs.Name, *inputs.VMID, nodeName)
	options := inputs.BuildOptions(*inputs.VMID)

	var createTask *api.Task
	var timeout time.Duration
	if createTask, timeout, err = createVMTask(ctx, inputs, options); err != nil {
		l.Errorf("error: %v", err)
		return id, Output{}, err
	}

	interval := 5 * time.Second
	if err = createTask.Wait(ctx, interval, timeout); err != nil {
		l.Errorf("error waiting for VM creation task: %v", err)
		return id, Output{}, err
	}

	if inputs.Clone != nil {
		if err = finalizeClone(ctx, pxClient, inputs, options); err != nil {
			l.Errorf("error: %v", err)
			return id, Output{}, err
		}
	}

	// Read the current state of the VM after creation
	if output, err = readCurrentOutput(ctx, vm, id, inputs, output); err != nil {
		l.Errorf("error: %v", err)
		return id, Output{}, err
	}

	return id, output, nil
}

// setNextVMId sets the next available VM ID.
func setNextVMId(ctx context.Context, cluster *api.Cluster, inputs *Input) error {
	vmIDInt, err := cluster.NextID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get next VM ID: %v", err)
	}
	inputs.VMID = &vmIDInt
	return nil
}

// createVMTask creates a task for creating a new VM or cloning an existing one.
func createVMTask(
	ctx context.Context,
	inputs Input,
	options []api.VirtualMachineOption,
) (
	createTask *api.Task,
	timeout time.Duration,
	err error,
) {
	if inputs.Clone != nil {
		createTask, err = handleClone(ctx, inputs)
		timeout = time.Duration(inputs.Clone.Timeout) * time.Second
	} else {
		createTask, err = handleNewVM(ctx, inputs, options)
		timeout = 60 * time.Second
	}
	return createTask, timeout, err
}

// finalizeClone finalizes the cloning process by updating the disks and configuration.
func finalizeClone(
	ctx context.Context,
	pxClient *px.Client,
	inputs Input,
	options []api.VirtualMachineOption,
) (err error) {
	var virtualMachine *api.VirtualMachine
	virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *inputs.VMID, inputs.Node)
	if err != nil {
		return fmt.Errorf("failed to find cloned VM: %v", err)
	}

	// Update disks after clone based on the inputs
	if err = updateDisksAfterClone(ctx, options, virtualMachine); err != nil {
		return fmt.Errorf("error during updating disks options: %v", err)
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
	id string,
	inputs Input,
	output Output,
) (currentOutput Output, err error) {
	if _, _, currentOutput, err = vm.Read(ctx, id, inputs, output); err != nil {
		return Output{}, fmt.Errorf("failed to read VM after creation: %v", err)
	}
	return currentOutput, nil
}

// getNodeName returns the node name from the inputs or selects the first node from the cluster.
func getNodeName(inputs Input, cluster *api.Cluster) (string, error) {
	if inputs.Node != nil {
		return *inputs.Node, nil
	}
	if len(cluster.Nodes) == 0 {
		return "", errors.New("no nodes found in the cluster")
	}
	return cluster.Nodes[0].Name, nil
}

// handleClone handles the cloning of a virtual machine.
func handleClone(ctx context.Context, inputs Input) (createTask *api.Task, err error) {
	pxc, err := client.GetProxmoxClient(ctx)
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
		NewID:  *inputs.VMID,
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
	inputs Input,
	options []api.VirtualMachineOption,
) (createTask *api.Task, err error) {
	pxc, err := client.GetProxmoxClient(ctx)
	if err != nil {
		return nil, err
	}

	var node *api.Node
	if node, err = pxc.Node(ctx, *inputs.Node); err != nil {
		return nil, err
	}

	if createTask, err = node.NewVirtualMachine(ctx, *inputs.VMID, options...); err != nil {
		return nil, err
	}

	p.GetLogger(ctx).Debugf("Create VM Task: %v", createTask)
	return createTask, nil
}

// Read reads the state of the virtual machine.
func (vm *VM) Read(
	ctx context.Context,
	id string,
	inputs Input,
	output Output,
) (
	idRet string,
	normalizedInputs Input,
	normalizedOutputs Output,
	err error,
) {
	l := p.GetLogger(ctx)
	l.Debugf("Read VM with ID: %v", output.VMID)

	var pxClient *px.Client
	if pxClient, err = client.GetProxmoxClient(ctx); err != nil {
		err = fmt.Errorf("failed to get Proxmox client: %v", err)
		l.Errorf("Error during getting Proxmox client: %v", err)
		return "", Input{}, Output{}, err
	}

	var virtualMachine *api.VirtualMachine
	if virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *inputs.VMID, inputs.Node); err != nil {
		return "", Input{}, Output{}, err
	}

	if normalizedOutputs.Input, err = ConvertVMConfigToInputs(virtualMachine); err != nil {
		err = fmt.Errorf("failed to convert VM to inputs %v", err)
		l.Errorf("Error during converting VM to inputs for %v: %v", virtualMachine.VMID, err)
		return "", Input{}, Output{}, err
	}

	l.Debugf("VM: %v", virtualMachine)
	return id, normalizedOutputs.Input, normalizedOutputs, nil
}

// Update updates the state of the virtual machine.
func (vm *VM) Update(
	ctx context.Context,
	id string,
	output Output,
	inputs Input,
	preview bool,
) (outputRet Output, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Update VM with ID: %v", id)

	vmID := output.VMID
	if inputs.VMID == nil {
		inputs.VMID = vmID
	}

	nodeID := output.Node
	if inputs.Node == nil {
		inputs.Node = nodeID
	}

	outputRet.Input = inputs

	if preview {
		return outputRet, nil
	}

	var pxClient *px.Client
	if pxClient, err = client.GetProxmoxClient(ctx); err != nil {
		return outputRet, err
	}

	var virtualMachine *api.VirtualMachine
	if virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *vmID, output.Node); err != nil {
		return outputRet, err
	}
	l.Debugf("VM: %v", virtualMachine)
	options := inputs.BuildOptionsDiff(*vmID, &output.Input)

	var task *api.Task
	if task, err = virtualMachine.Config(ctx, options...); err != nil {
		return outputRet, err
	}

	l.Debugf("Update VM Task: %v", task)
	return outputRet, nil
}

// Delete deletes the virtual machine.
func (vm *VM) Delete(ctx context.Context, id string, output Output) (err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Deleting VM: %v", id)

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return fmt.Errorf("failed to get Proxmox client: %v", err)
	}

	var virtualMachine *api.VirtualMachine
	if virtualMachine, _, _, err = pxc.FindVirtualMachine(ctx, *output.VMID, output.Node); err != nil {
		return err
	}

	var task *api.Task
	if task, err = virtualMachine.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete VM %d: %v", *output.VMID, err)
	}

	l.Debugf("Delete VM Task: %v", task)
	return nil
}

// Annotate sets default values for the Args struct.
func (inputs *Input) Annotate(a infer.Annotator) {
	a.SetDefault(&inputs.Cores, 1)
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

	for diskInterface, currentDiskStr := range disks {
		diskOption := getDiskOption(options, diskInterface)
		if diskOption != nil {
			disk := Disk{}
			if err = disk.ParseDiskConfig(diskOption.Value.(string)); err != nil {
				return fmt.Errorf("failed to parse disk config: %v", err)
			}

			currentDisk := Disk{}
			if err = currentDisk.ParseDiskConfig(currentDiskStr); err != nil {
				return fmt.Errorf("failed to parse current disk config: %v", err)
			}

			disk.FileID = currentDisk.FileID
			_, diskConfig := disk.ToProxmoxDiskKeyConfig()
			diskOption.Value = diskConfig

			// Resize disk if necessary
			if disk.Size != currentDisk.Size {
				if err = virtualMachine.ResizeDisk(ctx, diskInterface, strconv.Itoa(disk.Size)+"G"); err != nil {
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

	return nil
}
