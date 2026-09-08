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

func TestNextDiskName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing proxmox.DiskMap
		expected string
	}{
		{
			name:     "empty map starts from disk-0",
			existing: proxmox.DiskMap{},
			expected: "disk-0",
		},
		{
			name: "fills smallest numeric gap",
			existing: proxmox.DiskMap{
				"disk-0": nil,
				"disk-2": nil,
			},
			expected: "disk-1",
		},
		{
			name: "ignores non-standard names while allocating",
			existing: proxmox.DiskMap{
				"database": nil,
				"disk-0":   nil,
			},
			expected: "disk-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, proxmox.NextDiskName(tt.existing))
		})
	}
}

func TestCheckDiskInterfaceMove(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		diskName    string
		fromIface   string
		toIface     string
		otherDisks  proxmox.DiskMap
		wantErr     bool
		errContains string
	}{
		{
			name:      "same-bus scsi move is allowed",
			diskName:  "db",
			fromIface: "scsi0",
			toIface:   "scsi1",
			wantErr:   false,
		},
		{
			name:      "same-bus sata move is allowed",
			diskName:  "db",
			fromIface: "sata0",
			toIface:   "sata2",
			wantErr:   false,
		},
		{
			name:      "cross-bus scsi to sata is allowed",
			diskName:  "db",
			fromIface: "scsi0",
			toIface:   "sata0",
			wantErr:   false,
		},
		{
			name:      "cross-bus virtio to scsi is allowed",
			diskName:  "db",
			fromIface: "virtio0",
			toIface:   "scsi0",
			wantErr:   false,
		},
		{
			name:      "target slot free is allowed",
			diskName:  "db",
			fromIface: "scsi0",
			toIface:   "scsi3",
			otherDisks: proxmox.DiskMap{
				"logs": {Interface: "scsi1", Size: 10, DiskBase: proxmox.DiskBase{Storage: "local-lvm"}},
			},
			wantErr: false,
		},
		{
			name:      "target slot claimed by other disk is rejected",
			diskName:  "db",
			fromIface: "scsi0",
			toIface:   "scsi1",
			otherDisks: proxmox.DiskMap{
				"logs": {Interface: "scsi1", Size: 10, DiskBase: proxmox.DiskBase{Storage: "local-lvm"}},
			},
			wantErr:     true,
			errContains: "already claimed by disk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			desired := proxmox.DiskMap{
				tt.diskName: {Interface: tt.toIface, Size: 20, DiskBase: proxmox.DiskBase{Storage: "local-lvm"}},
			}
			for key, disk := range tt.otherDisks {
				desired[key] = disk
			}
			err := proxmox.CheckDiskInterfaceMove(tt.diskName, tt.fromIface, tt.toIface, desired)
			if tt.wantErr {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestDiskMapFromSliceByIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		disks []*proxmox.Disk
		check func(t *testing.T, actual proxmox.DiskMap)
	}{
		{
			name:  "empty slice returns empty map",
			disks: []*proxmox.Disk{},
			check: func(t *testing.T, actual proxmox.DiskMap) {
				t.Helper()
				assert.Empty(t, actual)
			},
		},
		{
			name: "fileID sorting wins over interface",
			disks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local", FileID: testutils.Ptr("vm-100-disk-9")},
					Interface: "scsi0",
					Size:      20,
				},
				{
					DiskBase:  proxmox.DiskBase{Storage: "local", FileID: testutils.Ptr("vm-100-disk-1")},
					Interface: "virtio0",
					Size:      20,
				},
			},
			check: func(t *testing.T, actual proxmox.DiskMap) {
				t.Helper()
				assert.Equal(t, "vm-100-disk-1", *actual["disk-0"].FileID)
				assert.Equal(t, "vm-100-disk-9", *actual["disk-1"].FileID)
			},
		},
		{
			name: "interface fallback when fileID missing",
			disks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local"}, Interface: "virtio1", Size: 20},
				{DiskBase: proxmox.DiskBase{Storage: "local"}, Interface: "scsi0", Size: 20},
			},
			check: func(t *testing.T, actual proxmox.DiskMap) {
				t.Helper()
				assert.Equal(t, "scsi0", actual["disk-0"].Interface)
				assert.Equal(t, "virtio1", actual["disk-1"].Interface)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := proxmox.DiskMapFromSliceByIdentity(tt.disks)
			tt.check(t, actual)
		})
	}
}

func TestDiskMapByInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		disks    proxmox.DiskMap
		expected map[string]*proxmox.Disk
	}{
		{
			name: "skips nil and empty interface",
			disks: proxmox.DiskMap{
				"nil":      nil,
				"empty":    {Interface: "", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 1},
				"database": {Interface: "scsi0", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 20},
			},
			expected: map[string]*proxmox.Disk{
				"scsi0": {Interface: "scsi0", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 20},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, proxmox.DiskMapByInterface(tt.disks))
		})
	}
}

func TestDiskMapFromCurrentInterfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		current  map[string]proxmox.Disk
		existing proxmox.DiskMap
		check    func(t *testing.T, actual proxmox.DiskMap)
	}{
		{
			name: "preserves matching names and adds new with disk-N",
			current: map[string]proxmox.Disk{
				"scsi0":   {Interface: "scsi0", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 20},
				"virtio1": {Interface: "virtio1", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 30},
			},
			existing: proxmox.DiskMap{
				"database": {Interface: "scsi0", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 10},
			},
			check: func(t *testing.T, actual proxmox.DiskMap) {
				t.Helper()
				assert.Equal(t, "scsi0", actual["database"].Interface)
				assert.Equal(t, "virtio1", actual["disk-0"].Interface)
			},
		},
		{
			name: "fills numeric gap for newly discovered disk",
			current: map[string]proxmox.Disk{
				"scsi0": {Interface: "scsi0", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 20},
				"scsi2": {Interface: "scsi2", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 20},
			},
			existing: proxmox.DiskMap{
				"disk-0": {Interface: "scsi0", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 20},
				"disk-2": {Interface: "missing0", DiskBase: proxmox.DiskBase{Storage: "local"}, Size: 20},
			},
			check: func(t *testing.T, actual proxmox.DiskMap) {
				t.Helper()
				assert.Equal(t, "scsi0", actual["disk-0"].Interface)
				assert.Equal(t, "scsi2", actual["disk-1"].Interface)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := proxmox.DiskMapFromCurrentInterfaces(tt.current, tt.existing)
			tt.check(t, actual)
		})
	}
}
