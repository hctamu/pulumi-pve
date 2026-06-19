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
	"errors"
	"fmt"
	"time"

	pveproxmox "github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

const (
	sdnApplyPath            = "/cluster/sdn"
	sdnLockPath             = "/cluster/sdn/lock"
	defaultLockRetryTimeout = 60 * time.Second
	lockRetryInterval       = 2 * time.Second
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

// Lock acquires the SDN cluster lock and returns a lock token.
// The token is generated server-side by Proxmox.
// The request always includes allow-pending so Proxmox accepts the lock
// even when there are pending changes.
func (adapter *SDNAdapter) Lock(ctx context.Context, retryTimeout time.Duration) (string, error) {
	if retryTimeout <= 0 {
		retryTimeout = defaultLockRetryTimeout
	}

	body := pveproxmox.SDNLockBody{AllowPending: 1}

	deadline := time.Now().Add(retryTimeout)
	var lastErr error

	for {
		var token string
		if err := adapter.client.Post(ctx, sdnLockPath, body, &token); err != nil {
			lastErr = fmt.Errorf("failed to acquire SDN lock: %w", err)
		} else if token == "" {
			lastErr = errors.New("failed to acquire SDN lock: empty lock token returned")
		} else {
			return token, nil
		}

		if !time.Now().Before(deadline) {
			return "", fmt.Errorf("failed to acquire SDN lock within %s: %w", retryTimeout, lastErr)
		}

		if err := sleepWithContext(ctx, lockRetryInterval); err != nil {
			return "", fmt.Errorf("failed to acquire SDN lock: %w", err)
		}
	}
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Apply applies pending SDN configuration changes.
// lockToken must be the token returned by Lock.
// applyTimeout controls how long to wait for the apply task; <= 0 uses the default.
func (adapter *SDNAdapter) Apply(ctx context.Context, lockToken string, applyTimeout time.Duration) error {
	body := pveproxmox.SDNApplyBody{Lock: lockToken, ReleaseLock: 1}

	var taskUPID string
	if err := adapter.client.Put(ctx, sdnApplyPath, body, &taskUPID); err != nil {
		return fmt.Errorf("failed to apply SDN changes: %w", err)
	}

	if applyTimeout <= 0 {
		applyTimeout = 60 * time.Second
	}

	if err := adapter.client.WaitForTask(ctx, taskUPID, applyTimeout, 0); err != nil {
		return fmt.Errorf("failed to wait for SDN apply task: %w", err)
	}

	return nil
}
