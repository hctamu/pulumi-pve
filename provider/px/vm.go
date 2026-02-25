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

package px

import (
	"context"
	"fmt"

	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
)

// FindVirtualMachine finds a virtual machine by its ID and returns the VM, node, and cluster information.
func (client *Client) FindVirtualMachine(
	ctx context.Context,
	vmID int,
	lastKnowNode *string,
) (*api.VirtualMachine, *api.Node, *api.Cluster, error) {
	logger := p.GetLogger(ctx)

	cluster, err := client.Cluster(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	var vm *api.VirtualMachine
	var node *api.Node

	if lastKnowNode != nil {
		vm, node, err = client.FindVirtualMachineOnNode(ctx, vmID, *lastKnowNode)
		if err == nil {
			return vm, node, cluster, nil
		}

		logger.Debugf("VM not found on last known node '%v': %v", *lastKnowNode, err)
	}

	for _, nodeStatus := range cluster.Nodes {
		vm, node, err = client.FindVirtualMachineOnNode(ctx, vmID, nodeStatus.Name)
		if vm != nil {
			return vm, node, cluster, err
		}
	}

	if vm == nil {
		return nil, node, cluster, fmt.Errorf("VM with ID %d not found on any nodes", vmID)
	}

	return vm, node, cluster, err
}

// FindVirtualMachineOnNode finds a virtual machine by its ID on a specific node.
func (client *Client) FindVirtualMachineOnNode(
	ctx context.Context,
	vmID int,
	nodeName string,
) (*api.VirtualMachine, *api.Node, error) {
	node, err := client.Node(ctx, nodeName)
	if err != nil {
		return nil, nil, err
	}

	vm, err := node.VirtualMachine(ctx, vmID)
	if vm == nil {
		return nil, node, fmt.Errorf("VM with ID %d not found on '%v' nodes", vmID, nodeName)
	}

	return vm, node, err
}
