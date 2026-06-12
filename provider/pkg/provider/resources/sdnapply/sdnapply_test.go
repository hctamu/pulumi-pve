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

package sdnapply_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	sdnapplyResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/sdnapply"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

type mockSdnOperations struct {
	applyFunc func(ctx context.Context) error
}

func (mock *mockSdnOperations) Apply(ctx context.Context) error {
	if mock.applyFunc != nil {
		return mock.applyFunc(ctx)
	}
	return nil
}

func TestSdnApplyCreate(t *testing.T) {
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
			mock := &mockSdnOperations{
				applyFunc: func(_ context.Context) error {
					called = true
					return tt.applyErr
				},
			}

			resource := &sdnapplyResource.SdnApply{SdnOps: mock}
			req := infer.CreateRequest[proxmox.SdnApplyInputs]{
				Name:   "test-sdn-apply",
				Inputs: proxmox.SdnApplyInputs{Triggers: tt.triggers},
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

func TestSdnApplyCreateMissingOps(t *testing.T) {
	t.Parallel()

	resource := &sdnapplyResource.SdnApply{}
	req := infer.CreateRequest[proxmox.SdnApplyInputs]{
		Name:   "test-sdn-apply",
		Inputs: proxmox.SdnApplyInputs{},
	}
	_, err := resource.Create(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "SdnOperations not configured", err.Error())
}

func TestSdnApplyUpdate(t *testing.T) {
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
			mock := &mockSdnOperations{
				applyFunc: func(_ context.Context) error {
					called = true
					return tt.applyErr
				},
			}

			resource := &sdnapplyResource.SdnApply{SdnOps: mock}
			req := infer.UpdateRequest[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs]{
				ID:     "test-sdn-apply",
				Inputs: proxmox.SdnApplyInputs{Triggers: tt.newTriggers},
				State: proxmox.SdnApplyOutputs{
					SdnApplyInputs: proxmox.SdnApplyInputs{Triggers: tt.oldTriggers},
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

func TestSdnApplyUpdateMissingOps(t *testing.T) {
	t.Parallel()

	resource := &sdnapplyResource.SdnApply{}
	req := infer.UpdateRequest[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs]{
		ID:     "test-sdn-apply",
		Inputs: proxmox.SdnApplyInputs{},
	}
	_, err := resource.Update(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "SdnOperations not configured", err.Error())
}

func TestSdnApplyDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ops  *mockSdnOperations
	}{
		{"with ops configured", &mockSdnOperations{}},
		{"without ops", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnapplyResource.SdnApply{SdnOps: tt.ops}
			req := infer.DeleteRequest[proxmox.SdnApplyOutputs]{
				ID:    "test-sdn-apply",
				State: proxmox.SdnApplyOutputs{},
			}
			_, err := resource.Delete(context.Background(), req)
			require.NoError(t, err, "delete should always be a no-op")
		})
	}
}

func TestSdnApplyRead(t *testing.T) {
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

			resource := &sdnapplyResource.SdnApply{SdnOps: &mockSdnOperations{}}
			req := infer.ReadRequest[proxmox.SdnApplyInputs, proxmox.SdnApplyOutputs]{
				ID:     "test-sdn-apply",
				Inputs: proxmox.SdnApplyInputs{Triggers: tt.triggers},
				State: proxmox.SdnApplyOutputs{
					SdnApplyInputs: proxmox.SdnApplyInputs{Triggers: tt.triggers},
				},
			}
			resp, err := resource.Read(context.Background(), req)
			require.NoError(t, err, "read should always be a no-op")
			assert.Equal(t, tt.triggers, resp.State.Triggers)
		})
	}
}
