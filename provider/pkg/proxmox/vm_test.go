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

package proxmox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

func TestCompareDisksByInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		desired  []*proxmox.Disk
		current  []*proxmox.Disk
		expected map[string]proxmox.DiskChangeType
	}{
		{
			name:     "both empty",
			desired:  []*proxmox.Disk{},
			current:  []*proxmox.Disk{},
			expected: map[string]proxmox.DiskChangeType{},
		},
		{
			name: "disk added",
			desired: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			current:  []*proxmox.Disk{},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskAdded},
		},
		{
			name:    "disk removed",
			desired: []*proxmox.Disk{},
			current: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskRemoved},
		},
		{
			name: "disk unchanged",
			desired: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			current: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskUnchanged},
		},
		{
			name: "disk resized",
			desired: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 64, Interface: "scsi0"},
			},
			current: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskResized},
		},
		{
			name: "disk shrunk",
			desired: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 16, Interface: "scsi0"},
			},
			current: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskShrunk},
		},
		{
			name: "disk storage changed",
			desired: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "ceph-pool"}, Size: 32, Interface: "scsi0"},
			},
			current: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskStorageChanged},
		},
		{
			name: "disk fileID changed",
			desired: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("local-lvm:vm-100-disk-1")},
					Size:      32,
					Interface: "scsi0",
				},
			},
			current: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("local-lvm:vm-100-disk-0")},
					Size:      32,
					Interface: "scsi0",
				},
			},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskFileIDChanged},
		},
		{
			name: "nil desired fileID is not a change",
			desired: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: nil}, Size: 32, Interface: "scsi0"},
			},
			current: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("local-lvm:vm-100-disk-0")},
					Size:      32,
					Interface: "scsi0",
				},
			},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskUnchanged},
		},
		{
			name: "storage change takes priority over resize",
			desired: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "ceph-pool"}, Size: 64, Interface: "scsi0"},
			},
			current: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskStorageChanged},
		},
		{
			name: "mixed changes across multiple disks",
			desired: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 64, Interface: "scsi0"},    // resized
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 50, Interface: "scsi1"},    // added
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 100, Interface: "virtio0"}, // unchanged
			},
			current: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},    // resized
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 20, Interface: "scsi2"},    // removed
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 100, Interface: "virtio0"}, // unchanged
			},
			expected: map[string]proxmox.DiskChangeType{
				"scsi0":   proxmox.DiskResized,
				"scsi1":   proxmox.DiskAdded,
				"scsi2":   proxmox.DiskRemoved,
				"virtio0": proxmox.DiskUnchanged,
			},
		},
		{
			name: "nil disk entries are skipped",
			desired: []*proxmox.Disk{
				nil,
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			current: []*proxmox.Disk{
				nil,
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 32, Interface: "scsi0"},
			},
			expected: map[string]proxmox.DiskChangeType{"scsi0": proxmox.DiskUnchanged},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			changes := proxmox.CompareDisksByInterface(tt.desired, tt.current)

			// Build a lookup map from interface → change type for easy assertion.
			byInterface := func(changes []proxmox.DiskChange) map[string]proxmox.DiskChangeType {
				m := make(map[string]proxmox.DiskChangeType, len(changes))
				for _, c := range changes {
					m[c.Interface] = c.Type
				}
				return m
			}(changes)

			assert.Equal(t, tt.expected, byInterface)
		})
	}
}

func TestValidateDiskFlags(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name    string
		disk    proxmox.Disk
		wantErr string // empty means no error expected
	}{
		// iothread: allowed on scsi and virtio only
		{name: "iothread on scsi0 ok", disk: proxmox.Disk{Interface: "scsi0", IOThread: boolPtr(true)}},
		{name: "iothread on virtio0 ok", disk: proxmox.Disk{Interface: "virtio0", IOThread: boolPtr(true)}},
		{
			name:    "iothread on sata0 rejected",
			disk:    proxmox.Disk{Interface: "sata0", IOThread: boolPtr(true)},
			wantErr: "iothread is only supported on scsi and virtio interfaces",
		},
		{
			name:    "iothread on ide1 rejected",
			disk:    proxmox.Disk{Interface: "ide1", IOThread: boolPtr(false)},
			wantErr: "iothread is only supported on scsi and virtio interfaces",
		},

		// ro: allowed on scsi and virtio only
		{name: "ro on scsi1 ok", disk: proxmox.Disk{Interface: "scsi1", ReadOnly: boolPtr(true)}},
		{name: "ro on virtio1 ok", disk: proxmox.Disk{Interface: "virtio1", ReadOnly: boolPtr(false)}},
		{
			name:    "ro on sata0 rejected",
			disk:    proxmox.Disk{Interface: "sata0", ReadOnly: boolPtr(true)},
			wantErr: "ro (read-only) is only supported on scsi and virtio interfaces",
		},
		{
			name:    "ro on ide0 rejected",
			disk:    proxmox.Disk{Interface: "ide0", ReadOnly: boolPtr(true)},
			wantErr: "ro (read-only) is only supported on scsi and virtio interfaces",
		},

		// ssd: not allowed on virtio
		{name: "ssd on scsi0 ok", disk: proxmox.Disk{Interface: "scsi0", SSD: boolPtr(true)}},
		{name: "ssd on sata1 ok", disk: proxmox.Disk{Interface: "sata1", SSD: boolPtr(true)}},
		{name: "ssd on ide0 ok", disk: proxmox.Disk{Interface: "ide0", SSD: boolPtr(false)}},
		{
			name:    "ssd on virtio0 rejected",
			disk:    proxmox.Disk{Interface: "virtio0", SSD: boolPtr(true)},
			wantErr: "ssd emulation is not supported on virtio interfaces",
		},

		// nil flags are always fine
		{name: "nil flags on any interface ok", disk: proxmox.Disk{Interface: "sata0"}},

		// nil disk is fine
		{name: "nil disk ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var d *proxmox.Disk
			if tt.disk.Interface != "" || tt.disk.IOThread != nil || tt.disk.ReadOnly != nil || tt.disk.SSD != nil {
				d = &tt.disk
			}
			err := proxmox.ValidateDiskFlags(d)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
