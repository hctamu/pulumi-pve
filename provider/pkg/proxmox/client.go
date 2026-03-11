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

// Package proxmox provides interfaces and domain types for interacting with Proxmox VE.
package proxmox

import (
	"context"
)

// Client is the general interface for interacting with Proxmox VE.
// It provides low-level HTTP operations and cluster-level helpers.
type Client interface {
	// Get performs a GET request to the Proxmox API.
	Get(ctx context.Context, path string, result any) error

	// Post performs a POST request to the Proxmox API.
	Post(ctx context.Context, path string, body any, result any) error

	// Put performs a PUT request to the Proxmox API.
	Put(ctx context.Context, path string, body any, result any) error

	// Delete performs a DELETE request to the Proxmox API.
	Delete(ctx context.Context, path string, result any) error

	// ResolveNode returns node if non-nil, otherwise selects the first available
	// node in the cluster.
	ResolveNode(ctx context.Context, node *string) (string, error)

	// NextVMID returns the next available VM ID from the cluster.
	NextVMID(ctx context.Context) (int, error)
}
