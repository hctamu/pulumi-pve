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

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// mockVMOps is a test double for VMOperations that captures calls and returns
// configurable responses.
type mockVMOps struct {
	createFunc func(ctx context.Context, inputs proxmox.VMInputs) (int, string, error)
	getFunc    func(ctx context.Context, vmID int, node *string, existingInputs proxmox.VMInputs) (proxmox.VMInputs, proxmox.VMInputs, error)
	updateFunc func(ctx context.Context, vmID int, node *string, inputs proxmox.VMInputs, stateInputs proxmox.VMInputs) error
	deleteFunc func(ctx context.Context, vmID int, node *string) error
}

func (m *mockVMOps) Create(ctx context.Context, inputs proxmox.VMInputs) (int, string, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, inputs)
	}
	return 100, "pve-node", nil
}

func (m *mockVMOps) Get(
	ctx context.Context,
	vmID int,
	node *string,
	existingInputs proxmox.VMInputs,
) (proxmox.VMInputs, proxmox.VMInputs, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, vmID, node, existingInputs)
	}
	return proxmox.VMInputs{VMID: &vmID}, proxmox.VMInputs{VMID: &vmID}, nil
}

func (m *mockVMOps) Update(
	ctx context.Context,
	vmID int,
	node *string,
	inputs proxmox.VMInputs,
	stateInputs proxmox.VMInputs,
) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, vmID, node, inputs, stateInputs)
	}
	return nil
}

func (m *mockVMOps) Delete(ctx context.Context, vmID int, node *string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, vmID, node)
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

	computed, preserved, err := adapters.ConvertVMConfigToInputs(vm, prev)
	require.NoError(t, err)

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

	_, preserved, err := adapters.ConvertVMConfigToInputs(vm, prev)
	require.NoError(t, err)

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
		getFunc: func(_ context.Context, id int, node *string, existing proxmox.VMInputs) (proxmox.VMInputs, proxmox.VMInputs, error) {
			computed := proxmox.VMInputs{
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
			}
			// Preserve: VMID nil (user omitted), disk FileID nil (omitted), EFI FileID included (newly discovered)
			preserved := proxmox.VMInputs{
				Node: testutils.Ptr(nodeName),
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      32,
				}},
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: &efiFileID},
					EfiType:  proxmox.EfiType4M,
				},
			}
			return computed, preserved, nil
		},
	}

	vmRes := &VM{VMOps: ops}
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
		getFunc: func(_ context.Context, id int, node *string, existing proxmox.VMInputs) (proxmox.VMInputs, proxmox.VMInputs, error) {
			computed := proxmox.VMInputs{
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
			}
			// VMID and Node preserved since user provided them; disk/efi FileID nil (omitted)
			preserved := proxmox.VMInputs{
				VMID: &id,
				Node: testutils.Ptr(nodeName),
				Disks: []*proxmox.Disk{{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      32,
				}},
				EfiDisk: &proxmox.EfiDisk{
					DiskBase: proxmox.DiskBase{Storage: "local-lvm"},
					EfiType:  proxmox.EfiType4M,
				},
			}
			return computed, preserved, nil
		},
	}

	vmRes := &VM{VMOps: ops}
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
		createFunc: func(_ context.Context, inputs proxmox.VMInputs) (int, string, error) {
			return nextID, nodeName, nil
		},
		getFunc: func(_ context.Context, id int, node *string, existing proxmox.VMInputs) (proxmox.VMInputs, proxmox.VMInputs, error) {
			computed := proxmox.VMInputs{
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
			}
			return computed, computed, nil
		},
	}

	vmRes := &VM{VMOps: ops}
	req := infer.CreateRequest[proxmox.VMInputs]{
		Name: "500",
		Inputs: proxmox.VMInputs{
			Name: testutils.Ptr("test-vm"),
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

	// proxmox.VMOutputs contain computed VMID
	require.NotNil(t, resp.Output.VMID)
	assert.Equal(t, nextID, *resp.Output.VMID)

	// proxmox.VMInputs should not contain computed VMID (user omitted it)
	assert.Nil(t, req.Inputs.VMID)
}
