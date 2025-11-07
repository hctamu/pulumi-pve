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

// Package vm_test contains comprehensive tests for VM resource operations.
package vm_test

import (
	"context"
	"testing"

	vmResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a pointer to a string
func strPtr(s string) *string {
	return &s
}

// Helper function to create a pointer to an int
func intPtr(i int) *int {
	return &i
}

func TestVMDiffDisksChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		inputDisks    []*vmResource.Disk
		stateDisks    []*vmResource.Disk
		expectChange  bool
		expectDiffKey string
	}{
		{
			name: "disk size changed",
			inputDisks: []*vmResource.Disk{
				{
					Size:      50,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "disk interface changed",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi1",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "disk storage changed",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  false, // Same size and interface
			expectDiffKey: "",
		},
		{
			name: "disk added",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
				{
					Size:      50,
					Interface: "scsi1",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "disk removed",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
				{
					Size:      50,
					Interface: "scsi1",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "no disk changes",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  false,
			expectDiffKey: "",
		},
		{
			name:          "both empty",
			inputDisks:    []*vmResource.Disk{},
			stateDisks:    []*vmResource.Disk{},
			expectChange:  false,
			expectDiffKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:  strPtr("test-vm"),
					Disks: tt.inputDisks,
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:  strPtr("test-vm"),
						Disks: tt.stateDisks,
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, "Expected changes to be detected")
				assert.Contains(t, resp.DetailedDiff, tt.expectDiffKey, "Expected diff key to be present")
				assert.Equal(t, p.Update, resp.DetailedDiff[tt.expectDiffKey].Kind)
			} else {
				assert.False(t, resp.HasChanges, "Expected no changes")
			}
		})
	}
}

func TestVMDiffEfiDiskChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		inputEfiDisk  *vmResource.EfiDisk
		stateEfiDisk  *vmResource.EfiDisk
		expectChange  bool
		expectDiffKey string
	}{
		{
			name: "efi disk added",
			inputEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			stateEfiDisk:  nil,
			expectChange:  true,
			expectDiffKey: "efidisk",
		},
		{
			name:          "efi disk removed",
			inputEfiDisk:  nil,
			stateEfiDisk:  &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
			expectChange:  true,
			expectDiffKey: "efidisk",
		},
		{
			name:         "efi disk type changed",
			inputEfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
			stateEfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType2M},
			expectChange: true,
		},
		{
			name:         "efi disk unchanged",
			inputEfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
			stateEfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
			expectChange: false,
		},
		{
			name:         "both nil",
			inputEfiDisk: nil,
			stateEfiDisk: nil,
			expectChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:    strPtr("test-vm"),
					EfiDisk: tt.inputEfiDisk,
					Disks:   []*vmResource.Disk{}, // Empty disks to focus on EFI
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:    strPtr("test-vm"),
						EfiDisk: tt.stateEfiDisk,
						Disks:   []*vmResource.Disk{},
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, "Expected changes to be detected")
				if tt.expectDiffKey != "" {
					assert.Contains(t, resp.DetailedDiff, tt.expectDiffKey, "Expected diff key to be present")
				}
			} else {
				assert.False(t, resp.HasChanges, "Expected no changes")
			}
		})
	}
}

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
			stateVMID:    intPtr(100),
			expectChange: false, // Computed field, no change expected
		},
		{
			name:              "vmId changed - should trigger replace",
			inputVMID:         intPtr(200),
			stateVMID:         intPtr(100),
			expectChange:      true,
			expectReplace:     true,
			expectedDiffField: "vmId",
		},
		{
			name:         "vmId unchanged",
			inputVMID:    intPtr(100),
			stateVMID:    intPtr(100),
			expectChange: false,
		},
		{
			name:         "node nil in input, present in state (computed)",
			inputNode:    nil,
			stateNode:    strPtr("pve-node1"),
			expectChange: false, // Computed field, no change expected
		},
		{
			name:              "node changed",
			inputNode:         strPtr("pve-node2"),
			stateNode:         strPtr("pve-node1"),
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
					Name:  strPtr("test-vm"),
					VMID:  tt.inputVMID,
					Node:  tt.inputNode,
					Disks: []*vmResource.Disk{},
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:  strPtr("test-vm"),
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
			inputMemory:  intPtr(4096),
			stateMemory:  intPtr(2048),
			expectChange: true,
		},
		{
			name:         "memory unchanged",
			inputMemory:  intPtr(2048),
			stateMemory:  intPtr(2048),
			expectChange: false,
		},
		{
			name:         "memory cleared (set to nil)",
			inputMemory:  nil,
			stateMemory:  intPtr(2048),
			expectChange: true,
		},
		{
			name:         "memory set from nil",
			inputMemory:  intPtr(2048),
			stateMemory:  nil,
			expectChange: true,
		},
		{
			name:         "cores changed",
			inputCores:   intPtr(4),
			stateCores:   intPtr(2),
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
					Name:   strPtr("test-vm"),
					Memory: tt.inputMemory,
					Cores:  tt.inputCores,
					Disks:  []*vmResource.Disk{},
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:   strPtr("test-vm"),
						Memory: tt.stateMemory,
						Cores:  tt.stateCores,
						Disks:  []*vmResource.Disk{},
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
			Name:   strPtr("new-name"),
			Memory: intPtr(4096),
			Cores:  intPtr(4),
			Disks: []*vmResource.Disk{
				{Size: 50, Interface: "scsi0"},
			},
			EfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
		},
		State: vmResource.Outputs{
			Inputs: vmResource.Inputs{
				Name:   strPtr("old-name"),
				Memory: intPtr(2048),
				Cores:  intPtr(2),
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
	assert.Contains(t, resp.DetailedDiff, "efidisk")

	// All should be updates, not replacements
	for key, diff := range resp.DetailedDiff {
		if key == "vmId" {
			assert.Equal(t, p.UpdateReplace, diff.Kind)
		} else {
			assert.Equal(t, p.Update, diff.Kind)
		}
	}
}

func TestVMDiffDiskFileIDIgnored(t *testing.T) {
	t.Parallel()

	fileID1 := "vm-100-disk-0"
	fileID2 := "vm-100-disk-1"
	storage := "local-lvm"

	tests := []struct {
		name         string
		inputDisks   []*vmResource.Disk
		stateDisks   []*vmResource.Disk
		expectChange bool
		description  string
	}{
		{
			name: "FileID nil in input, present in state - no change",
			inputDisks: []*vmResource.Disk{
				{Size: 40, Interface: "scsi0"},
			},
			stateDisks: []*vmResource.Disk{
				{Size: 40, Interface: "scsi0"},
			},
			expectChange: false,
			description:  "FileID computed by provider should not trigger diff",
		},
		{
			name:         "FileID explicitly set in input, different from state - change",
			inputDisks:   []*vmResource.Disk{},
			stateDisks:   []*vmResource.Disk{},
			expectChange: true,
			description:  "Explicitly set FileID that differs should trigger diff",
		},
	}

	// Manually set embedded fields after construction since diskBase is not exported
	tests[0].inputDisks[0].Storage = storage
	tests[0].inputDisks[0].FileID = nil // User didn't specify

	tests[0].stateDisks[0].Storage = storage
	tests[0].stateDisks[0].FileID = &fileID1 // Computed from API

	// Test 2: Different FileIDs
	disk1 := &vmResource.Disk{Size: 40, Interface: "scsi0"}
	disk1.Storage = storage
	disk1.FileID = &fileID2 // User explicitly set it
	tests[1].inputDisks = append(tests[1].inputDisks, disk1)

	disk2 := &vmResource.Disk{Size: 40, Interface: "scsi0"}
	disk2.Storage = storage
	disk2.FileID = &fileID1
	tests[1].stateDisks = append(tests[1].stateDisks, disk2)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:  strPtr("test-vm"),
					Disks: tt.inputDisks,
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:  strPtr("test-vm"),
						Disks: tt.stateDisks,
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, tt.description)
				assert.Contains(t, resp.DetailedDiff, "disks", "Expected disks to be in diff")
			} else {
				assert.False(t, resp.HasChanges, tt.description)
			}
		})
	}
}
