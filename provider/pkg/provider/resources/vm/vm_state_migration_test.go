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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

func TestMigrateLegacyVMOutputs(t *testing.T) {
	t.Parallel()

	legacy := legacyVMOutputs{
		legacyVMInputs: legacyVMInputs{
			Name: "legacy-vm",
			VMID: testutils.Ptr(101),
			Node: testutils.Ptr("pve-node"),
			Tags: proxmox.TagList{"prod", "db"},
			Disks: proxmox.DiskList{
				{Interface: "scsi0", DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32},
				{Interface: "scsi1", DiskBase: proxmox.DiskBase{Storage: "ceph-ha"}, Size: 64},
			},
		},
	}

	result, err := migrateLegacyVMOutputs(context.Background(), legacy)
	require.NoError(t, err)
	require.NotNil(t, result.Result)

	migrated := result.Result
	assert.Equal(t, legacy.Name, migrated.Name)
	assert.Equal(t, legacy.VMID, migrated.VMID)
	assert.Equal(t, legacy.Node, migrated.Node)
	assert.Equal(t, legacy.Tags, migrated.Tags)
	require.Len(t, migrated.Disks, 2)

	migratedDisks := testutils.DiskSlice(migrated.Disks)
	assert.Equal(t, "scsi0", migratedDisks[0].Interface)
	assert.Equal(t, "scsi1", migratedDisks[1].Interface)
	assert.Contains(t, migrated.Disks, "disk-1")
	assert.Contains(t, migrated.Disks, "disk-2")
}

func TestMigrateLegacyVMOutputsUsesFilenameFirstIdentity(t *testing.T) {
	t.Parallel()

	legacy := legacyVMOutputs{
		legacyVMInputs: legacyVMInputs{
			Name: "legacy-vm",
			VMID: testutils.Ptr(101),
			Node: testutils.Ptr("pve-node"),
			Disks: proxmox.DiskList{
				{
					Interface: "scsi1",
					DiskBase: proxmox.DiskBase{
						Storage: "local-lvm",
						FileID:  testutils.Ptr("local-lvm:vm-101-disk-1"),
					},
					Size: 64,
				},
				{
					Interface: "scsi0",
					DiskBase: proxmox.DiskBase{
						Storage: "local-lvm",
						FileID:  testutils.Ptr("local-lvm:vm-101-disk-0"),
					},
					Size: 32,
				},
			},
		},
	}

	result, err := migrateLegacyVMOutputs(context.Background(), legacy)
	require.NoError(t, err)
	require.NotNil(t, result.Result)

	disk1, ok := result.Result.Disks["disk-1"]
	require.True(t, ok)
	require.NotNil(t, disk1)
	assert.Equal(t, "scsi0", disk1.Interface)
	require.NotNil(t, disk1.FileID)
	assert.Equal(t, "local-lvm:vm-101-disk-0", *disk1.FileID)

	disk2, ok := result.Result.Disks["disk-2"]
	require.True(t, ok)
	require.NotNil(t, disk2)
	assert.Equal(t, "scsi1", disk2.Interface)
	require.NotNil(t, disk2.FileID)
	assert.Equal(t, "local-lvm:vm-101-disk-1", *disk2.FileID)
}

func TestUpdateAfterLegacyMigrationDoesNotMutateDisks(t *testing.T) {
	t.Parallel()

	legacy := legacyVMOutputs{
		legacyVMInputs: legacyVMInputs{
			Name: "legacy-vm",
			VMID: testutils.Ptr(101),
			Node: testutils.Ptr("pve-node"),
			Disks: proxmox.DiskList{
				{
					Interface: "scsi1",
					DiskBase: proxmox.DiskBase{
						Storage: "local-lvm",
						FileID:  testutils.Ptr("local-lvm:vm-101-disk-1"),
					},
					Size: 64,
				},
				{
					Interface: "scsi0",
					DiskBase: proxmox.DiskBase{
						Storage: "local-lvm",
						FileID:  testutils.Ptr("local-lvm:vm-101-disk-0"),
					},
					Size: 32,
				},
			},
		},
	}

	migrationResult, err := migrateLegacyVMOutputs(context.Background(), legacy)
	require.NoError(t, err)
	require.NotNil(t, migrationResult.Result)

	var removeDiskCalls int
	var resizeDiskCalls int
	var moveDiskCalls int
	var getCalls int
	var updateConfigCalls int

	operations := &mockVMOps{
		removeDiskFunc: func(_ context.Context, _ int, _ *string, _ string) error {
			removeDiskCalls++
			return nil
		},
		resizeDiskFunc: func(_ context.Context, _ int, _ *string, _ string, _ int) error {
			resizeDiskCalls++
			return nil
		},
		moveDiskFunc: func(_ context.Context, _ int, _ *string, _ string, _ string) error {
			moveDiskCalls++
			return nil
		},
		getFunc: func(_ context.Context, vmID int, _ *string, _ proxmox.DiskMap) (proxmox.VMInputs, error) {
			getCalls++
			return proxmox.VMInputs{VMID: &vmID}, nil
		},
		updateConfigFunc: func(_ context.Context, _ int, _ *string, _ proxmox.VMInputs, _ proxmox.VMInputs) error {
			updateConfigCalls++
			return nil
		},
	}

	vmResource := &VM{VMOps: operations}
	updateRequest := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "legacy-vm",
		Inputs: proxmox.VMInputs{
			Name: "legacy-vm",
			Node: legacy.Node,
			VMID: legacy.VMID,
			Disks: proxmox.DiskMap{
				"disk-1": {DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
				"disk-2": {DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 64, Interface: "scsi1"},
			},
		},
		State: *migrationResult.Result,
	}

	_, err = vmResource.Update(context.Background(), updateRequest)
	require.NoError(t, err)
	assert.Equal(t, 1, updateConfigCalls)
	assert.Zero(t, removeDiskCalls, "migration follow-up update must not remove disks")
	assert.Zero(t, resizeDiskCalls, "migration follow-up update must not resize disks")
	assert.Zero(t, moveDiskCalls, "migration follow-up update must not move disks")
	assert.Zero(t, getCalls, "unchanged migrated disks should not trigger a re-read")
}

func TestMigrateLegacyVMOutputsEmptyDisks(t *testing.T) {
	t.Parallel()

	result, err := migrateLegacyVMOutputs(context.Background(), legacyVMOutputs{})
	require.NoError(t, err)
	assert.Nil(t, result.Result)
}

func TestMigrateLegacyVMOutputsNoopWhenDisksAlreadyMapShaped(t *testing.T) {
	t.Parallel()

	legacy := legacyVMOutputs{
		legacyVMInputs: legacyVMInputs{
			Name: "already-new",
			Disks: map[string]any{
				"root": map[string]any{"interface": "scsi0", "storage": "ceph-ha", "size": 8},
			},
		},
	}

	result, err := migrateLegacyVMOutputs(context.Background(), legacy)
	require.NoError(t, err)
	assert.Nil(t, result.Result)
}

func TestDecodeLegacyDiskListAcceptsRawList(t *testing.T) {
	t.Parallel()

	decoded, shouldMigrate, err := decodeLegacyDiskList([]any{
		map[string]any{"interface": "scsi0", "storage": "local-lvm", "size": 32},
		map[string]any{"interface": "scsi1", "storage": "ceph-ha", "size": 64},
	})
	require.NoError(t, err)
	assert.True(t, shouldMigrate)
	require.Len(t, decoded, 2)
	assert.Equal(t, "scsi0", decoded[0].Interface)
	assert.Equal(t, "scsi1", decoded[1].Interface)
}
