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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

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
	once          sync.Once
	initErr       error
	PVEConfig     *config.Config
	client        *api.Client
	httpClient    *http.Client
	resolvedURL   string
	resolvedUser  string
	resolvedToken string
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

		proxmoxAdapter.client, proxmoxAdapter.httpClient, proxmoxAdapter.initErr = newClient(
			pveConfig.PveURL,
			pveConfig.PveUser,
			pveConfig.PveToken,
			pveConfig.InsecureSkipVerify,
		)
		proxmoxAdapter.resolvedURL = pveConfig.PveURL
		proxmoxAdapter.resolvedUser = pveConfig.PveUser
		proxmoxAdapter.resolvedToken = pveConfig.PveToken
	})

	if proxmoxAdapter.initErr != nil {
		p.GetLogger(ctx).Errorf("Error creating Proxmox client: %v", proxmoxAdapter.initErr)
		return proxmoxAdapter.initErr
	}

	return nil
}

// newClient creates a new Proxmox client
func newClient(
	pveURL,
	pveUser,
	pveToken string,
	insecureSkipVerify bool,
) (*api.Client, *http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	//nolint:gosec // InsecureSkipVerify is controlled by the user via provider config
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecureSkipVerify}

	httpClient := &http.Client{Transport: transport}

	apiClient := api.NewClient(
		pveURL,
		api.WithAPIToken(pveUser, pveToken),
		api.WithHTTPClient(httpClient),
	)

	return apiClient, httpClient, nil
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

// Put performs a PUT request to the Proxmox API, returning the full response body on error.
func (proxmoxAdapter *ProxmoxAdapter) Put(ctx context.Context, path string, body, result any) error {
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := proxmoxAdapter.resolvedURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(
		"Authorization",
		fmt.Sprintf("PVEAPIToken=%s=%s", proxmoxAdapter.resolvedUser, proxmoxAdapter.resolvedToken),
	)

	res, err := proxmoxAdapter.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	respBody, _ := io.ReadAll(res.Body)

	if res.StatusCode >= 400 {
		if len(respBody) > 0 {
			var errResp struct {
				Message string                     `json:"message"`
				Errors  map[string]json.RawMessage `json:"errors"`
			}
			if jsonErr := json.Unmarshal(respBody, &errResp); jsonErr == nil {
				if details := formatProxmoxErrors(errResp.Errors); details != "" {
					if errResp.Message != "" {
						return fmt.Errorf("%s %s", strings.TrimSpace(errResp.Message), details)
					}
					return errors.New(details)
				}
				if errResp.Message != "" {
					return errors.New(strings.TrimSpace(errResp.Message))
				}
			}
			return fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(respBody)))
		}
		return errors.New(res.Status)
	}

	if result == nil {
		return nil
	}

	var datakey map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &datakey); err != nil {
		return err
	}
	if d, ok := datakey["data"]; ok {
		return json.Unmarshal(d, result)
	}
	return json.Unmarshal(respBody, result)
}

// formatProxmoxErrors converts a Proxmox API "errors" map (field -> reason) into a
// deterministic, human-readable "field: reason" list. Returns an empty string when
// there are no field-level errors.
func formatProxmoxErrors(fieldErrors map[string]json.RawMessage) string {
	if len(fieldErrors) == 0 {
		return ""
	}

	details := make([]string, 0, len(fieldErrors))
	for field, reason := range fieldErrors {
		details = append(details, fmt.Sprintf("%s: %s", field, strings.Trim(string(reason), `"`)))
	}
	sort.Strings(details)

	return strings.Join(details, ", ")
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

// WaitForTask waits for a Proxmox task to complete, polling every interval up to timeout.
// interval=0 uses the default production poll interval of 5 seconds.
// It returns an error if the task times out, encounters an error, or reports a failure.
// An empty UPID is treated as a no-op and returns nil immediately.
func (proxmoxAdapter *ProxmoxAdapter) WaitForTask(
	ctx context.Context,
	upid string,
	timeout, interval time.Duration,
) error {
	if upid == "" {
		return nil
	}
	if err := proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}
	const defaultInterval = 5 * time.Second
	if interval == 0 {
		interval = defaultInterval
	}
	task := api.NewTask(api.UPID(upid), proxmoxAdapter.client)
	if err := task.Wait(ctx, interval, timeout); err != nil {
		return err
	}
	if task.IsFailed {
		return fmt.Errorf("task failed: %s", task.ExitStatus)
	}
	return nil
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
