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

package vm

import (
	"context"
	"testing"
	"time"

	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

// mockVMOps is a test double for VMOperations that captures calls and returns
// configurable responses.
type mockVMOps struct {
	createVMFunc    func(ctx context.Context, inputs proxmox.VMInputs) error
	cloneVMFunc     func(ctx context.Context, inputs proxmox.VMInputs) error
	applyConfigFunc func(
		ctx context.Context, vmID int, node *string,
		inputs proxmox.VMInputs, timeout time.Duration,
	) error
	getCurrentDisksFunc func(
		ctx context.Context, vmID int, node *string,
	) (map[string]proxmox.Disk, *proxmox.EfiDisk, error)
	resizeDiskFunc    func(ctx context.Context, vmID int, node *string, diskInterface string, sizeGB int) error
	removeDiskFunc    func(ctx context.Context, vmID int, node *string, diskInterface string) error
	removeEfiDiskFunc func(ctx context.Context, vmID int, node *string) error
	getFunc           func(
		ctx context.Context, vmID int, node *string, userDisks []*proxmox.Disk,
	) (proxmox.VMInputs, error)
	updateConfigFunc func(
		ctx context.Context, vmID int, node *string,
		inputs proxmox.VMInputs, stateInputs proxmox.VMInputs,
	) error
	deleteFunc func(ctx context.Context, vmID int, node *string) error
}

func (mock *mockVMOps) CreateVM(ctx context.Context, inputs proxmox.VMInputs) error {
	if mock.createVMFunc != nil {
		return mock.createVMFunc(ctx, inputs)
	}
	return nil
}

func (mock *mockVMOps) CloneVM(ctx context.Context, inputs proxmox.VMInputs) error {
	if mock.cloneVMFunc != nil {
		return mock.cloneVMFunc(ctx, inputs)
	}
	return nil
}

func (mock *mockVMOps) ApplyConfig(
	ctx context.Context, vmID int, node *string, inputs proxmox.VMInputs, timeout time.Duration,
) error {
	if mock.applyConfigFunc != nil {
		return mock.applyConfigFunc(ctx, vmID, node, inputs, timeout)
	}
	return nil
}

func (mock *mockVMOps) GetCurrentDisks(
	ctx context.Context, vmID int, node *string,
) (map[string]proxmox.Disk, *proxmox.EfiDisk, error) {
	if mock.getCurrentDisksFunc != nil {
		return mock.getCurrentDisksFunc(ctx, vmID, node)
	}
	return nil, nil, nil
}

func (mock *mockVMOps) ResizeDisk(
	ctx context.Context, vmID int, node *string, diskInterface string, sizeGB int,
) error {
	if mock.resizeDiskFunc != nil {
		return mock.resizeDiskFunc(ctx, vmID, node, diskInterface, sizeGB)
	}
	return nil
}

func (mock *mockVMOps) RemoveDisk(
	ctx context.Context, vmID int, node *string, diskInterface string,
) error {
	if mock.removeDiskFunc != nil {
		return mock.removeDiskFunc(ctx, vmID, node, diskInterface)
	}
	return nil
}

func (mock *mockVMOps) RemoveEfiDisk(ctx context.Context, vmID int, node *string) error {
	if mock.removeEfiDiskFunc != nil {
		return mock.removeEfiDiskFunc(ctx, vmID, node)
	}
	return nil
}

func (mock *mockVMOps) Get(
	ctx context.Context,
	vmID int,
	node *string,
	userDisks []*proxmox.Disk,
) (proxmox.VMInputs, error) {
	if mock.getFunc != nil {
		return mock.getFunc(ctx, vmID, node, userDisks)
	}
	return proxmox.VMInputs{VMID: &vmID}, nil
}

func (mock *mockVMOps) UpdateConfig(
	ctx context.Context,
	vmID int,
	node *string,
	inputs proxmox.VMInputs,
	stateInputs proxmox.VMInputs,
) error {
	if mock.updateConfigFunc != nil {
		return mock.updateConfigFunc(ctx, vmID, node, inputs, stateInputs)
	}
	return nil
}

func (mock *mockVMOps) Delete(ctx context.Context, vmID int, node *string) error {
	if mock.deleteFunc != nil {
		return mock.deleteFunc(ctx, vmID, node)
	}
	return nil
}

// createMockVMWithConfig creates a minimal api.VirtualMachine with basic fields populated
// for testing ConvertVMConfigToInputs and preservation logic.
func createMockVMWithConfig(node string, vmid uint64, diskConfigs map[string]string, efi string) *api.VirtualMachine {
	cfg := &api.VirtualMachineConfig{
		Name:        "test-vm",
		Description: "desc",
	}

	// Populate disk fields based on interface name
	for iface, conf := range diskConfigs {
		switch iface {
		case "scsi0":
			cfg.SCSI0 = conf
		case "scsi1":
			cfg.SCSI1 = conf
		case "virtio0":
			cfg.VirtIO0 = conf
		case "ide2":
			cfg.IDE2 = conf
		case "sata0":
			cfg.SATA0 = conf
		}
	}
	if efi != "" {
		cfg.EFIDisk0 = efi
	}

	return &api.VirtualMachine{
		Node:                 node,
		VMID:                 api.StringOrUint64(vmid),
		VirtualMachineConfig: cfg,
	}
}

func TestConvertAndPreserve_NoVMIDOrNodeInPrev(t *testing.T) {
	t.Parallel()

	// Prev inputs: user did not provide vmId/node and omitted disk/efi file IDs
	prev := proxmox.VMInputs{
		Disks: []*proxmox.Disk{{
			DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
			Interface: "scsi0",
			Size:      32,
		}},
		// No proxmox.EfiDisk in prev
	}

	// API VM returns concrete vmId/node and file IDs
	vm := createMockVMWithConfig(
		"pve-node1",
		100,
		map[string]string{
			"scsi0": "local-lvm:vm-100-disk-0,size=32G",
		},
		"local-lvm:vm-100-efidisk,size=1G,efitype=4m,pre-enrolled-keys=0",
	)

	computed, err := adapters.ConvertVMConfigToInputs(vm, prev.Disks)
	require.NoError(t, err)

	preserved := preserveInputs(computed, prev)

	// Computed should carry values from API
	require.NotNil(t, computed.VMID)
	assert.Equal(t, 100, *computed.VMID)
	require.NotNil(t, computed.Node)
	assert.Equal(t, "pve-node1", *computed.Node)
	require.NotNil(t, computed.Disks)
	require.Len(t, computed.Disks, 1)
	require.NotNil(t, computed.Disks[0].FileID)
	assert.Equal(t, "vm-100-disk-0", *computed.Disks[0].FileID)
	// Efi present in computed
	require.NotNil(t, computed.EfiDisk)
	require.NotNil(t, computed.EfiDisk.FileID)

	// Preserve emptiness based on prev (returned from ConvertVMConfigToInputs)

	// VMID and Node must remain nil in proxmox.VMInputs
	assert.Nil(t, preserved.VMID)
	assert.Nil(t, preserved.Node)
	// proxmox.Disk FileID cleared because prev omitted it
	require.Len(t, preserved.Disks, 1)
	assert.Nil(t, preserved.Disks[0].FileID)
	// EFI was present in VM config; include it in preserved inputs
	require.NotNil(t, preserved.EfiDisk)
	require.NotNil(t, preserved.EfiDisk.FileID)
	assert.Equal(t, "vm-100-efidisk", *preserved.EfiDisk.FileID)
}

func TestConvertAndPreserve_WithVMIDAndNodeInPrev(t *testing.T) {
	t.Parallel()

	vmid := 100
	node := "pve-node1"
	prev := proxmox.VMInputs{
		VMID: &vmid,
		Node: &node,
		EfiDisk: &proxmox.EfiDisk{ // present but without FileID
			DiskBase: proxmox.DiskBase{Storage: "local-lvm"},
			EfiType:  proxmox.EfiType4M,
		},
		Disks: []*proxmox.Disk{{
			DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
			Interface: "scsi0",
			Size:      32,
		}},
	}

	vm := createMockVMWithConfig(
		node,
		100,
		map[string]string{
			"scsi0": "local-lvm:vm-100-disk-0,size=32G",
		},
		"local-lvm:vm-100-efidisk,size=1G,efitype=4m,pre-enrolled-keys=1",
	)

	computed, err := adapters.ConvertVMConfigToInputs(vm, prev.Disks)
	require.NoError(t, err)

	preserved := preserveInputs(computed, prev)

	// VMID and Node should remain set
	require.NotNil(t, preserved.VMID)
	assert.Equal(t, vmid, *preserved.VMID)
	require.NotNil(t, preserved.Node)
	assert.Equal(t, node, *preserved.Node)

	// proxmox.Disk FileID still nil because prev omitted it
	require.Len(t, preserved.Disks, 1)
	assert.Nil(t, preserved.Disks[0].FileID)

	// proxmox.EfiDisk remains present but FileID stays nil because prev omitted it
	require.NotNil(t, preserved.EfiDisk)
	assert.Nil(t, preserved.EfiDisk.FileID)
}

// --- Update IO handling tests ---

func TestUpdateCopiesVMIDAndNodeFromState(t *testing.T) {
	t.Parallel()

	vm := &VM{}
	stateVMID := 123
	stateNode := "pve-node1"

	req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID:     "vm-123",
		DryRun: true,
		Inputs: proxmox.VMInputs{ /* VMID & Node omitted */ },
		State:  proxmox.VMOutputs{VMInputs: proxmox.VMInputs{VMID: &stateVMID, Node: &stateNode}},
	}

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)

	// response.Output should carry VMID and Node copied from prior state
	require.NotNil(t, resp.Output.VMID)
	assert.Equal(t, stateVMID, *resp.Output.VMID)
	require.NotNil(t, resp.Output.Node)
	assert.Equal(t, stateNode, *resp.Output.Node)
}

func TestUpdateCopiesDiskFileIDsFromState(t *testing.T) {
	t.Parallel()

	vm := &VM{}
	stateVMID := 200
	stateNode := "pve-node2"

	req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID:     "vm-200",
		DryRun: true,
		Inputs: proxmox.VMInputs{
			// Disks omit FileID but have same interfaces
			Disks: []*proxmox.Disk{
				{Interface: "scsi0", DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32},
				{Interface: "scsi1", DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 64},
			},
		},
		State: proxmox.VMOutputs{VMInputs: proxmox.VMInputs{
			VMID: &stateVMID,
			Node: &stateNode,
			Disks: []*proxmox.Disk{
				{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-200-disk-0")},
					Size:      32,
				},
				{
					Interface: "scsi1",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-200-disk-1")},
					Size:      64,
				},
			},
		}},
	}

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)

	// VMID/Node should be copied as well
	require.NotNil(t, resp.Output.VMID)
	require.NotNil(t, resp.Output.Node)

	// FileIDs should be copied for both disks
	require.Len(t, resp.Output.Disks, 2)
	if assert.NotNil(t, resp.Output.Disks[0].FileID) {
		assert.Equal(t, "vm-200-disk-0", *resp.Output.Disks[0].FileID)
	}
	if assert.NotNil(t, resp.Output.Disks[1].FileID) {
		assert.Equal(t, "vm-200-disk-1", *resp.Output.Disks[1].FileID)
	}
}

func TestUpdateCopiesEfiFileIDFromState(t *testing.T) {
	t.Parallel()

	vm := &VM{}
	stateVMID := 300
	stateNode := "pve-node3"

	req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID:     "vm-300",
		DryRun: true,
		Inputs: proxmox.VMInputs{
			// proxmox.EfiDisk present but fileId omitted
			EfiDisk: &proxmox.EfiDisk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm"},
				EfiType:  proxmox.EfiType4M,
			},
		},
		State: proxmox.VMOutputs{VMInputs: proxmox.VMInputs{
			VMID: &stateVMID,
			Node: &stateNode,
			EfiDisk: &proxmox.EfiDisk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-300-efidisk")},
				EfiType:  proxmox.EfiType4M,
			},
		}},
	}

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)

	// EFI disk FileID should be copied from state
	require.NotNil(t, resp.Output.EfiDisk)
	require.NotNil(t, resp.Output.EfiDisk.FileID)
	assert.Equal(t, "vm-300-efidisk", *resp.Output.EfiDisk.FileID)
}

func TestUpdateDoesNotOverwriteUserProvidedFileIDs(t *testing.T) {
	t.Parallel()

	vm := &VM{}
	stateVMID := 400
	stateNode := "pve-node4"

	req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID:     "vm-400",
		DryRun: true,
		Inputs: proxmox.VMInputs{
			Disks: []*proxmox.Disk{
				{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("custom-file")},
					Size:      32,
				},
			},
		},
		State: proxmox.VMOutputs{VMInputs: proxmox.VMInputs{
			VMID: &stateVMID,
			Node: &stateNode,
			Disks: []*proxmox.Disk{
				{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-400-disk-0")},
					Size:      32,
				},
			},
		}},
	}

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)

	// User-provided FileID should remain unchanged
	require.Len(t, resp.Output.Disks, 1)
	if assert.NotNil(t, resp.Output.Disks[0].FileID) {
		assert.Equal(t, "custom-file", *resp.Output.Disks[0].FileID)
	}
}

// --- Read handling tests using mockVMOps ---

func TestVMReadComputedAndPreserved_NoPrevIDs(t *testing.T) {
	t.Parallel()

	vmID := 200
	nodeName := "pve-node"
	fileID := "vm-200-disk-0"
	efiFileID := "vm-200-efidisk"

	ops := &mockVMOps{
		getFunc: func(
			_ context.Context, id int, node *string, _ []*proxmox.Disk,
		) (proxmox.VMInputs, error) {
			return proxmox.VMInputs{
				VMID: &id,
				Node: testutils.Ptr(nodeName),
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: &fileID},
					Size:      32,
				}},
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
					EfiType:  proxmox.EfiType4M,
				},
			}, nil
		},
	}

	vmRes := &VM{VMOps: ops, Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID}}
	req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "200",
		Inputs: proxmox.VMInputs{
			Node: &nodeName,
			Disks: []*proxmox.Disk{{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Interface: "scsi0",
				Size:      32,
			}},
		},
		State: proxmox.VMOutputs{VMInputs: proxmox.VMInputs{VMID: testutils.Ptr(vmID)}},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "200", resp.ID)

	// proxmox.VMOutputs (state) carry computed values from API
	require.NotNil(t, resp.State.VMID)
	assert.Equal(t, vmID, *resp.State.VMID)
	require.Len(t, resp.State.Disks, 1)
	require.NotNil(t, resp.State.EfiDisk)

	// proxmox.VMInputs preserve emptiness where user previously omitted
	assert.Nil(t, resp.Inputs.VMID)
	require.NotNil(t, resp.Inputs.Node)
	assert.Equal(t, nodeName, *resp.Inputs.Node)
	require.Len(t, resp.Inputs.Disks, 1)
	assert.Nil(t, resp.Inputs.Disks[0].FileID)
	// EFI was added on Proxmox; include it in preserved inputs with computed values
	require.NotNil(t, resp.Inputs.EfiDisk)
	require.NotNil(t, resp.Inputs.EfiDisk.FileID)
	assert.Equal(t, efiFileID, *resp.Inputs.EfiDisk.FileID)
}

func TestVMReadComputedAndPreserved_WithPrevIDs(t *testing.T) {
	t.Parallel()

	vmID := 300
	nodeName := "pve-node"
	diskFileID := "vm-300-disk-0"
	efiFileID := "vm-300-efidisk"

	ops := &mockVMOps{
		getFunc: func(
			_ context.Context, id int, node *string, _ []*proxmox.Disk,
		) (proxmox.VMInputs, error) {
			return proxmox.VMInputs{
				VMID: &id,
				Node: testutils.Ptr(nodeName),
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: &diskFileID},
					Size:      32,
				}},
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
					EfiType:  proxmox.EfiType4M,
				},
			}, nil
		},
	}

	vmRes := &VM{VMOps: ops, Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID}}
	req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "300",
		Inputs: proxmox.VMInputs{
			VMID: testutils.Ptr(vmID),
			Node: &nodeName,
			Disks: []*proxmox.Disk{{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Interface: "scsi0",
				Size:      32,
			}},
			EfiDisk: &proxmox.EfiDisk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm"},
				EfiType:  proxmox.EfiType4M,
			},
		},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "300", resp.ID)

	// proxmox.VMOutputs carry computed values from API
	require.NotNil(t, resp.State.VMID)
	assert.Equal(t, vmID, *resp.State.VMID)
	require.Len(t, resp.State.Disks, 1)
	require.NotNil(t, resp.State.EfiDisk)

	// proxmox.VMInputs preserve user-provided VMID/Node and keep FileIDs nil when omitted
	require.NotNil(t, resp.Inputs.VMID)
	assert.Equal(t, vmID, *resp.Inputs.VMID)
	require.NotNil(t, resp.Inputs.Node)
	assert.Equal(t, nodeName, *resp.Inputs.Node)
	require.Len(t, resp.Inputs.Disks, 1)
	assert.Nil(t, resp.Inputs.Disks[0].FileID)
	require.NotNil(t, resp.Inputs.EfiDisk)
	assert.Nil(t, resp.Inputs.EfiDisk.FileID)
}

// --- Create handling test ---

func TestVMCreateOutputsContainComputedValues(t *testing.T) {
	t.Parallel()

	nodeName := "pve-node"
	nextID := 500
	diskFileID := "vm-500-disk-0"
	efiFileID := "vm-500-efidisk"

	ops := &mockVMOps{
		createVMFunc: func(_ context.Context, _ proxmox.VMInputs) error {
			return nil
		},
		getFunc: func(_ context.Context, id int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			return proxmox.VMInputs{
				VMID: &id,
				Node: testutils.Ptr(nodeName),
				Name: "test-vm",
				Disks: []*proxmox.Disk{{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: &diskFileID},
					Interface: "scsi0",
					Size:      32,
				}},
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
					EfiType:  proxmox.EfiType4M,
				},
			}, nil
		},
	}

	vmRes := &VM{VMOps: ops, Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: nextID}}
	req := infer.CreateRequest[proxmox.VMInputs]{
		Name: "500",
		Inputs: proxmox.VMInputs{
			Name: "test-vm",
			Node: &nodeName,
			Disks: []*proxmox.Disk{{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Interface: "scsi0",
				Size:      32,
			}},
			EfiDisk: &proxmox.EfiDisk{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, EfiType: proxmox.EfiType4M},
		},
	}

	resp, err := vmRes.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "500", resp.ID)

	// Outputs contain computed VMID resolved during create
	require.NotNil(t, resp.Output.VMID)
	assert.Equal(t, nextID, *resp.Output.VMID)

	// Node is preserved (user provided it)
	require.NotNil(t, resp.Output.Node)
	assert.Equal(t, nodeName, *resp.Output.Node)

	// Disk FileID is saved in state from the API read-back
	require.Len(t, resp.Output.Disks, 1)
	require.NotNil(t, resp.Output.Disks[0].FileID)
	assert.Equal(t, diskFileID, *resp.Output.Disks[0].FileID)

	// EFI disk FileID is saved in state from the API read-back
	require.NotNil(t, resp.Output.EfiDisk)
	require.NotNil(t, resp.Output.EfiDisk.FileID)
	assert.Equal(t, efiFileID, *resp.Output.EfiDisk.FileID)

	// Original request inputs should not have been mutated with computed VMID
	assert.Nil(t, req.Inputs.VMID)
}

// --- Clone preservation tests ---

// TestVMReadPreservesCloneFromInputs verifies that when the user provides clone
// configuration in their inputs, it is preserved in the returned Inputs after a Read
// (since Proxmox does not expose clone metadata via the VM config API).
func TestVMReadPreservesCloneFromInputs(t *testing.T) {
	t.Parallel()

	vmID := 600
	nodeName := "pve-node"

	userClone := &proxmox.Clone{
		VMID:      9000,
		FullClone: testutils.Ptr(true),
		Timeout:   300,
	}

	inputs := proxmox.VMInputs{
		VMID:  testutils.Ptr(vmID),
		Node:  &nodeName,
		Clone: userClone,
	}

	tests := []struct {
		name      string
		reqInputs proxmox.VMInputs
		reqState  proxmox.VMOutputs
		wantClone *proxmox.Clone
	}{
		{
			name:      "clone in inputs is preserved when state has no clone",
			reqInputs: inputs,
			reqState:  proxmox.VMOutputs{VMInputs: inputs},
			wantClone: userClone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ops := &mockVMOps{
				getFunc: func(_ context.Context, id int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
					// API returns state without clone info
					return proxmox.VMInputs{
						VMID: &id,
						Node: testutils.Ptr(nodeName),
					}, nil
				},
			}

			vmRes := &VM{VMOps: ops, Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID}}
			req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID:     "600",
				Inputs: tt.reqInputs,
				State:  tt.reqState,
			}

			resp, err := vmRes.Read(context.Background(), req)
			require.NoError(t, err)

			require.Equal(t, tt.wantClone, resp.Inputs.Clone, "clone should be preserved in Inputs")
			require.Equal(t, tt.wantClone, resp.State.Clone, "clone should be preserved in State")
		})
	}
}

func TestPreserveInputs_ZeroValueFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		state      proxmox.VMInputs
		userInputs proxmox.VMInputs
		check      func(t *testing.T, preserved proxmox.VMInputs)
	}{
		{
			name: "balloon 0 preserved when API returns nil",
			state: proxmox.VMInputs{
				Balloon: nil, // API returned 0 → adapter returns nil
			},
			userInputs: proxmox.VMInputs{
				Balloon: testutils.Ptr(0), // user explicitly disabled
			},
			check: func(t *testing.T, preserved proxmox.VMInputs) {
				require.NotNil(t, preserved.Balloon)
				assert.Equal(t, 0, *preserved.Balloon)
			},
		},
		{
			name: "balloon non-zero not overwritten",
			state: proxmox.VMInputs{
				Balloon: testutils.Ptr(256), // API returned 256
			},
			userInputs: proxmox.VMInputs{
				Balloon: testutils.Ptr(512), // user wanted 512 (drift scenario)
			},
			check: func(t *testing.T, preserved proxmox.VMInputs) {
				require.NotNil(t, preserved.Balloon)
				assert.Equal(t, 256, *preserved.Balloon, "should keep API value for drift detection")
			},
		},
		{
			name: "autostart 0 preserved when API returns nil",
			state: proxmox.VMInputs{
				Autostart: nil,
			},
			userInputs: proxmox.VMInputs{
				Autostart: testutils.Ptr(0),
			},
			check: func(t *testing.T, preserved proxmox.VMInputs) {
				require.NotNil(t, preserved.Autostart)
				assert.Equal(t, 0, *preserved.Autostart)
			},
		},
		{
			name: "autostart non-zero not overwritten",
			state: proxmox.VMInputs{
				Autostart: testutils.Ptr(1),
			},
			userInputs: proxmox.VMInputs{
				Autostart: testutils.Ptr(0), // user wants disabled but API shows enabled
			},
			check: func(t *testing.T, preserved proxmox.VMInputs) {
				require.NotNil(t, preserved.Autostart)
				assert.Equal(t, 1, *preserved.Autostart, "should keep API value for drift detection")
			},
		},
		{
			name: "template 0 preserved when API returns nil",
			state: proxmox.VMInputs{
				Template: nil,
			},
			userInputs: proxmox.VMInputs{
				Template: testutils.Ptr(0),
			},
			check: func(t *testing.T, preserved proxmox.VMInputs) {
				require.NotNil(t, preserved.Template)
				assert.Equal(t, 0, *preserved.Template)
			},
		},
		{
			name: "numa false preserved when API returns nil",
			state: proxmox.VMInputs{
				CPU: &proxmox.CPU{Cores: testutils.Ptr(2), Numa: nil},
			},
			userInputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{Cores: testutils.Ptr(2), Numa: testutils.Ptr(false)},
			},
			check: func(t *testing.T, preserved proxmox.VMInputs) {
				require.NotNil(t, preserved.CPU)
				require.NotNil(t, preserved.CPU.Numa)
				assert.False(t, *preserved.CPU.Numa)
			},
		},
		{
			name: "numa not overwritten when API returns true",
			state: proxmox.VMInputs{
				CPU: &proxmox.CPU{Cores: testutils.Ptr(2), Numa: testutils.Ptr(true)},
			},
			userInputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{Cores: testutils.Ptr(2), Numa: testutils.Ptr(false)},
			},
			check: func(t *testing.T, preserved proxmox.VMInputs) {
				require.NotNil(t, preserved.CPU)
				require.NotNil(t, preserved.CPU.Numa)
				assert.True(t, *preserved.CPU.Numa, "should keep API value for drift detection")
			},
		},
		{
			name: "user nil fields stay nil",
			state: proxmox.VMInputs{
				Balloon:   nil,
				Autostart: nil,
				Template:  nil,
			},
			userInputs: proxmox.VMInputs{
				Balloon:   nil, // user never set these
				Autostart: nil,
				Template:  nil,
			},
			check: func(t *testing.T, preserved proxmox.VMInputs) {
				assert.Nil(t, preserved.Balloon)
				assert.Nil(t, preserved.Autostart)
				assert.Nil(t, preserved.Template)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			preserved := preserveInputs(tt.state, tt.userInputs)
			tt.check(t, preserved)
		})
	}
}

func TestVMReadStatePreservesZeroValueFields(t *testing.T) {
	t.Parallel()

	vmID := 500
	nodeName := "pve-node"

	ops := &mockVMOps{
		getFunc: func(_ context.Context, id int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			// Simulate API: returns nil for all zero-value fields
			return proxmox.VMInputs{
				VMID:      &id,
				Node:      testutils.Ptr(nodeName),
				Name:      "zero-val-vm",
				Balloon:   nil, // API returned 0 → adapter intOrNil(0) = nil
				Autostart: nil, // same
				Template:  nil, // same
				CPU:       &proxmox.CPU{Cores: testutils.Ptr(2), Numa: nil},
			}, nil
		},
	}

	vmRes := &VM{VMOps: ops, Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID}}
	req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "500",
		Inputs: proxmox.VMInputs{
			VMID:      testutils.Ptr(vmID),
			Node:      &nodeName,
			Balloon:   testutils.Ptr(0), // user explicitly disabled balloon
			Autostart: testutils.Ptr(0), // user explicitly disabled autostart
			Template:  testutils.Ptr(0), // user explicitly set not-template
			CPU:       &proxmox.CPU{Cores: testutils.Ptr(2), Numa: testutils.Ptr(false)},
		},
		State: proxmox.VMOutputs{VMInputs: proxmox.VMInputs{VMID: testutils.Ptr(vmID)}},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)

	// Inputs should have zero-value fields preserved
	require.NotNil(t, resp.Inputs.Balloon)
	assert.Equal(t, 0, *resp.Inputs.Balloon)
	require.NotNil(t, resp.Inputs.Autostart)
	assert.Equal(t, 0, *resp.Inputs.Autostart)
	require.NotNil(t, resp.Inputs.Template)
	assert.Equal(t, 0, *resp.Inputs.Template)
	require.NotNil(t, resp.Inputs.CPU)
	require.NotNil(t, resp.Inputs.CPU.Numa)
	assert.False(t, *resp.Inputs.CPU.Numa)

	// State (written to state file) must ALSO preserve these fields
	// to prevent spurious diffs on next `pulumi up`
	require.NotNil(t, resp.State.Balloon, "state file must record balloon=0")
	assert.Equal(t, 0, *resp.State.Balloon)
	require.NotNil(t, resp.State.Autostart, "state file must record autostart=0")
	assert.Equal(t, 0, *resp.State.Autostart)
	require.NotNil(t, resp.State.Template, "state file must record template=0")
	assert.Equal(t, 0, *resp.State.Template)
	require.NotNil(t, resp.State.CPU)
	require.NotNil(t, resp.State.CPU.Numa, "state file must record numa=false")
	assert.False(t, *resp.State.CPU.Numa)
}

// --- preserveCreateState vs preserveInputs tests ---

func TestPreserveCreateState_KeepsFileIDs(t *testing.T) {
	t.Parallel()

	diskFileID := "vm-100-disk-0"
	efiFileID := "vm-100-efidisk"
	nodeName := "pve-node"

	tests := []struct {
		name       string
		state      proxmox.VMInputs
		userInputs proxmox.VMInputs
		check      func(t *testing.T, result proxmox.VMInputs)
	}{
		{
			name: "disk FileID kept when user omitted it",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(100),
				Node: &nodeName,
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: &diskFileID},
					Size:      32,
				}},
			},
			userInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(100),
				Node: &nodeName,
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      32,
				}},
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.Len(t, result.Disks, 1)
				require.NotNil(t, result.Disks[0].FileID, "Create state must keep disk FileID")
				assert.Equal(t, diskFileID, *result.Disks[0].FileID)
			},
		},
		{
			name: "EFI FileID kept when user omitted it",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(100),
				Node: &nodeName,
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
					EfiType:  proxmox.EfiType4M,
				},
			},
			userInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(100),
				Node: &nodeName,
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm"},
					EfiType:  proxmox.EfiType4M,
				},
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.NotNil(t, result.EfiDisk)
				require.NotNil(t, result.EfiDisk.FileID, "Create state must keep EFI FileID")
				assert.Equal(t, efiFileID, *result.EfiDisk.FileID)
			},
		},
		{
			name: "VMID and Node kept even if user hypothetically omitted them",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(100),
				Node: &nodeName,
			},
			userInputs: proxmox.VMInputs{
				// Simulating computed VMID/Node already set on request.Inputs
				VMID: testutils.Ptr(100),
				Node: &nodeName,
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.NotNil(t, result.VMID)
				assert.Equal(t, 100, *result.VMID)
				require.NotNil(t, result.Node)
				assert.Equal(t, nodeName, *result.Node)
			},
		},
		{
			name: "clone info preserved from user inputs",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(100),
				Node: &nodeName,
			},
			userInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(100),
				Node: &nodeName,
				Clone: &proxmox.Clone{
					VMID:      9000,
					FullClone: testutils.Ptr(true),
					Timeout:   300,
				},
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.NotNil(t, result.Clone)
				assert.Equal(t, 9000, result.Clone.VMID)
				require.NotNil(t, result.Clone.FullClone)
				assert.True(t, *result.Clone.FullClone)
			},
		},
		{
			name: "zero-value fields preserved in create state",
			state: proxmox.VMInputs{
				VMID:      testutils.Ptr(100),
				Node:      &nodeName,
				Balloon:   nil,
				Autostart: nil,
				Template:  nil,
				CPU:       &proxmox.CPU{Cores: testutils.Ptr(2), Numa: nil},
			},
			userInputs: proxmox.VMInputs{
				VMID:      testutils.Ptr(100),
				Node:      &nodeName,
				Balloon:   testutils.Ptr(0),
				Autostart: testutils.Ptr(0),
				Template:  testutils.Ptr(0),
				CPU:       &proxmox.CPU{Cores: testutils.Ptr(2), Numa: testutils.Ptr(false)},
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.NotNil(t, result.Balloon)
				assert.Equal(t, 0, *result.Balloon)
				require.NotNil(t, result.Autostart)
				assert.Equal(t, 0, *result.Autostart)
				require.NotNil(t, result.Template)
				assert.Equal(t, 0, *result.Template)
				require.NotNil(t, result.CPU)
				require.NotNil(t, result.CPU.Numa)
				assert.False(t, *result.CPU.Numa)
			},
		},
		{
			name: "tags preserved in user order",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(100),
				Node: &nodeName,
				Tags: []string{"alpha", "beta", "gamma"},
			},
			userInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(100),
				Node: &nodeName,
				Tags: []string{"gamma", "beta", "alpha"},
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.Equal(t, []string{"gamma", "beta", "alpha"}, result.Tags)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := preserveCreateState(tt.state, tt.userInputs)
			tt.check(t, result)
		})
	}
}

func TestPreserveInputs_ClearsFileIDs(t *testing.T) {
	t.Parallel()

	diskFileID := "vm-200-disk-0"
	efiFileID := "vm-200-efidisk"
	nodeName := "pve-node"

	tests := []struct {
		name       string
		state      proxmox.VMInputs
		userInputs proxmox.VMInputs
		check      func(t *testing.T, result proxmox.VMInputs)
	}{
		{
			name: "disk FileID cleared when user omitted it",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(200),
				Node: &nodeName,
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: &diskFileID},
					Size:      32,
				}},
			},
			userInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(200),
				Node: &nodeName,
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      32,
				}},
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.Len(t, result.Disks, 1)
				assert.Nil(t, result.Disks[0].FileID, "Read inputs must clear disk FileID when user omitted it")
			},
		},
		{
			name: "EFI FileID cleared when user omitted it",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(200),
				Node: &nodeName,
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
					EfiType:  proxmox.EfiType4M,
				},
			},
			userInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(200),
				Node: &nodeName,
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm"},
					EfiType:  proxmox.EfiType4M,
				},
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.NotNil(t, result.EfiDisk)
				assert.Nil(t, result.EfiDisk.FileID, "Read inputs must clear EFI FileID when user omitted it")
			},
		},
		{
			name: "VMID cleared when user omitted it",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(200),
				Node: &nodeName,
			},
			userInputs: proxmox.VMInputs{
				Node: &nodeName,
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				assert.Nil(t, result.VMID, "Read inputs must clear VMID when user omitted it")
				require.NotNil(t, result.Node)
			},
		},
		{
			name: "Node cleared when user omitted it",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(200),
				Node: &nodeName,
			},
			userInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(200),
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.NotNil(t, result.VMID)
				assert.Nil(t, result.Node, "Read inputs must clear Node when user omitted it")
			},
		},
		{
			name: "disk FileID kept when user explicitly provided it",
			state: proxmox.VMInputs{
				VMID: testutils.Ptr(200),
				Node: &nodeName,
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: &diskFileID},
					Size:      32,
				}},
			},
			userInputs: proxmox.VMInputs{
				VMID: testutils.Ptr(200),
				Node: &nodeName,
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: &diskFileID},
					Size:      32,
				}},
			},
			check: func(t *testing.T, result proxmox.VMInputs) {
				require.Len(t, result.Disks, 1)
				require.NotNil(t, result.Disks[0].FileID, "Read inputs must keep FileID when user provided it")
				assert.Equal(t, diskFileID, *result.Disks[0].FileID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := preserveInputs(tt.state, tt.userInputs)
			tt.check(t, result)
		})
	}
}

func TestCreateFullLifecycle_FileIDsInState(t *testing.T) {
	t.Parallel()

	nodeName := "pve-node"
	vmID := 700
	diskFileID := "vm-700-disk-0"
	efiFileID := "vm-700-efidisk"

	ops := &mockVMOps{
		createVMFunc: func(_ context.Context, _ proxmox.VMInputs) error {
			return nil
		},
		getFunc: func(_ context.Context, id int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			return proxmox.VMInputs{
				VMID: &id,
				Node: testutils.Ptr(nodeName),
				Name: "lifecycle-vm",
				Disks: []*proxmox.Disk{
					{
						Interface: "scsi0",
						DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: &diskFileID},
						Size:      32,
					},
				},
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
					EfiType:  proxmox.EfiType4M,
				},
			}, nil
		},
	}

	vmRes := &VM{VMOps: ops, Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID}}

	// Create — FileIDs should be in the state
	createReq := infer.CreateRequest[proxmox.VMInputs]{
		Name: "lifecycle-vm",
		Inputs: proxmox.VMInputs{
			Name: "lifecycle-vm",
			Node: &nodeName,
			Disks: []*proxmox.Disk{{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      32,
			}},
			EfiDisk: &proxmox.EfiDisk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm"},
				EfiType:  proxmox.EfiType4M,
			},
		},
	}

	createResp, err := vmRes.Create(context.Background(), createReq)
	require.NoError(t, err)

	// Verify FileIDs are stored in state after Create
	require.Len(t, createResp.Output.Disks, 1)
	require.NotNil(t, createResp.Output.Disks[0].FileID, "disk FileID must be in state after Create")
	assert.Equal(t, diskFileID, *createResp.Output.Disks[0].FileID)
	require.NotNil(t, createResp.Output.EfiDisk)
	require.NotNil(t, createResp.Output.EfiDisk.FileID, "EFI FileID must be in state after Create")
	assert.Equal(t, efiFileID, *createResp.Output.EfiDisk.FileID)

	// Update (dry-run) — FileIDs should propagate from prior state
	updateReq := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID:     "lifecycle-vm",
		DryRun: true,
		Inputs: proxmox.VMInputs{
			Name: "lifecycle-vm",
			Node: &nodeName,
			Disks: []*proxmox.Disk{{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      32,
			}},
			EfiDisk: &proxmox.EfiDisk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm"},
				EfiType:  proxmox.EfiType4M,
			},
		},
		State: createResp.Output,
	}

	updateResp, err := vmRes.Update(context.Background(), updateReq)
	require.NoError(t, err)

	// FileIDs should be copied from state into update output
	require.Len(t, updateResp.Output.Disks, 1)
	require.NotNil(t, updateResp.Output.Disks[0].FileID, "disk FileID must propagate from state to Update output")
	assert.Equal(t, diskFileID, *updateResp.Output.Disks[0].FileID)
	require.NotNil(t, updateResp.Output.EfiDisk)
	require.NotNil(t, updateResp.Output.EfiDisk.FileID, "EFI FileID must propagate from state to Update output")
	assert.Equal(t, efiFileID, *updateResp.Output.EfiDisk.FileID)
}
