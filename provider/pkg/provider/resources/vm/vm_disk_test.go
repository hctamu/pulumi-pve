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

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	vmResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

func TestVMDiffDisksChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputDisks     []*proxmox.Disk
		stateDisks     []*proxmox.Disk
		expectChange   bool
		expectError    bool
		expectDiffKeys map[string]p.DiffKind
	}{
		{
			name: "disk resized",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 50, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			expectChange:   true,
			expectDiffKeys: map[string]p.DiffKind{"disks[0].size": p.Update},
		},
		{
			// Interface rename: scsi0 → scsi1 appears as a Remove + Add, both at index 0
			// since the input and state each have one disk. The diff map ends with the Add.
			name: "disk interface changed (remove old + add new)",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi1"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			expectChange:   true,
			expectDiffKeys: map[string]p.DiffKind{"disks[0]": p.Add},
		},
		{
			name: "disk added",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 50, Interface: "scsi1"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			expectChange:   true,
			expectDiffKeys: map[string]p.DiffKind{"disks[1]": p.Add},
		},
		{
			name: "disk removed",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 50, Interface: "scsi1"},
			},
			expectChange:   true,
			expectDiffKeys: map[string]p.DiffKind{"disks[1]": p.Delete},
		},
		{
			name: "file id changed",
			inputDisks: []*proxmox.Disk{{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("local-lvm:vm-100-disk-1")},
				Size:      40,
				Interface: "scsi0",
			}},
			stateDisks: []*proxmox.Disk{{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("local-lvm:vm-100-disk-0")},
				Size:      40,
				Interface: "scsi0",
			}},
			expectChange:   true,
			expectDiffKeys: map[string]p.DiffKind{"disks[0].filename": p.Update},
		},
		{
			name: "nil fileID in input is not a change",
			inputDisks: []*proxmox.Disk{{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      40,
				Interface: "scsi0",
			}},
			stateDisks: []*proxmox.Disk{{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("local-lvm:vm-100-disk-0")},
				Size:      40,
				Interface: "scsi0",
			}},
			expectChange: false,
		},
		{
			name: "no disk changes",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			expectChange: false,
		},
		{
			name:         "both empty",
			inputDisks:   []*proxmox.Disk{},
			stateDisks:   []*proxmox.Disk{},
			expectChange: false,
		},
		{
			name: "disk shrunk returns error at diff time",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			expectError: true,
		},
		{
			name: "disk storage changed returns error at diff time",
			inputDisks: []*proxmox.Disk{{
				DiskBase:  proxmox.DiskBase{Storage: "ceph-pool"},
				Size:      40,
				Interface: "scsi0",
			}},
			stateDisks: []*proxmox.Disk{{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      40,
				Interface: "scsi0",
			}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vmInstance := &vmResource.VM{}
			req := infer.DiffRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID: "100",
				Inputs: proxmox.VMInputs{
					Name:  "test-vm",
					Disks: tt.inputDisks,
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
						Name:  "test-vm",
						Disks: tt.stateDisks,
					},
				},
			}

			resp, err := vmInstance.Diff(context.Background(), req)
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, "Expected changes to be detected")
				for key, kind := range tt.expectDiffKeys {
					if assert.Contains(t, resp.DetailedDiff, key) {
						assert.Equal(t, kind, resp.DetailedDiff[key].Kind)
					}
				}
			} else {
				assert.False(t, resp.HasChanges, "Expected no changes")
				assert.Empty(t, resp.DetailedDiff, "Expected no diff entries")
			}
		})
	}
}
