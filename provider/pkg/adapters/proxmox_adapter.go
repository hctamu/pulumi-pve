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

// Package adapters provides concrete implementations of proxmox interfaces.
package adapters

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"

	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// Ensure ProxmoxAdapter implements the ProxmoxClient interface
var _ proxmox.Client = (*ProxmoxAdapter)(nil)

// ProxmoxAdapter adapts the px.Client to implement the proxmox.ProxmoxClient interface
// It handles lazy initialization of the underlying client
type ProxmoxAdapter struct {
	once      sync.Once
	initErr   error
	PVEConfig *config.Config
	client    *api.Client
}

// NewProxmoxAdapter creates a new ProxmoxAdapter
func NewProxmoxAdapter(pveConfig *config.Config) *ProxmoxAdapter {
	return &ProxmoxAdapter{
		PVEConfig: pveConfig,
	}
}

// Connect initializes the Proxmox client if it hasn't been initialized yet
func (proxmoxAdapter *ProxmoxAdapter) Connect(ctx context.Context) error {
	proxmoxAdapter.once.Do(func() {
		p.GetLogger(ctx).Debugf("Client is not initialized, initializing now")

		var pveConfig config.Config
		if proxmoxAdapter.PVEConfig == nil {
			// If no config provided, get from context
			pveConfig = infer.GetConfig[config.Config](ctx)
		} else {
			pveConfig = *proxmoxAdapter.PVEConfig
		}

		proxmoxAdapter.client, proxmoxAdapter.initErr = newClient(pveConfig.PveURL, pveConfig.PveUser, pveConfig.PveToken)
	})

	if proxmoxAdapter.initErr != nil {
		p.GetLogger(ctx).Errorf("Error creating Proxmox client: %v", proxmoxAdapter.initErr)
		return proxmoxAdapter.initErr
	}

	return nil
}

// newClient creates a new Proxmox client
func newClient(pveURL, pveUser, pveToken string) (*api.Client, error) {
	transport := http.DefaultTransport.(*http.Transport)
	//nolint:gosec // Required for Proxmox API self-signed certificates
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	httpClient := http.DefaultClient
	httpClient.Transport = transport

	apiClient := api.NewClient(pveURL,
		api.WithAPIToken(pveUser, pveToken),
		api.WithHTTPClient(httpClient),
	)

	client := apiClient

	return client, nil
}

// Get performs a GET request to the Proxmox API.
func (proxmoxAdapter *ProxmoxAdapter) Get(ctx context.Context, path string, result any) error {
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}
	return proxmoxAdapter.client.Get(ctx, path, result)
}

// Post performs a POST request to the Proxmox API.
func (proxmoxAdapter *ProxmoxAdapter) Post(ctx context.Context, path string, body, result any) error {
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}
	return proxmoxAdapter.client.Post(ctx, path, body, result)
}

// Put performs a PUT request to the Proxmox API.
func (proxmoxAdapter *ProxmoxAdapter) Put(ctx context.Context, path string, body, result any) error {
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}
	return proxmoxAdapter.client.Put(ctx, path, body, result)
}

// Delete performs a DELETE request to the Proxmox API.
func (proxmoxAdapter *ProxmoxAdapter) Delete(ctx context.Context, path string, result any) error {
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}
	return proxmoxAdapter.client.Delete(ctx, path, result)
}

// ResolveNode returns node if non-nil, otherwise selects the first available cluster node.
func (proxmoxAdapter *ProxmoxAdapter) ResolveNode(ctx context.Context, node *string) (string, error) {
	if node != nil {
		return *node, nil
	}
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return "", err
	}
	cluster, err := proxmoxAdapter.client.Cluster(ctx)
	if err != nil {
		return "", err
	}
	if len(cluster.Nodes) == 0 {
		return "", errors.New("no nodes found in the cluster")
	}
	return cluster.Nodes[0].Name, nil
}

// NextVMID returns the next available VM ID from the cluster.
func (proxmoxAdapter *ProxmoxAdapter) NextVMID(ctx context.Context) (int, error) {
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return 0, err
	}
	cluster, err := proxmoxAdapter.client.Cluster(ctx)
	if err != nil {
		return 0, err
	}
	vmID, err := cluster.NextID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get next VM ID: %v", err)
	}
	return vmID, nil
}

// Node returns the Proxmox node with the given name.
func (proxmoxAdapter *ProxmoxAdapter) Node(ctx context.Context, name string) (*api.Node, error) {
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return nil, err
	}
	return proxmoxAdapter.client.Node(ctx, name)
}

// FindVirtualMachine finds a virtual machine by its ID and returns the VM, node, and cluster.
// If lastKnownNode is provided, it checks that node first before scanning the cluster.
func (proxmoxAdapter *ProxmoxAdapter) FindVirtualMachine(
	ctx context.Context,
	vmID int,
	lastKnownNode *string,
) (*api.VirtualMachine, *api.Node, *api.Cluster, error) {
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return nil, nil, nil, err
	}

	logger := p.GetLogger(ctx)

	cluster, err := proxmoxAdapter.client.Cluster(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	var vm *api.VirtualMachine
	var node *api.Node

	if lastKnownNode != nil {
		vm, node, err = proxmoxAdapter.FindVirtualMachineOnNode(ctx, vmID, *lastKnownNode)
		if err == nil {
			return vm, node, cluster, nil
		}

		logger.Debugf("VM not found on last known node '%v': %v", *lastKnownNode, err)
	}

	for _, nodeStatus := range cluster.Nodes {
		vm, node, err = proxmoxAdapter.FindVirtualMachineOnNode(ctx, vmID, nodeStatus.Name)
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
func (proxmoxAdapter *ProxmoxAdapter) FindVirtualMachineOnNode(
	ctx context.Context,
	vmID int,
	nodeName string,
) (*api.VirtualMachine, *api.Node, error) {
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return nil, nil, err
	}

	node, err := proxmoxAdapter.client.Node(ctx, nodeName)
	if err != nil {
		return nil, nil, err
	}

	vm, err := node.VirtualMachine(ctx, vmID)
	if vm == nil {
		return nil, node, fmt.Errorf("VM with ID %d not found on '%v' nodes", vmID, nodeName)
	}

	return vm, node, err
}
