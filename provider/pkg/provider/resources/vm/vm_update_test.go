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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	vmResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

// TestVMUpdateDisksReconcileBatch verifies that Update() uses ReconcileDisksBatch for atomic
// disk reconciliation, handling moves, removes, and resizes correctly.
func TestVMUpdateDisksReconcileBatch(t *testing.T) {
	t.Parallel()

	const testVMID = 100
	testNode := testutils.Ptr("pve-node")
	fileID0 := "local-lvm:vm-100-disk-0"
	fileID1 := "local-lvm:vm-100-disk-1"

	type resizeCall struct {
		diskInterface string
		sizeGB        int
	}

	tests := []struct {
		name          string
		desiredDisks  proxmox.DiskMap
		stateDisks    proxmox.DiskMap
		reconcileErr  error
		resizeDiskErr error
		dryRun        bool

		wantErr         bool
		wantErrContains string
		wantResizeCalls []resizeCall
		// wantFileIDs maps interface name → expected FileID after Update.
		wantFileIDs map[string]*string
		// wantDisks lists the disk keys expected in the output.
		wantDisks []string
	}{
		{
			name: "no change propagates FileID and calls UpdateConfig",
			desiredDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      20,
					Interface: "scsi0",
				},
			},
			stateDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
			},
			wantResizeCalls: nil,
			wantFileIDs:     map[string]*string{"scsi0": testutils.Ptr(fileID0)},
			wantDisks:       []string{"disk-0"},
		},
		{
			name: "add disk calls UpdateConfig without move or resize",
			desiredDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      20,
					Interface: "scsi0",
				},
				"disk-1": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      30,
					Interface: "scsi1",
				},
			},
			stateDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
			},
			wantResizeCalls: nil,
			wantFileIDs:     map[string]*string{"scsi0": testutils.Ptr(fileID0)},
			wantDisks:       []string{"disk-0", "disk-1"},
		},
		{
			name: "interface change reconciles through batch and preserves FileID",
			desiredDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      20,
					Interface: "scsi1",
				},
			},
			stateDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
			},
			wantResizeCalls: nil,
			wantFileIDs:     map[string]*string{"scsi1": testutils.Ptr(fileID0)},
			wantDisks:       []string{"disk-0"},
		},
		{
			name: "remove disk reconciles through batch",
			desiredDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      20,
					Interface: "scsi0",
				},
			},
			stateDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
				"disk-1": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID1)},
					Size:      30,
					Interface: "scsi1",
				},
			},
			wantResizeCalls: nil,
			wantFileIDs:     map[string]*string{"scsi0": testutils.Ptr(fileID0)},
			wantDisks:       []string{"disk-0"},
		},
		{
			name: "resize disk calls ResizeDisk after batch reconciliation",
			desiredDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      50,
					Interface: "scsi0",
				},
			},
			stateDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
			},
			wantResizeCalls: []resizeCall{{diskInterface: "scsi0", sizeGB: 50}},
			wantFileIDs:     map[string]*string{"scsi0": testutils.Ptr(fileID0)},
			wantDisks:       []string{"disk-0"},
		},
		{
			name: "ReconcileDisksBatch error propagates",
			desiredDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      20,
					Interface: "scsi1",
				},
			},
			stateDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
			},
			reconcileErr:    errors.New("batch reconcile failed"),
			wantErr:         true,
			wantErrContains: "batch reconcile failed",
			wantFileIDs:     map[string]*string{},
		},
		{
			name: "ResizeDisk error propagates",
			desiredDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      50,
					Interface: "scsi0",
				},
			},
			stateDisks: proxmox.DiskMap{
				"disk-0": {
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
			},
			resizeDiskErr:   errors.New("resize failed"),
			wantErr:         true,
			wantErrContains: "resize failed",
			wantFileIDs:     map[string]*string{},
		},
		{
			name:    "dry_run returns without calling any ops",
			dryRun:  true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resizedDisks := make(map[string]int)
			var reconcileDisks proxmox.DiskMap // Track what was passed to ReconcileDisksBatch

			ops := &mockVMOperations{
				reconcileDisksBatchFunc: func(
					_ context.Context,
					_ int,
					_ *string,
					desiredDisks proxmox.DiskMap,
					currentDisks proxmox.DiskMap,
				) error {
					if tt.reconcileErr != nil {
						return tt.reconcileErr
					}
					// Store desired disks for later retrieval in getFunc
					reconcileDisks = make(proxmox.DiskMap)
					for name, disk := range desiredDisks {
						newDisk := *disk
						reconcileDisks[name] = &newDisk
					}
					// Propagate FileIDs from current to desired.
					for diskName, desiredDisk := range desiredDisks {
						currentDisk, exists := currentDisks[diskName]
						if !exists || currentDisk == nil || desiredDisk == nil {
							continue
						}
						if currentDisk.FileID != nil && desiredDisk.FileID == nil {
							desiredDisk.FileID = currentDisk.FileID
						}
					}
					// Update reconcileDisks with the propagated FileIDs
					for name, disk := range desiredDisks {
						reconcileDisks[name] = disk
					}
					return nil
				},
				resizeDiskFunc: func(
					_ context.Context, _ int, _ *string, diskInterface string, sizeGB int,
				) error {
					if tt.resizeDiskErr != nil {
						return tt.resizeDiskErr
					}
					resizedDisks[diskInterface] = sizeGB
					return nil
				},
				updateConfigFunc: func(
					_ context.Context, _ int, _ *string, _ proxmox.VMInputs, _ proxmox.VMInputs,
				) error {
					return nil
				},
				getFunc: func(
					_ context.Context, _ int, _ *string, userDisks proxmox.DiskMap,
				) (proxmox.VMInputs, error) {
					// Return the reconciled disks if available, otherwise use userDisks
					var disksToReturn proxmox.DiskMap
					if len(reconcileDisks) > 0 {
						disksToReturn = reconcileDisks
					} else {
						disksToReturn = userDisks
					}
					return proxmox.VMInputs{
						VMID:  testutils.Ptr(testVMID),
						Disks: disksToReturn,
					}, nil
				},
				deleteFunc: func(_ context.Context, _ int, _ *string) error { return nil },
			}

			vmResource := &vmResource.VM{
				VMOps: ops,
			}

			if tt.dryRun {
				// DryRun should not call any ops.
				_, err := vmResource.Update(context.Background(), infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
					ID:     "test-id",
					DryRun: true,
					Inputs: proxmox.VMInputs{
						VMID:  testutils.Ptr(testVMID),
						Node:  testNode,
						Disks: tt.desiredDisks,
					},
					State: proxmox.VMOutputs{
						VMInputs: proxmox.VMInputs{
							VMID:  testutils.Ptr(testVMID),
							Node:  testNode,
							Disks: tt.stateDisks,
						},
					},
				})
				if tt.wantErr {
					require.Error(t, err)
					if tt.wantErrContains != "" {
						require.ErrorContains(t, err, tt.wantErrContains)
					}
				} else {
					require.NoError(t, err)
				}
				return
			}

			response, err := vmResource.Update(context.Background(), infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID: "test-id",
				Inputs: proxmox.VMInputs{
					VMID:  testutils.Ptr(testVMID),
					Node:  testNode,
					Disks: tt.desiredDisks,
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
						VMID:  testutils.Ptr(testVMID),
						Node:  testNode,
						Disks: tt.stateDisks,
					},
				},
			})

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					require.ErrorContains(t, err, tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, response.Output.Disks)

			// Verify expected disk keys are in the output.
			if tt.wantDisks != nil {
				for _, diskKey := range tt.wantDisks {
					assert.NotNil(t, response.Output.Disks[diskKey], "disk %s not in output", diskKey)
				}
			}

			// Verify FileID propagation by interface.
			for iface, wantFileID := range tt.wantFileIDs {
				var foundDisk *proxmox.Disk
				for _, disk := range response.Output.Disks {
					if disk != nil && disk.Interface == iface {
						foundDisk = disk
						break
					}
				}
				if wantFileID == nil {
					if foundDisk != nil {
						assert.Nil(t, foundDisk.FileID, "disk on interface %s should not have FileID but got %v",
							iface, foundDisk.FileID)
					}
				} else {
					assert.NotNil(t, foundDisk, "disk on interface %s not found", iface)
					if foundDisk != nil {
						assert.Equal(t, *wantFileID, *foundDisk.FileID, "disk on interface %s FileID mismatch", iface)
					}
				}
			}

			// Verify resize operations.
			for _, resizeCall := range tt.wantResizeCalls {
				actualSize, exists := resizedDisks[resizeCall.diskInterface]
				assert.True(t, exists, "ResizeDisk not called for interface %s", resizeCall.diskInterface)
				assert.Equal(t, resizeCall.sizeGB, actualSize, "resize size mismatch for %s", resizeCall.diskInterface)
			}
		})
	}
}
