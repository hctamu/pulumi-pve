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

	vmResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestVMDiffComputedFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		inputVMID         *int
		stateVMID         *int
		inputNode         *string
		stateNode         *string
		expectChange      bool
		expectReplace     bool
		expectedDiffField string
	}{
		{
			name:         "vmId nil in input, present in state (computed)",
			inputVMID:    nil,
			stateVMID:    testutils.Ptr(100),
			expectChange: false, // Computed field, no change expected
		},
		{
			name:              "vmId changed - should trigger replace",
			inputVMID:         testutils.Ptr(200),
			stateVMID:         testutils.Ptr(100),
			expectChange:      true,
			expectReplace:     true,
			expectedDiffField: "vmId",
		},
		{
			name:         "vmId unchanged",
			inputVMID:    testutils.Ptr(100),
			stateVMID:    testutils.Ptr(100),
			expectChange: false,
		},
		{
			name:         "node nil in input, present in state (computed)",
			inputNode:    nil,
			stateNode:    testutils.Ptr("pve-node1"),
			expectChange: false, // Computed field, no change expected
		},
		{
			name:              "node changed",
			inputNode:         testutils.Ptr("pve-node2"),
			stateNode:         testutils.Ptr("pve-node1"),
			expectChange:      true,
			expectedDiffField: "node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:  testutils.Ptr("test-vm"),
					VMID:  tt.inputVMID,
					Node:  tt.inputNode,
					Disks: []*vmResource.Disk{},
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:  testutils.Ptr("test-vm"),
						VMID:  tt.stateVMID,
						Node:  tt.stateNode,
						Disks: []*vmResource.Disk{},
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, "Expected changes to be detected")
				if tt.expectReplace {
					assert.Equal(t, p.UpdateReplace, resp.DetailedDiff[tt.expectedDiffField].Kind)
					assert.True(t, resp.DeleteBeforeReplace)
				} else if tt.expectedDiffField != "" {
					assert.Equal(t, p.Update, resp.DetailedDiff[tt.expectedDiffField].Kind)
				}
			} else {
				assert.False(t, resp.HasChanges, "Expected no changes")
			}
		})
	}
}

func TestVMDiffPointerFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputMemory  *int
		stateMemory  *int
		inputCores   *int
		stateCores   *int
		expectChange bool
	}{
		{
			name:         "memory changed",
			inputMemory:  testutils.Ptr(4096),
			stateMemory:  testutils.Ptr(2048),
			expectChange: true,
		},
		{
			name:         "memory unchanged",
			inputMemory:  testutils.Ptr(2048),
			stateMemory:  testutils.Ptr(2048),
			expectChange: false,
		},
		{
			name:         "memory cleared (set to nil)",
			inputMemory:  nil,
			stateMemory:  testutils.Ptr(2048),
			expectChange: true,
		},
		{
			name:         "memory set from nil",
			inputMemory:  testutils.Ptr(2048),
			stateMemory:  nil,
			expectChange: true,
		},
		{
			name:         "cores changed",
			inputCores:   testutils.Ptr(4),
			stateCores:   testutils.Ptr(2),
			expectChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:   testutils.Ptr("test-vm"),
					Memory: tt.inputMemory,
					Cpu: &vmResource.Cpu{
						Cores: tt.inputCores,
					},
					Disks: []*vmResource.Disk{},
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:   testutils.Ptr("test-vm"),
						Memory: tt.stateMemory,
						Cpu: &vmResource.Cpu{
							Cores: tt.stateCores,
						},
						Disks: []*vmResource.Disk{},
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectChange, resp.HasChanges)
		})
	}
}

func TestVMDiffMultipleChanges(t *testing.T) {
	t.Parallel()

	vm := &vmResource.VM{}
	req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
		ID: "100",
		Inputs: vmResource.Inputs{
			Name:   testutils.Ptr("new-name"),
			Memory: testutils.Ptr(4096),
			Cpu: &vmResource.Cpu{
				Cores: testutils.Ptr(4),
			},
			Disks: []*vmResource.Disk{
				{Size: 50, Interface: "scsi0"},
			},
			EfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
		},
		State: vmResource.Outputs{
			Inputs: vmResource.Inputs{
				Name:   testutils.Ptr("old-name"),
				Memory: testutils.Ptr(2048),
				Cpu: &vmResource.Cpu{
					Cores: testutils.Ptr(2),
				},
				Disks: []*vmResource.Disk{
					{Size: 40, Interface: "scsi0"},
				},
				EfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType2M},
			},
		},
	}

	resp, err := vm.Diff(context.Background(), req)
	require.NoError(t, err)

	assert.True(t, resp.HasChanges)
	assert.Contains(t, resp.DetailedDiff, "name")
	assert.Contains(t, resp.DetailedDiff, "memory")
	assert.Contains(t, resp.DetailedDiff, "cores")
	assert.Contains(t, resp.DetailedDiff, "disks")
	// EfiDisk now produces granular diffs
	assert.Contains(t, resp.DetailedDiff, "efidisk.efitype")

	// All should be updates, not replacements
	for key, diff := range resp.DetailedDiff {
		if key == "vmId" {
			assert.Equal(t, p.UpdateReplace, diff.Kind)
		} else {
			assert.Equal(t, p.Update, diff.Kind)
		}
	}
}
