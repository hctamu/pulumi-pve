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

package sdn_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	sdnResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/sdn"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

type mockSDNOperations struct {
	applyFunc func(ctx context.Context, lockToken string, applyTimeout time.Duration) error
	lockFunc  func(ctx context.Context, retryTimeout time.Duration, allowPending bool) (string, error)
}

func (mock *mockSDNOperations) Lock(
	ctx context.Context,
	retryTimeout time.Duration,
	allowPending bool,
) (string, error) {
	if mock.lockFunc != nil {
		return mock.lockFunc(ctx, retryTimeout, allowPending)
	}
	return "", nil
}

func (mock *mockSDNOperations) Apply(ctx context.Context, lockToken string, applyTimeout time.Duration) error {
	if mock.applyFunc != nil {
		return mock.applyFunc(ctx, lockToken, applyTimeout)
	}
	return nil
}

func TestSDNApplyCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		triggers     map[string]any
		applyErr     error
		dryRun       bool
		expectErr    bool
		expectCalled bool
	}{
		{
			name:         "success with string triggers",
			triggers:     map[string]any{"zone": "my-zone", "version": "1"},
			expectCalled: true,
		},
		{
			name:         "success with complex triggers",
			triggers:     map[string]any{"vmId": 100, "config": map[string]string{"key": "value"}},
			expectCalled: true,
		},
		{
			name:         "success without triggers",
			triggers:     nil,
			expectCalled: true,
		},
		{
			name:         "preview skips apply",
			triggers:     map[string]any{"zone": "my-zone"},
			dryRun:       true,
			expectCalled: false,
		},
		{
			name:         "backend failure",
			applyErr:     errors.New("failed to apply SDN changes: 500 Internal Server Error"),
			expectErr:    true,
			expectCalled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called bool
			mock := &mockSDNOperations{
				applyFunc: func(_ context.Context, _ string, _ time.Duration) error {
					called = true
					return tt.applyErr
				},
			}

			resource := &sdnResource.SDNApply{SDNOps: mock}
			req := infer.CreateRequest[proxmox.SDNApplyInputs]{
				Name:   "test-sdn-apply",
				Inputs: proxmox.SDNApplyInputs{Triggers: tt.triggers},
				DryRun: tt.dryRun,
			}
			resp, err := resource.Create(context.Background(), req)
			if tt.expectErr {
				require.Error(t, err)
				assert.Equal(t, tt.applyErr, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "test-sdn-apply", resp.ID)
				assert.Equal(t, tt.triggers, resp.Output.Triggers)
			}
			assert.Equal(t, tt.expectCalled, called)
		})
	}
}

func TestSDNApplyCreatePassesDefaultTimeoutToLock(t *testing.T) {
	t.Parallel()

	lockTimeout := time.Duration(0)
	applyCalled := false
	mock := &mockSDNOperations{
		lockFunc: func(_ context.Context, retryTimeout time.Duration, allowPending bool) (string, error) {
			lockTimeout = retryTimeout
			assert.True(t, allowPending)
			return "lock-token", nil
		},
		applyFunc: func(_ context.Context, lockToken string, _ time.Duration) error {
			applyCalled = true
			assert.Equal(t, "lock-token", lockToken)
			return nil
		},
	}

	resource := &sdnResource.SDNApply{SDNOps: mock}

	req := infer.CreateRequest[proxmox.SDNApplyInputs]{
		Name:   "test-sdn-apply",
		Inputs: proxmox.SDNApplyInputs{AllowPending: true},
	}

	_, err := resource.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 0*time.Second, lockTimeout)
	assert.True(t, applyCalled)
}

func TestSDNApplyCreatePassesCustomTimeoutToLock(t *testing.T) {
	t.Parallel()

	lockTimeout := time.Duration(0)
	mock := &mockSDNOperations{
		lockFunc: func(_ context.Context, retryTimeout time.Duration, _ bool) (string, error) {
			lockTimeout = retryTimeout
			return "", errors.New("lock busy")
		},
	}

	resource := &sdnResource.SDNApply{SDNOps: mock}

	req := infer.CreateRequest[proxmox.SDNApplyInputs]{
		Name: "test-sdn-apply",
		Inputs: proxmox.SDNApplyInputs{
			LockTimeoutSeconds: 2,
		},
	}

	_, err := resource.Create(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, 2*time.Second, lockTimeout)
}

func TestSDNApplyCreateMissingOps(t *testing.T) {
	t.Parallel()

	resource := &sdnResource.SDNApply{}
	req := infer.CreateRequest[proxmox.SDNApplyInputs]{
		Name:   "test-sdn-apply",
		Inputs: proxmox.SDNApplyInputs{},
	}
	_, err := resource.Create(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "SDNOperations not configured", err.Error())
}

func TestSDNApplyUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		oldTriggers  map[string]any
		newTriggers  map[string]any
		applyErr     error
		dryRun       bool
		expectErr    bool
		expectCalled bool
	}{
		{
			name:         "string trigger value changed",
			oldTriggers:  map[string]any{"zone": "old-zone"},
			newTriggers:  map[string]any{"zone": "new-zone"},
			expectCalled: true,
		},
		{
			name:         "complex trigger changed",
			oldTriggers:  map[string]any{"config": map[string]string{"a": "1"}},
			newTriggers:  map[string]any{"config": map[string]string{"a": "2"}},
			expectCalled: true,
		},
		{
			name:         "trigger added",
			oldTriggers:  nil,
			newTriggers:  map[string]any{"zone": "my-zone"},
			expectCalled: true,
		},
		{
			name:         "preview skips apply",
			newTriggers:  map[string]any{"zone": "new-zone"},
			dryRun:       true,
			expectCalled: false,
		},
		{
			name:         "backend failure",
			newTriggers:  map[string]any{"zone": "my-zone"},
			applyErr:     errors.New("failed to apply SDN changes: 500 Internal Server Error"),
			expectErr:    true,
			expectCalled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called bool
			mock := &mockSDNOperations{
				applyFunc: func(_ context.Context, _ string, _ time.Duration) error {
					called = true
					return tt.applyErr
				},
			}

			resource := &sdnResource.SDNApply{SDNOps: mock}
			req := infer.UpdateRequest[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs]{
				ID:     "test-sdn-apply",
				Inputs: proxmox.SDNApplyInputs{Triggers: tt.newTriggers},
				State: proxmox.SDNApplyOutputs{
					SDNApplyInputs: proxmox.SDNApplyInputs{Triggers: tt.oldTriggers},
				},
				DryRun: tt.dryRun,
			}
			resp, err := resource.Update(context.Background(), req)
			if tt.expectErr {
				require.Error(t, err)
				assert.Equal(t, tt.applyErr, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.newTriggers, resp.Output.Triggers)
			}
			assert.Equal(t, tt.expectCalled, called)
		})
	}
}

func TestSDNApplyUpdateMissingOps(t *testing.T) {
	t.Parallel()

	resource := &sdnResource.SDNApply{}
	req := infer.UpdateRequest[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs]{
		ID:     "test-sdn-apply",
		Inputs: proxmox.SDNApplyInputs{},
	}
	_, err := resource.Update(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "SDNOperations not configured", err.Error())
}

func TestSDNApplyDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ops  *mockSDNOperations
	}{
		{"with ops configured", &mockSDNOperations{}},
		{"without ops", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnResource.SDNApply{SDNOps: tt.ops}
			req := infer.DeleteRequest[proxmox.SDNApplyOutputs]{
				ID:    "test-sdn-apply",
				State: proxmox.SDNApplyOutputs{},
			}
			_, err := resource.Delete(context.Background(), req)
			require.NoError(t, err, "delete should always be a no-op")
		})
	}
}

func TestSDNApplyRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		triggers map[string]any
	}{
		{"with string triggers", map[string]any{"zone": "my-zone"}},
		{"with complex triggers", map[string]any{"config": map[string]int{"x": 42}}},
		{"without triggers", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnResource.SDNApply{SDNOps: &mockSDNOperations{}}
			req := infer.ReadRequest[proxmox.SDNApplyInputs, proxmox.SDNApplyOutputs]{
				ID:     "test-sdn-apply",
				Inputs: proxmox.SDNApplyInputs{Triggers: tt.triggers},
				State: proxmox.SDNApplyOutputs{
					SDNApplyInputs: proxmox.SDNApplyInputs{Triggers: tt.triggers},
				},
			}
			resp, err := resource.Read(context.Background(), req)
			require.NoError(t, err, "read should always be a no-op")
			assert.Equal(t, tt.triggers, resp.State.Triggers)
		})
	}
}
