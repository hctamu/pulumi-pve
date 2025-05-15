package vm

import (
	"context"
	"fmt"
	"strconv"
	"time"

	api "github.com/luthermonson/go-proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/px"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

type Vm struct{}

var _ = (infer.CustomResource[VmInput, VmOutput])((*Vm)(nil))
var _ = (infer.CustomDelete[VmOutput])((*Vm)(nil))
var _ = (infer.CustomRead[VmInput, VmOutput])((*Vm)(nil))
var _ = (infer.CustomUpdate[VmInput, VmOutput])((*Vm)(nil))

type VmOutput struct {
	VmInput
}

// Create creates a new virtual machine based on the provided inputs.
func (vm *Vm) Create(ctx context.Context, id string, inputs VmInput, preview bool) (idRet string, output VmOutput, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Create VM: %v", inputs.VmId)

	if preview {
		return id, VmOutput{VmInput: inputs}, nil
	}

	pxClient, err := client.GetProxmoxClient(ctx)
	if err != nil {
		return id, VmOutput{}, err
	}

	cluster, err := pxClient.Cluster(ctx)
	if err != nil {
		return id, VmOutput{}, err
	}

	nodeName, err := getNodeName(inputs, cluster)
	if err != nil {
		return id, VmOutput{}, err
	}

	inputs.Node = &nodeName

	if inputs.VmId == nil {
		if err = setNextVmId(ctx, cluster, &inputs); err != nil {
			l.Errorf("error: %v", err)
			return id, VmOutput{}, err
		}
	}

	output = VmOutput{VmInput: inputs}

	l.Infof("Create VM '%v(%v)' on '%v'", *inputs.Name, *inputs.VmId, nodeName)
	options := inputs.BuildOptions(*inputs.VmId)

	var createTask *api.Task
	var timeout time.Duration
	if createTask, timeout, err = createVmTask(ctx, inputs, options); err != nil {
		l.Errorf("error: %v", err)
		return id, VmOutput{}, err
	}

	interval := time.Duration(5 * time.Second)
	createTask.Wait(ctx, interval, timeout)

	if inputs.Clone != nil {
		if err = finalizeClone(ctx, pxClient, inputs, options); err != nil {
			l.Errorf("error: %v", err)
			return id, VmOutput{}, err
		}
	}

	// Read the current state of the VM after creation
	if output, err = readCurrentOutput(ctx, vm, id, inputs, output); err != nil {
		l.Errorf("error: %v", err)
		return id, VmOutput{}, err
	}

	return id, output, nil
}

// setNextVmId sets the next available VM ID.
func setNextVmId(ctx context.Context, cluster *api.Cluster, inputs *VmInput) error {
	vmIdInt, err := cluster.NextID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get next VM ID: %v", err)
	}
	inputs.VmId = &vmIdInt
	return nil
}

// createVmTask creates a task for creating a new VM or cloning an existing one.
func createVmTask(ctx context.Context, inputs VmInput, options []api.VirtualMachineOption) (createTask *api.Task, timeout time.Duration, err error) {
	if inputs.Clone != nil {
		createTask, err = handleClone(ctx, inputs)
		timeout = time.Duration(inputs.Clone.Timeout) * time.Second
	} else {
		createTask, err = handleNewVm(ctx, inputs, options)
		timeout = time.Duration(60 * time.Second)
	}
	return createTask, timeout, err
}

// finalizeClone finalizes the cloning process by updating the disks and configuration.
func finalizeClone(ctx context.Context, pxClient *px.Client, inputs VmInput, options []api.VirtualMachineOption) (err error) {
	var virtualMachine *api.VirtualMachine
	virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *inputs.VmId, inputs.Node)
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

	interval := time.Duration(5 * time.Second)
	timeout := time.Duration(inputs.Clone.Timeout) * time.Second
	if err = task.Wait(ctx, interval, timeout); err != nil {
		return fmt.Errorf("failed to wait for cloned VM update: %v", err)
	}

	return nil
}

// readCurrentOutput reads the current state of the VM after creation.
func readCurrentOutput(ctx context.Context, vm *Vm, id string, inputs VmInput, output VmOutput) (currentOutput VmOutput, err error) {
	if _, _, currentOutput, err = vm.Read(ctx, id, inputs, output); err != nil {
		return VmOutput{}, fmt.Errorf("failed to read VM after creation: %v", err)
	}
	return currentOutput, nil
}

// getNodeName returns the node name from the inputs or selects the first node from the cluster.
func getNodeName(inputs VmInput, cluster *api.Cluster) (string, error) {
	if inputs.Node != nil {
		return *inputs.Node, nil
	}
	if len(cluster.Nodes) == 0 {
		return "", fmt.Errorf("no nodes found in the cluster")
	}
	return cluster.Nodes[0].Name, nil
}

// handleClone handles the cloning of a virtual machine.
func handleClone(ctx context.Context, inputs VmInput) (createTask *api.Task, err error) {
	pxc, err := client.GetProxmoxClient(ctx)
	if err != nil {
		return nil, err
	}

	var sourceVm *api.VirtualMachine
	if sourceVm, _, _, err = pxc.FindVirtualMachine(ctx, inputs.Clone.VmId, nil); err != nil {
		return nil, fmt.Errorf("error during finding source VM for clone: %v", err)
	}

	fullClone := uint8(0)
	if inputs.Clone.FullClone != nil && *inputs.Clone.FullClone {
		fullClone = uint8(1)
	}

	cloneOptions := api.VirtualMachineCloneOptions{
		Full:   fullClone,
		Target: *inputs.Node,
		NewID:  *inputs.VmId,
	}

	var cloneTask *api.Task
	if _, cloneTask, err = sourceVm.Clone(ctx, &cloneOptions); err != nil {
		return nil, fmt.Errorf("error during cloning VM %v: %v", inputs.Clone.VmId, err)
	}

	return cloneTask, nil
}

// handleNewVm handles the creation of a new virtual machine.
func handleNewVm(ctx context.Context, inputs VmInput, options []api.VirtualMachineOption) (createTask *api.Task, err error) {
	pxc, err := client.GetProxmoxClient(ctx)
	if err != nil {
		return nil, err
	}

	var node *api.Node
	if node, err = pxc.Node(ctx, *inputs.Node); err != nil {
		return nil, err
	}

	if createTask, err = node.NewVirtualMachine(ctx, *inputs.VmId, options...); err != nil {
		return nil, err
	}

	p.GetLogger(ctx).Debugf("Create VM Task: %v", createTask)
	return createTask, nil
}

// Read reads the state of the virtual machine.
func (vm *Vm) Read(ctx context.Context, id string, inputs VmInput, output VmOutput) (idRet string, normalizedInputs VmInput, normalizedOutputs VmOutput, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Read VM with ID: %v", output.VmId)
	idRet = id

	var pxClient *px.Client
	if pxClient, err = client.GetProxmoxClient(ctx); err != nil {
		err = fmt.Errorf("failed to get Proxmox client: %v", err)
		l.Errorf("Error during getting Proxmox client: %v", err)
		return "", VmInput{}, VmOutput{}, err
	}

	var virtualMachine *api.VirtualMachine
	if virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *inputs.VmId, inputs.Node); err != nil {
		return "", VmInput{}, VmOutput{}, err
	}

	if normalizedOutputs.VmInput, err = ConvertVmConfigToInputs(virtualMachine); err != nil {
		err = fmt.Errorf("failed to convert VM to inputs %v", err)
		l.Errorf("Error during converting VM to inputs for %v: %v", virtualMachine.VMID, err)
		return "", VmInput{}, VmOutput{}, err
	}

	l.Debugf("VM: %v", virtualMachine)
	return id, normalizedOutputs.VmInput, normalizedOutputs, nil
}

// Update updates the state of the virtual machine.
func (vm *Vm) Update(ctx context.Context, id string, output VmOutput, inputs VmInput, preview bool) (outputRet VmOutput, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Update VM with ID: %v", id)

	vmId := output.VmId
	if inputs.VmId == nil {
		inputs.VmId = vmId
	}

	nodeId := output.Node
	if inputs.Node == nil {
		inputs.Node = nodeId
	}

	outputRet.VmInput = inputs

	if preview {
		return outputRet, nil
	}

	var pxClient *px.Client
	if pxClient, err = client.GetProxmoxClient(ctx); err != nil {
		return outputRet, err
	}

	var virtualMachine *api.VirtualMachine
	if virtualMachine, _, _, err = pxClient.FindVirtualMachine(ctx, *vmId, output.Node); err != nil {
		return outputRet, err
	}
	l.Debugf("VM: %v", virtualMachine)
	options := inputs.BuildOptionsDiff(*vmId, &output.VmInput)

	var task *api.Task
	if task, err = virtualMachine.Config(ctx, options...); err != nil {
		return outputRet, err
	}

	l.Debugf("Update VM Task: %v", task)
	return outputRet, nil
}

// Delete deletes the virtual machine.
func (vm *Vm) Delete(ctx context.Context, id string, output VmOutput) (err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Deleting VM: %v", id)

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return fmt.Errorf("failed to get Proxmox client: %v", err)
	}

	var virtualMachine *api.VirtualMachine
	if virtualMachine, _, _, err = pxc.FindVirtualMachine(ctx, *output.VmId, output.Node); err != nil {
		return err
	}

	var task *api.Task
	if task, err = virtualMachine.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete VM %d: %v", *output.VmId, err)
	}

	l.Debugf("Delete VM Task: %v", task)
	return nil
}

// Annotate sets default values for the Args struct.
func (inputs *VmInput) Annotate(a infer.Annotator) {
	a.SetDefault(&inputs.Cores, 1)
}

// UpdateDisksAfterClone updates the disks of the virtual machine after a clone operation.
// It updates the file and size of the disks and resizes them if necessary.
// It also removes disks that are not present in the new configuration.
func updateDisksAfterClone(ctx context.Context, options []api.VirtualMachineOption, virtualMachine *api.VirtualMachine) (err error) {
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

			disk.FileId = currentDisk.FileId
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
