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
	_ = (infer.CustomResource[Inputs, Outputs])((*VM)(nil))
	_ = (infer.CustomDelete[Outputs])((*VM)(nil))
	_ = (infer.CustomRead[Inputs, Outputs])((*VM)(nil))
	_ = (infer.CustomUpdate[Inputs, Outputs])((*VM)(nil))
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

	pxClient, err := client.GetProxmoxClient(ctx)
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
		if err = setNextVMId(ctx, cluster, &request.Inputs); err != nil {
			l.Errorf("error: %v", err)
			return response, err
		}
	}

	l.Infof("Create VM '%v(%v)' on '%v'", *request.Inputs.Name, *request.Inputs.VMID, nodeName)
	options := request.Inputs.BuildOptions(*request.Inputs.VMID)

	var createTask *api.Task
	var timeout time.Duration
	if createTask, timeout, err = createVMTask(ctx, request.Inputs, options); err != nil {
		l.Errorf("error: %v", err)
		return response, err
	}

	interval := 5 * time.Second
	if err = createTask.Wait(ctx, interval, timeout); err != nil {
		l.Errorf("error waiting for VM creation task: %v", err)
		return response, err
	}

	if request.Inputs.Clone != nil {
		if err = finalizeClone(ctx, pxClient, request.Inputs, options); err != nil {
			l.Errorf("error: %v", err)
			return response, err
		}
	}

	// Read the current state of the VM after creation
	if response.Output, err = readCurrentOutput(ctx, vm, request); err != nil {
		l.Errorf("error: %v", err)
		return response, err
	}

	return response, nil
}

// setNextVMId sets the next available VM ID.
func setNextVMId(ctx context.Context, cluster *api.Cluster, inputs *Inputs) error {
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
	inputs Inputs,
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
	inputs Inputs,
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
	request infer.CreateRequest[Inputs],
) (currentOutput Outputs, err error) {
	readRequest := infer.ReadRequest[Inputs, Outputs]{
		ID:     request.Name,
		Inputs: request.Inputs,
		State:  Outputs{Inputs: request.Inputs},
	}

	var readResponse infer.ReadResponse[Inputs, Outputs]

	if readResponse, err = vm.Read(ctx, readRequest); err != nil {
		return Outputs{}, fmt.Errorf("failed to read VM after creation: %v", err)
	}

	currentOutput = readResponse.State
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
func handleClone(ctx context.Context, inputs Inputs) (createTask *api.Task, err error) {
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
	inputs Inputs,
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
	request infer.ReadRequest[Inputs, Outputs],
) (response infer.ReadResponse[Inputs, Outputs], err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Read VM with ID: %v", request.Inputs.VMID)

	var pxClient *px.Client
	if pxClient, err = client.GetProxmoxClient(ctx); err != nil {
		err = fmt.Errorf("failed to get Proxmox client: %v", err)
		l.Errorf("Error during getting Proxmox client: %v", err)
		return response, err
	}

	var virtualMachine *api.VirtualMachine
	virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *request.Inputs.VMID, request.Inputs.Node)
	if err != nil {
		l.Errorf("Error during finding VM %v: %v", request.Inputs.VMID, err)
		return response, err

	}

	if response.State.Inputs, err = ConvertVMConfigToInputs(virtualMachine); err != nil {
		err = fmt.Errorf("failed to convert VM to inputs %v", err)
		l.Errorf("Error during converting VM to inputs for %v: %v", virtualMachine.VMID, err)
		return response, err
	}
	response.ID = request.ID

	l.Debugf("VM: %v", virtualMachine)
	return response, nil
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

	response.Output = Outputs{
		Inputs: request.Inputs,
	}

	if request.DryRun {
		return response, nil
	}

	var pxClient *px.Client
	if pxClient, err = client.GetProxmoxClient(ctx); err != nil {
		return response, err
	}

	var virtualMachine *api.VirtualMachine
	if virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *vmID, request.State.Node); err != nil {
		return response, err
	}
	l.Debugf("VM: %v", virtualMachine)
	options := request.Inputs.BuildOptionsDiff(*vmID, &response.Output.Inputs)

	var task *api.Task
	if task, err = virtualMachine.Config(ctx, options...); err != nil {
		return response, err
	}

	l.Debugf("Update VM Task: %v", task)
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
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
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

// Annotate sets default values for the Args struct.
func (inputs *Inputs) Annotate(a infer.Annotator) {
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

	return nil
}
