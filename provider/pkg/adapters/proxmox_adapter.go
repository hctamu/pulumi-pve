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
	"net/http"
	"sync"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
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
