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
	"fmt"
	"time"

	pveproxmox "github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

const (
	sdnApplyPath = "/cluster/sdn"
)

// Ensure SDNAdapter implements the SDNOperations interface
var _ pveproxmox.SDNOperations = (*SDNAdapter)(nil)

// SDNAdapter implements pveproxmox.SDNOperations using a ProxmoxClient.
type SDNAdapter struct {
	client pveproxmox.Client
}

// NewSDNAdapter creates a new SDNAdapter wrapping the given ProxmoxClient.
func NewSDNAdapter(client pveproxmox.Client) *SDNAdapter {
	return &SDNAdapter{client: client}
}

// Apply applies pending SDN configuration changes.
// Returns the task UPID for tracking the async operation.
func (adapter *SDNAdapter) Apply(ctx context.Context) error {
	var taskUPID string
	if err := adapter.client.Put(ctx, sdnApplyPath, nil, &taskUPID); err != nil {
		return fmt.Errorf("failed to apply SDN changes: %w", err)
	}

	if err := adapter.client.WaitForTask(ctx, taskUPID, 60*time.Second, 0); err != nil {
		return fmt.Errorf("failed to wait for SDN apply task: %w", err)
	}

	return nil
}
