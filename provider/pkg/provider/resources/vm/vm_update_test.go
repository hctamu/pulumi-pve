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

// TestVMUpdateDisksReconcile verifies that Update() correctly reconciles disk state:
// removing disks absent from desired, resizing disks that have grown, propagating
// live FileIDs into inputs, and forwarding all changes to UpdateConfig.
func TestVMUpdateDisksReconcile(t *testing.T) {
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
		desiredDisks  []*proxmox.Disk
		stateDisks    []*proxmox.Disk
		removeDiskErr error
		resizeDiskErr error
		dryRun        bool

		wantErr         bool
		wantErrContains string
		wantRemoveDisks []string
		wantResizeCalls []resizeCall
		// wantFileIDs maps interface name → expected FileID in request.Inputs.Disks after Update.
		// A nil pointer value asserts that FileID must remain nil.
		wantFileIDs map[string]*string
	}{
		{
			name: "no change propagates FileID and calls UpdateConfig",
			desiredDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
			},
			wantRemoveDisks: nil,
			wantResizeCalls: nil,
			wantFileIDs:     map[string]*string{"scsi0": testutils.Ptr(fileID0)},
		},
		{
			name: "add disk calls UpdateConfig without remove or resize",
			desiredDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi0"},
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 30, Interface: "scsi1"},
			},
			stateDisks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
			},
			wantRemoveDisks: nil,
			wantResizeCalls: nil,
			// scsi1 is DiskAdded: no FileID propagated; scsi0 is DiskUnchanged: FileID propagated.
			wantFileIDs: map[string]*string{
				"scsi0": testutils.Ptr(fileID0),
				"scsi1": nil,
			},
		},
		{
			name: "remove disk calls RemoveDisk then UpdateConfig",
			desiredDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID1)},
					Size:      30,
					Interface: "scsi1",
				},
			},
			wantRemoveDisks: []string{"scsi1"},
			wantResizeCalls: nil,
			wantFileIDs:     map[string]*string{"scsi0": testutils.Ptr(fileID0)},
		},
		{
			name: "resize disk calls ResizeDisk and propagates FileID",
			desiredDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 50, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr(fileID0)},
					Size:      20,
					Interface: "scsi0",
				},
			},
			wantRemoveDisks: nil,
			wantResizeCalls: []resizeCall{{"scsi0", 50}},
			wantFileIDs:     map[string]*string{"scsi0": testutils.Ptr(fileID0)},
		},
		{
			name: "dry run returns without calling any ops",
			desiredDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 50, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi0"},
			},
			dryRun:          true,
			wantRemoveDisks: nil,
			wantResizeCalls: nil,
		},
		{
			name: "RemoveDisk error propagates",
			desiredDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi0"},
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 30, Interface: "scsi1"},
			},
			removeDiskErr:   errors.New("unlink failed"),
			wantErr:         true,
			wantErrContains: "unlink failed",
		},
		{
			name: "ResizeDisk error propagates",
			desiredDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 50, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi0"},
			},
			resizeDiskErr:   errors.New("resize failed"),
			wantErr:         true,
			wantErrContains: "resize failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var removedDisks []string
			resizedDisks := make(map[string]int)
			var updateConfigCalled bool

			ops := &mockVMOperations{
				removeDiskFunc: func(_ context.Context, _ int, _ *string, diskInterface string) error {
					if tt.removeDiskErr != nil {
						return tt.removeDiskErr
					}
					removedDisks = append(removedDisks, diskInterface)
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
					updateConfigCalled = true
					return nil
				},
			}

			vmID := testVMID
			req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID:     "test-vm",
				DryRun: tt.dryRun,
				Inputs: proxmox.VMInputs{
					Name:  "test-vm",
					Node:  testNode,
					VMID:  &vmID,
					Disks: tt.desiredDisks,
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
						Name:  "test-vm",
						Node:  testNode,
						VMID:  &vmID,
						Disks: tt.stateDisks,
					},
				},
			}

			vmInstance := &vmResource.VM{VMOps: ops}
			_, err := vmInstance.Update(context.Background(), req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)

			if tt.dryRun {
				assert.False(t, updateConfigCalled, "UpdateConfig should not be called on dry run")
				return
			}

			assert.True(t, updateConfigCalled, "UpdateConfig should be called")

			// Verify remove calls.
			assert.ElementsMatch(t, tt.wantRemoveDisks, removedDisks, "removed disk interfaces")

			// Verify resize calls.
			wantResizeMap := make(map[string]int, len(tt.wantResizeCalls))
			for _, rc := range tt.wantResizeCalls {
				wantResizeMap[rc.diskInterface] = rc.sizeGB
			}
			assert.Equal(t, wantResizeMap, resizedDisks, "resized disks")

			// Verify FileIDs propagated into request.Inputs.Disks.
			if len(tt.wantFileIDs) > 0 {
				inputsByIface := make(map[string]*proxmox.Disk, len(req.Inputs.Disks))
				for _, d := range req.Inputs.Disks {
					if d != nil {
						inputsByIface[d.Interface] = d
					}
				}
				for iface, wantFileID := range tt.wantFileIDs {
					d := inputsByIface[iface]
					require.NotNilf(t, d, "disk %s should be present in inputs", iface)
					if wantFileID == nil {
						assert.Nilf(t, d.FileID, "FileID for disk %s should be nil", iface)
					} else {
						require.NotNilf(t, d.FileID, "FileID for disk %s should not be nil", iface)
						assert.Equalf(t, *wantFileID, *d.FileID, "FileID for disk %s", iface)
					}
				}
			}
		})
	}
}

// TestVMUpdateSkipsGetCurrentDisksWhenDisksUnchanged verifies that when the desired
// disk list is identical to the prior state, GetCurrentDisks is never called.
// This avoids a redundant Proxmox API round-trip on non-disk-related updates.
func TestVMUpdateSkipsGetCurrentDisksWhenDisksUnchanged(t *testing.T) {
	t.Parallel()

	const testVMID = 100
	testNode := testutils.Ptr("pve-node")
	fileID := "local-lvm:vm-100-disk-0"

	// getCurrentDisksFunc returns an error — if it is called the test will fail.
	ops := &mockVMOperations{
		getCurrentDisksFunc: func(
			_ context.Context, _ int, _ *string,
		) (map[string]proxmox.Disk, *proxmox.EfiDisk, error) {
			return nil, nil, errors.New("GetCurrentDisks must not be called when disks are unchanged")
		},
		getFunc: func(_ context.Context, id int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			return proxmox.VMInputs{VMID: &id}, nil
		},
	}

	vmID := testVMID
	disk := &proxmox.Disk{
		DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
		Size:      20,
		Interface: "scsi0",
	}
	stateDisk := &proxmox.Disk{
		DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: &fileID},
		Size:      20,
		Interface: "scsi0",
	}

	req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "test-vm",
		Inputs: proxmox.VMInputs{
			Name:   "test-vm",
			Node:   testNode,
			VMID:   &vmID,
			Disks:  []*proxmox.Disk{disk},
			Memory: testutils.Ptr(4096), // only memory changed
		},
		State: proxmox.VMOutputs{
			VMInputs: proxmox.VMInputs{
				Name:   "test-vm",
				Node:   testNode,
				VMID:   &vmID,
				Disks:  []*proxmox.Disk{stateDisk},
				Memory: testutils.Ptr(2048),
			},
		},
	}

	vmInstance := &vmResource.VM{VMOps: ops}
	_, err := vmInstance.Update(context.Background(), req)
	require.NoError(t, err, "Update must succeed without calling GetCurrentDisks")
}
