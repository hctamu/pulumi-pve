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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	vmResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

func TestVMDiffEfiDiskChange(t *testing.T) {
	t.Parallel()

	fileID1 := "vm-100-disk-efi"
	fileID2 := "vm-100-disk-efi-new"
	storage := "local-lvm"

	tests := []struct {
		name           string
		inputEfiDisk   *proxmox.EfiDisk
		stateEfiDisk   *proxmox.EfiDisk
		expectChange   bool
		expectDiffKeys []string // Changed to support multiple granular keys
		expectDiffKey  string   // Keep for backward compatibility (added/removed)
		description    string
	}{
		{
			name: "efi disk added",
			inputEfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
			stateEfiDisk:  nil,
			expectChange:  true,
			expectDiffKey: "efidisk",
			description:   "Adding EFI disk should trigger diff",
		},
		{
			name:          "efi disk removed",
			inputEfiDisk:  nil,
			stateEfiDisk:  &proxmox.EfiDisk{EfiType: proxmox.EfiType4M},
			expectChange:  true,
			expectDiffKey: "efidisk",
			description:   "Removing EFI disk should trigger diff",
		},
		{
			name:           "efi disk type changed",
			inputEfiDisk:   &proxmox.EfiDisk{EfiType: proxmox.EfiType4M},
			stateEfiDisk:   &proxmox.EfiDisk{EfiType: proxmox.EfiType2M},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.efitype"},
			description:    "Changing EFI type should trigger diff on efitype only",
		},
		{
			name:         "efi disk unchanged",
			inputEfiDisk: &proxmox.EfiDisk{EfiType: proxmox.EfiType4M},
			stateEfiDisk: &proxmox.EfiDisk{EfiType: proxmox.EfiType4M},
			expectChange: false,
			description:  "Identical EFI disk should not trigger diff",
		},
		{
			name:         "both nil",
			inputEfiDisk: nil,
			stateEfiDisk: nil,
			expectChange: false,
			description:  "Both nil should not trigger diff",
		},
		{
			name: "FileID nil in input, present in state - no change",
			inputEfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
			stateEfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
			expectChange: false,
			description:  "FileID computed by provider should not trigger diff",
		},
		{
			name: "FileID explicitly set in input, different from state - change",
			inputEfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
			stateEfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.fileId"},
			description:    "Explicitly set FileID that differs should trigger diff on fileId only",
		},
		{
			name: "FileID same in both - no change",
			inputEfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
			stateEfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
			expectChange: false,
			description:  "Same FileID should not trigger diff",
		},
		{
			name: "PreEnrolledKeys changed from true to false",
			inputEfiDisk: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType4M,
				PreEnrolledKeys: testutils.Ptr(false),
			},
			stateEfiDisk: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType4M,
				PreEnrolledKeys: testutils.Ptr(true),
			},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.preEnrolledKeys"},
			description:    "Changing PreEnrolledKeys should trigger diff on preEnrolledKeys only",
		},
		{
			name: "PreEnrolledKeys added",
			inputEfiDisk: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType4M,
				PreEnrolledKeys: testutils.Ptr(true),
			},
			stateEfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.preEnrolledKeys"},
			description:    "Adding PreEnrolledKeys should trigger diff on preEnrolledKeys only",
		},
		{
			name: "PreEnrolledKeys removed",
			inputEfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
			stateEfiDisk: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType4M,
				PreEnrolledKeys: testutils.Ptr(true),
			},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.preEnrolledKeys"},
			description:    "Removing PreEnrolledKeys should trigger diff on preEnrolledKeys only",
		},
		{
			name: "PreEnrolledKeys unchanged",
			inputEfiDisk: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType4M,
				PreEnrolledKeys: testutils.Ptr(true),
			},
			stateEfiDisk: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType4M,
				PreEnrolledKeys: testutils.Ptr(true),
			},
			expectChange: false,
			description:  "Same PreEnrolledKeys should not trigger diff",
		},
	}

	// Set up FileID scenarios after struct initialization
	// Test 5: FileID nil in input, present in state
	tests[5].inputEfiDisk.Storage = storage
	tests[5].inputEfiDisk.FileID = nil // User didn't specify
	tests[5].stateEfiDisk.Storage = storage
	tests[5].stateEfiDisk.FileID = &fileID1 // Computed from API

	// Test 6: Different FileIDs
	tests[6].inputEfiDisk.Storage = storage
	tests[6].inputEfiDisk.FileID = &fileID2 // User explicitly set it
	tests[6].stateEfiDisk.Storage = storage
	tests[6].stateEfiDisk.FileID = &fileID1

	// Test 7: Same FileIDs
	tests[7].inputEfiDisk.Storage = storage
	tests[7].inputEfiDisk.FileID = &fileID1
	tests[7].stateEfiDisk.Storage = storage
	tests[7].stateEfiDisk.FileID = &fileID1

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{
				Client: &testutils.MockProxmoxClient{DefaultNode: "pve-node", DefaultVMID: 100},
				VMOps:  &mockVMOperations{},
			}
			req := infer.DiffRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID: "100",
				Inputs: proxmox.VMInputs{
					Name:    "test-vm",
					EfiDisk: tt.inputEfiDisk,
					Disks:   []*proxmox.Disk{}, // Empty disks to focus on EFI
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
						Name:    "test-vm",
						EfiDisk: tt.stateEfiDisk,
						Disks:   []*proxmox.Disk{},
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, tt.description)
				if tt.expectDiffKey != "" {
					// For added/removed cases, check the single key
					assert.Contains(t, resp.DetailedDiff, tt.expectDiffKey, "Expected diff key to be present")
				}
				if len(tt.expectDiffKeys) > 0 {
					// For granular changes, verify each expected key is present
					for _, key := range tt.expectDiffKeys {
						assert.Contains(t, resp.DetailedDiff, key, "Expected granular diff key to be present: %s", key)
					}
				}
			} else {
				assert.False(t, resp.HasChanges, tt.description)
			}
		})
	}
}

func TestVMUpdateEfiDiskSuccess(t *testing.T) {
	t.Parallel()

	vmID := 100
	nodeName := "pve-node"

	var updateConfigCalled bool
	vmRes := &vmResource.VM{
		VMOps: &mockVMOperations{
			updateConfigFunc: func(_ context.Context, _ int, _ *string, _ proxmox.VMInputs, _ proxmox.VMInputs) error {
				updateConfigCalled = true
				return nil
			},
			getFunc: func(_ context.Context, id int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
				return proxmox.VMInputs{
					VMID: &id,
					Name: "test-vm",
					EfiDisk: &proxmox.EfiDisk{
						DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
						EfiType:  proxmox.EfiType4M,
					},
				}, nil
			},
		},
	}
	req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID: testutils.Ptr(vmID),
			Name: "test-vm",
			EfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
		},
		State: proxmox.VMOutputs{
			VMInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(vmID),
				Name: "test-vm",
				Node: &nodeName,
				EfiDisk: &proxmox.EfiDisk{
					EfiType: proxmox.EfiType2M,
				},
			},
		},
	}

	req.Inputs.EfiDisk.Storage = "local-lvm"
	req.State.EfiDisk.Storage = "local-lvm"
	req.State.EfiDisk.FileID = testutils.Ptr("vm-100-disk-0")

	resp, err := vmRes.Update(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, proxmox.EfiType4M, resp.Output.EfiDisk.EfiType)
	// FileID should have been copied from state
	assert.Equal(t, "vm-100-disk-0", *resp.Output.EfiDisk.FileID)
	assert.True(t, updateConfigCalled)
}

func TestVMUpdateEfiDiskPreEnrolledKeysChange(t *testing.T) {
	t.Parallel()

	vmID := 100
	nodeName := "pve-node"

	vmRes := &vmResource.VM{
		VMOps: &mockVMOperations{
			getFunc: func(_ context.Context, id int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
				return proxmox.VMInputs{
					VMID: &id,
					Name: "test-vm",
					EfiDisk: &proxmox.EfiDisk{
						DiskBase:        proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
						EfiType:         proxmox.EfiType4M,
						PreEnrolledKeys: testutils.Ptr(true),
					},
				}, nil
			},
		},
	}
	req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID: testutils.Ptr(vmID),
			Name: "test-vm",
			EfiDisk: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType4M,
				PreEnrolledKeys: testutils.Ptr(true),
			},
		},
		State: proxmox.VMOutputs{
			VMInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(vmID),
				Name: "test-vm",
				Node: &nodeName,
				EfiDisk: &proxmox.EfiDisk{
					EfiType: proxmox.EfiType4M,
				},
			},
		},
	}

	req.Inputs.EfiDisk.Storage = "local-lvm"
	req.State.EfiDisk.Storage = "local-lvm"
	req.State.EfiDisk.FileID = testutils.Ptr("vm-100-disk-0")

	resp, err := vmRes.Update(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, *resp.Output.EfiDisk.PreEnrolledKeys)
}

func TestVMReadWithEfiDisk(t *testing.T) {
	t.Parallel()

	vmID := 100
	nodeName := "pve-node"

	fileID := "vm-100-disk-0"
	preEnrolled := true
	vmRes := &vmResource.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID},
		VMOps: &mockVMOperations{
			getFunc: func(_ context.Context, _ int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
				return proxmox.VMInputs{
					VMID: testutils.Ptr(vmID),
					Name: "test-vm",
					EfiDisk: &proxmox.EfiDisk{
						DiskBase:        proxmox.DiskBase{Storage: "local-lvm", FileID: &fileID},
						EfiType:         proxmox.EfiType4M,
						PreEnrolledKeys: &preEnrolled,
					},
				}, nil
			},
		},
	}
	req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID: testutils.Ptr(vmID),
			Node: &nodeName,
		},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "100", resp.ID)
	assert.NotNil(t, resp.State.EfiDisk)
	assert.Equal(t, proxmox.EfiType4M, resp.State.EfiDisk.EfiType)
	assert.NotNil(t, resp.State.EfiDisk.PreEnrolledKeys)
	assert.True(t, *resp.State.EfiDisk.PreEnrolledKeys)
	assert.Equal(t, "local-lvm", resp.State.EfiDisk.Storage)
	assert.NotNil(t, resp.State.EfiDisk.FileID)
	assert.Equal(t, "vm-100-disk-0", *resp.State.EfiDisk.FileID)
}

func TestVMReadWithoutEfiDisk(t *testing.T) {
	t.Parallel()

	vmID := 100
	nodeName := "pve-node"

	vmRes := &vmResource.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID},
		VMOps: &mockVMOperations{
			getFunc: func(_ context.Context, _ int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
				return proxmox.VMInputs{
					VMID: testutils.Ptr(vmID),
					Name: "test-vm",
				}, nil
			},
		},
	}
	req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID: testutils.Ptr(vmID),
			Node: &nodeName,
		},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "100", resp.ID)
	assert.Nil(t, resp.State.EfiDisk)
}

func TestVMCloneRemovesUnwantedEfiDisk(t *testing.T) {
	t.Parallel()

	nodeName := "pve-node"
	sourceVMID := 999
	newVMID := 100

	efiFileID := "vm-100-disk-0"
	var removeEfiDiskCalled bool
	mockOps := &mockVMOperations{
		cloneVMFunc: func(_ context.Context, _ proxmox.VMInputs) error { return nil },
		getCurrentDisksFunc: func(_ context.Context, _ int, _ *string) (map[string]proxmox.Disk, *proxmox.EfiDisk, error) {
			return map[string]proxmox.Disk{}, &proxmox.EfiDisk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
				EfiType:  proxmox.EfiType4M,
			}, nil
		},
		removeEfiDiskFunc: func(_ context.Context, _ int, _ *string) error {
			removeEfiDiskCalled = true
			return nil
		},
		applyConfigFunc: func(_ context.Context, _ int, _ *string, _ proxmox.VMInputs, _ time.Duration) error {
			return nil
		},
		getFunc: func(_ context.Context, _ int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			return proxmox.VMInputs{VMID: testutils.Ptr(newVMID)}, nil
		},
	}

	vmRes := &vmResource.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: newVMID},
		VMOps:  mockOps,
	}
	req := infer.CreateRequest[proxmox.VMInputs]{
		Name: "cloned-vm",
		Inputs: proxmox.VMInputs{
			Name: "cloned-vm",
			Node: &nodeName,
			Clone: &proxmox.Clone{
				VMID:    sourceVMID,
				Timeout: 300,
			},
		},
	}

	resp, err := vmRes.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, req.Name, resp.ID)
	assert.Equal(t, newVMID, *resp.Output.VMID)
	assert.Nil(t, resp.Output.EfiDisk)
	assert.True(t, removeEfiDiskCalled)
}

func TestVMCloneAddsEfiDisk(t *testing.T) {
	t.Parallel()

	nodeName := "pve-node"
	sourceVMID := 999
	newVMID := 100

	efiFileID := "vm-100-disk-0"
	var applyConfigInputs proxmox.VMInputs
	mockOps := &mockVMOperations{
		cloneVMFunc: func(_ context.Context, _ proxmox.VMInputs) error { return nil },
		getCurrentDisksFunc: func(_ context.Context, _ int, _ *string) (map[string]proxmox.Disk, *proxmox.EfiDisk, error) {
			return map[string]proxmox.Disk{}, nil, nil
		},
		applyConfigFunc: func(_ context.Context, _ int, _ *string, inputs proxmox.VMInputs, _ time.Duration) error {
			applyConfigInputs = inputs
			return nil
		},
		getFunc: func(_ context.Context, _ int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			return proxmox.VMInputs{
				VMID: testutils.Ptr(newVMID),
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
					EfiType:  proxmox.EfiType4M,
				},
			}, nil
		},
	}

	vmRes := &vmResource.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: newVMID},
		VMOps:  mockOps,
	}
	req := infer.CreateRequest[proxmox.VMInputs]{
		Name: "cloned-vm-with-efi",
		Inputs: proxmox.VMInputs{
			Name: "cloned-vm",
			Node: &nodeName,
			Clone: &proxmox.Clone{
				VMID:    sourceVMID,
				Timeout: 300,
			},
			EfiDisk: &proxmox.EfiDisk{
				EfiType: proxmox.EfiType4M,
			},
		},
	}

	resp, err := vmRes.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, req.Name, resp.ID)
	assert.Equal(t, newVMID, *resp.Output.VMID)
	assert.NotNil(t, resp.Output.EfiDisk)
	assert.Equal(t, proxmox.EfiType4M, resp.Output.EfiDisk.EfiType)
	// Verify ApplyConfig was called with EFI disk in inputs
	assert.NotNil(t, applyConfigInputs.EfiDisk)
	assert.Equal(t, proxmox.EfiType4M, applyConfigInputs.EfiDisk.EfiType)
}

func TestVMCreateWithEfiDisk(t *testing.T) {
	t.Parallel()

	nodeName := "pve-node"
	newVMID := 100

	efiFileID := "vm-100-disk-0"
	preEnrolled := false
	var capturedInputs proxmox.VMInputs
	mockOps := &mockVMOperations{
		createVMFunc: func(_ context.Context, inputs proxmox.VMInputs) error {
			capturedInputs = inputs
			return nil
		},
		getFunc: func(_ context.Context, _ int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			return proxmox.VMInputs{
				VMID: testutils.Ptr(newVMID),
				Name: "test-vm-with-efi",
				EfiDisk: &proxmox.EfiDisk{
					DiskBase:        proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
					EfiType:         proxmox.EfiType4M,
					PreEnrolledKeys: &preEnrolled,
				},
			}, nil
		},
	}

	vmRes := &vmResource.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: newVMID},
		VMOps:  mockOps,
	}
	req := infer.CreateRequest[proxmox.VMInputs]{
		Name: "test-vm-with-efi",
		Inputs: proxmox.VMInputs{
			Name:   "test-vm-with-efi",
			Node:   &nodeName,
			CPU:    &proxmox.CPU{Cores: testutils.Ptr(2)},
			Memory: testutils.Ptr(2048),
			EfiDisk: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType4M,
				PreEnrolledKeys: testutils.Ptr(false),
			},
		},
	}

	resp, err := vmRes.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, req.Name, resp.ID)
	assert.Equal(t, newVMID, *resp.Output.VMID)
	assert.NotNil(t, resp.Output.EfiDisk)
	assert.Equal(t, proxmox.EfiType4M, resp.Output.EfiDisk.EfiType)
	assert.NotNil(t, resp.Output.EfiDisk.PreEnrolledKeys)
	assert.False(t, *resp.Output.EfiDisk.PreEnrolledKeys)
	// Verify CreateVM was called with EFI disk in inputs
	require.NotNil(t, capturedInputs.EfiDisk)
	assert.Equal(t, proxmox.EfiType4M, capturedInputs.EfiDisk.EfiType)
}
