package px

import (
	"context"
	"fmt"

	api "github.com/luthermonson/go-proxmox"
	p "github.com/pulumi/pulumi-go-provider"
)

// FindVirtualMachine finds a virtual machine by its ID and returns the VM, node, and cluster information.
func (client *Client) FindVirtualMachine(ctx context.Context, vmId int, lastKnowNode *string) (
	vm *api.VirtualMachine, node *api.Node, cluster *api.Cluster, err error) {
	logger := p.GetLogger(ctx)

	if cluster, err = client.Cluster(ctx); err != nil {
		return nil, nil, nil, err
	}

	if lastKnowNode != nil {
		if vm, node, err = client.FindVirtualMachineOnNode(ctx, vmId, *lastKnowNode); err == nil {
			return vm, node, cluster, nil
		}

		logger.Debugf("VM not found on last known node '%v': %v", *lastKnowNode, err)
	}

	for _, nodeStatus := range cluster.Nodes {
		if vm, node, err = client.FindVirtualMachineOnNode(ctx, vmId, nodeStatus.Name); vm != nil {
			break
		}
	}

	if vm == nil {
		err = fmt.Errorf("VM with ID %d not found on any nodes", vmId)
	}

	return vm, node, cluster, err
}

// FindVirtualMachineOnNode finds a virtual machine by its ID on a specific node.
func (client *Client) FindVirtualMachineOnNode(ctx context.Context, vmId int, nodeName string) (
	vm *api.VirtualMachine, node *api.Node, err error) {

	if node, err = client.Node(ctx, nodeName); err != nil {
		return nil, nil, err
	}

	vm, err = node.VirtualMachine(ctx, vmId)

	if vm == nil {
		err = fmt.Errorf("VM with ID %d not found on '%v' nodes", vmId, nodeName)
	}

	return vm, node, err
}
