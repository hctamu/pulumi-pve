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
		inputDisks     proxmox.DiskList
		stateDisks     proxmox.DiskList
		expectChange   bool
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
			expectDiffKeys: map[string]p.DiffKind{"disks[\"disk-1\"].size": p.Update},
		},
		{
			name: "disk resized and flags changed simultaneously",
			inputDisks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      50,
					Interface: "scsi0",
					Cache:     testutils.Ptr("writeback"),
				},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			expectChange: true,
			expectDiffKeys: map[string]p.DiffKind{
				"disks[\"disk-1\"].size":  p.Update,
				"disks[\"disk-1\"].cache": p.Update,
			},
		},
		{
			name: "disk flags changed and fileID changed simultaneously",
			inputDisks: []*proxmox.Disk{
				{
					DiskBase: proxmox.DiskBase{
						Storage: "local-lvm",
						FileID:  testutils.Ptr("local-lvm:vm-100-disk-1"),
					},
					Size:      40,
					Interface: "scsi0",
					Cache:     testutils.Ptr("writeback"),
				},
			},
			stateDisks: []*proxmox.Disk{
				{
					DiskBase: proxmox.DiskBase{
						Storage: "local-lvm",
						FileID:  testutils.Ptr("local-lvm:vm-100-disk-0"),
					},
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange: true,
			expectDiffKeys: map[string]p.DiffKind{
				"disks[\"disk-1\"].cache":    p.Update,
				"disks[\"disk-1\"].filename": p.Update,
			},
		},
		{
			// Interface rename on the same logical disk is now tracked as a property update.
			name: "disk interface changed (remove old + add new)",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi1"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			expectChange:   true,
			expectDiffKeys: map[string]p.DiffKind{"disks[\"disk-1\"].interface": p.Update},
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
			expectDiffKeys: map[string]p.DiffKind{"disks[\"disk-2\"]": p.Add},
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
			expectDiffKeys: map[string]p.DiffKind{"disks[\"disk-2\"]": p.Delete},
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
			expectDiffKeys: map[string]p.DiffKind{"disks[\"disk-1\"].filename": p.Update},
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
			name: "disk shrunk is a normal diff, not a Diff-time error",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi0"},
			},
			stateDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 40, Interface: "scsi0"},
			},
			expectChange:   true,
			expectDiffKeys: map[string]p.DiffKind{"disks[\"disk-1\"].size": p.Update},
		},
		{
			name: "disk storage changed is a normal diff, not a Diff-time error",
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
			expectChange:   true,
			expectDiffKeys: map[string]p.DiffKind{"disks[\"disk-1\"].storage": p.Update},
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
					Disks: testutils.DiskMap(tt.inputDisks...),
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
						Name:  "test-vm",
						Disks: testutils.DiskMap(tt.stateDisks...),
					},
				},
			}

			resp, err := vmInstance.Diff(context.Background(), req)
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
