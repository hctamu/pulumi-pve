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

package vm_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

// TestVMDiffTags verifies that Diff detects tag changes correctly.
func TestVMDiffTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputTags    []string
		stateTags    []string
		expectChange bool
	}{
		{
			name:         "tags unchanged - no diff",
			inputTags:    []string{"prod", "web"},
			stateTags:    []string{"prod", "web"},
			expectChange: false,
		},
		{
			name:         "tag added",
			inputTags:    []string{"prod", "web"},
			stateTags:    []string{"prod"},
			expectChange: true,
		},
		{
			name:         "tag removed",
			inputTags:    []string{"prod"},
			stateTags:    []string{"prod", "web"},
			expectChange: true,
		},
		{
			name:         "all tags removed",
			inputTags:    nil,
			stateTags:    []string{"prod"},
			expectChange: true,
		},
		{
			name:         "tags added from none",
			inputTags:    []string{"staging"},
			stateTags:    nil,
			expectChange: true,
		},
		{
			name:         "tag value changed",
			inputTags:    []string{"staging"},
			stateTags:    []string{"prod"},
			expectChange: true,
		},
		{
			name:         "both nil - no diff",
			inputTags:    nil,
			stateTags:    nil,
			expectChange: false,
		},
		{
			name:         "same tags different order - no diff",
			inputTags:    []string{"web", "prod"},
			stateTags:    []string{"prod", "web"},
			expectChange: false,
		},
		{
			name:         "same tags all reordered - no diff",
			inputTags:    []string{"zzz", "aaa", "mmm"},
			stateTags:    []string{"aaa", "mmm", "zzz"},
			expectChange: false,
		},
		{
			name:         "reorder plus extra tag - diff",
			inputTags:    []string{"web", "prod", "staging"},
			stateTags:    []string{"prod", "web"},
			expectChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vmResource := &vm.VM{
				Client: &testutils.MockProxmoxClient{DefaultNode: "pve-node", DefaultVMID: 100},
				VMOps:  &mockVMOperations{},
			}

			req := infer.DiffRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID:     "100",
				Inputs: proxmox.VMInputs{Tags: tt.inputTags},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{Tags: tt.stateTags},
				},
			}

			resp, err := vmResource.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, "expected HasChanges=true for %q", tt.name)
				assert.Contains(t, resp.DetailedDiff, "tags", "expected 'tags' in detailed diff")
			} else {
				assert.False(t, resp.HasChanges, "expected HasChanges=false for %q", tt.name)
				assert.NotContains(t, resp.DetailedDiff, "tags", "expected 'tags' NOT in detailed diff")
			}
		})
	}
}

// TestVMCreatePassesTags verifies that Create stores the correct tags in output state.
// The mock's Get return (mockReturnTags) simulates the normalised adapter response:
// the real adapter returns nil for VMs with no tags (including when the user supplied
// an empty slice and Proxmox echoes back a whitespace-only tags string).
func TestVMCreatePassesTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputTags      []string // tags supplied by the user in Create inputs
		mockReturnTags []string // tags the mock Get returns (simulates normalised adapter)
		expectedTags   []string // expected tags in the output state
	}{
		{
			name:           "nil input - nil state",
			inputTags:      nil,
			mockReturnTags: nil,
			expectedTags:   nil,
		},
		{
			name:           "empty slice input - nil state (adapter normalises whitespace response)",
			inputTags:      []string{},
			mockReturnTags: nil, // fixed adapter returns nil, not [" "], for no-tag VMs
			expectedTags:   nil,
		},
		{
			name:           "single tag forwarded",
			inputTags:      []string{"prod"},
			mockReturnTags: []string{"prod"},
			expectedTags:   []string{"prod"},
		},
		{
			name:           "multiple tags forwarded in order",
			inputTags:      []string{"prod", "web", "frontend"},
			mockReturnTags: []string{"prod", "web", "frontend"},
			expectedTags:   []string{"prod", "web", "frontend"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node := "pve-node"
			id := 100

			vmResource := &vm.VM{
				Client: &testutils.MockProxmoxClient{DefaultNode: node, DefaultVMID: id},
				VMOps: &mockVMOperations{
					createVMFunc: func(_ context.Context, _ proxmox.VMInputs) error {
						return nil
					},
					getFunc: func(_ context.Context, vmID int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
						return proxmox.VMInputs{
							VMID:  &vmID,
							Node:  &node,
							Tags:  tt.mockReturnTags,
							Disks: []*proxmox.Disk{},
						}, nil
					},
				},
			}

			req := infer.CreateRequest[proxmox.VMInputs]{
				Name: "test-vm",
				Inputs: proxmox.VMInputs{
					Name:  "test-vm",
					VMID:  &id,
					Node:  &node,
					Tags:  tt.inputTags,
					Disks: []*proxmox.Disk{},
				},
			}

			resp, err := vmResource.Create(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTags, resp.Output.Tags, "expected output state tags to match normalised mock response")
		})
	}
}

// TestVMReadReturnsTags verifies that Read populates the output state tags from VMOperations.Get.
func TestVMReadReturnsTags(t *testing.T) {
	t.Parallel()

	// mockTags is what the mock's Get returns; it simulates what the real adapter
	// returns after normalisation (always nil, never []string{}, for no-tag VMs).
	tests := []struct {
		name         string
		inputTags    []string // tags the user provides in Read inputs
		mockTags     []string // tags returned by mock Get (simulates normalised adapter)
		expectedTags []string // expected tags in output state
	}{
		{
			name:         "nil input tags - nil state",
			inputTags:    nil,
			mockTags:     nil,
			expectedTags: nil,
		},
		{
			name:         "empty slice input - nil state (adapter normalises whitespace response)",
			inputTags:    []string{},
			mockTags:     nil, // fixed adapter normalises [" "] → nil
			expectedTags: nil,
		},
		{
			name:         "read returns single tag",
			inputTags:    []string{"prod"},
			mockTags:     []string{"prod"},
			expectedTags: []string{"prod"},
		},
		{
			name:         "read returns multiple tags",
			inputTags:    []string{"prod", "web", "frontend"},
			mockTags:     []string{"prod", "web", "frontend"},
			expectedTags: []string{"prod", "web", "frontend"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node := "pve-node"
			id := 100

			vmResource := &vm.VM{
				Client: &testutils.MockProxmoxClient{DefaultNode: node, DefaultVMID: id},
				VMOps: &mockVMOperations{
					getFunc: func(_ context.Context, vmID int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
						return proxmox.VMInputs{
							VMID:  &vmID,
							Node:  &node,
							Tags:  tt.mockTags,
							Disks: []*proxmox.Disk{},
						}, nil
					},
				},
			}

			req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID: "100",
				Inputs: proxmox.VMInputs{
					VMID:  &id,
					Node:  &node,
					Tags:  tt.inputTags,
					Disks: []*proxmox.Disk{},
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
						VMID:  &id,
						Node:  &node,
						Disks: []*proxmox.Disk{},
					},
				},
			}

			resp, err := vmResource.Read(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTags, resp.State.Tags, "expected state tags from Read")
		})
	}
}

// TestVMReadPreservesUserTagOrder verifies that Read preserves the user's tag ordering when
// Proxmox returns the same set of tags in alphabetical order (as it always does).
// After a refresh the stored inputs should reflect the user's original ordering, not Proxmox's,
// so subsequent plans don't show spurious changes.
func TestVMReadPreservesUserTagOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		userInputTags []string // tags as the user wrote them in the YAML
		apiTags       []string // tags as Proxmox returns them (alphabetical)
		expectedTags  []string // tags expected in the preserved inputs after Read
	}{
		{
			name:          "proxmox returns tags alphabetically - user order preserved",
			userInputTags: []string{"web", "prod"},
			apiTags:       []string{"prod", "web"},
			expectedTags:  []string{"web", "prod"},
		},
		{
			name:          "three tags reordered alphabetically - user order preserved",
			userInputTags: []string{"zzz", "aaa", "mmm"},
			apiTags:       []string{"aaa", "mmm", "zzz"},
			expectedTags:  []string{"zzz", "aaa", "mmm"},
		},
		{
			name:          "tags already in api order - unchanged",
			userInputTags: []string{"aaa", "bbb"},
			apiTags:       []string{"aaa", "bbb"},
			expectedTags:  []string{"aaa", "bbb"},
		},
		{
			name:          "different tag sets - api tags returned as-is",
			userInputTags: []string{"prod"},
			apiTags:       []string{"staging"},
			expectedTags:  []string{"staging"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node := "pve-node"
			id := 100

			vmResource := &vm.VM{
				Client: &testutils.MockProxmoxClient{DefaultNode: node, DefaultVMID: id},
				VMOps: &mockVMOperations{
					getFunc: func(_ context.Context, vmID int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
						return proxmox.VMInputs{
							VMID:  &vmID,
							Node:  &node,
							Tags:  tt.apiTags,
							Disks: []*proxmox.Disk{},
						}, nil
					},
				},
			}

			req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID: "100",
				Inputs: proxmox.VMInputs{
					VMID:  &id,
					Node:  &node,
					Tags:  tt.userInputTags,
					Disks: []*proxmox.Disk{},
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
						VMID:  &id,
						Node:  &node,
						Disks: []*proxmox.Disk{},
					},
				},
			}

			resp, err := vmResource.Read(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTags, resp.Inputs.Tags, "expected preserved inputs to have user tag order")
		})
	}
}

// TestVMUpdatePassesTags verifies that Update calls UpdateConfig with the new tags.
func TestVMUpdatePassesTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		newTags      []string
		oldTags      []string
		expectedTags []string
	}{
		{
			name:         "tags updated from one to many",
			newTags:      []string{"staging", "qa"},
			oldTags:      []string{"prod"},
			expectedTags: []string{"staging", "qa"},
		},
		{
			name:         "tags cleared",
			newTags:      nil,
			oldTags:      []string{"prod"},
			expectedTags: nil,
		},
		{
			name:         "tags added to untagged VM",
			newTags:      []string{"prod"},
			oldTags:      nil,
			expectedTags: []string{"prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var receivedNewTags []string
			node := "pve-node"
			id := 100

			vmResource := &vm.VM{
				Client: &testutils.MockProxmoxClient{DefaultNode: node, DefaultVMID: id},
				VMOps: &mockVMOperations{
					updateConfigFunc: func(
						_ context.Context, _ int, _ *string,
						inputs, _ proxmox.VMInputs,
					) error {
						receivedNewTags = inputs.Tags
						return nil
					},
					getFunc: func(_ context.Context, vmID int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
						return proxmox.VMInputs{
							VMID:  &vmID,
							Node:  &node,
							Tags:  tt.newTags,
							Disks: []*proxmox.Disk{},
						}, nil
					},
				},
			}

			req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID: "100",
				Inputs: proxmox.VMInputs{
					Name:  "test-vm",
					VMID:  &id,
					Node:  &node,
					Tags:  tt.newTags,
					Disks: []*proxmox.Disk{},
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
						Name:  "test-vm",
						VMID:  &id,
						Node:  &node,
						Tags:  tt.oldTags,
						Disks: []*proxmox.Disk{},
					},
				},
			}

			_, err := vmResource.Update(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTags, receivedNewTags, "expected UpdateConfig to receive new tags")
		})
	}
}
