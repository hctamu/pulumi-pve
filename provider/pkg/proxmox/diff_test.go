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

	p "github.com/pulumi/pulumi-go-provider"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

func TestSDNListDiffers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		differ  proxmox.FieldDiffer
		state   any
		changed bool
	}{
		{
			name:    "peer reordering changes diff",
			differ:  proxmox.PeerList{"node-b", "node-a"},
			state:   proxmox.PeerList{"node-a", "node-b"},
			changed: true,
		},
		{
			name:    "node reordering does not change diff",
			differ:  proxmox.NodeList{"node-b", "node-a"},
			state:   proxmox.NodeList{"node-a", "node-b"},
			changed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.changed, len(tt.differ.DiffFrom("nodes", tt.state)) > 0)
		})
	}
}

func TestTagListDiffFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tags     proxmox.TagList
		state    proxmox.TagList
		expected map[string]p.PropertyDiff
	}{
		{
			name:  "reordering does not change diff",
			tags:  proxmox.TagList{"production", "database"},
			state: proxmox.TagList{"database", "production"},
		},
		{
			name:  "membership change produces update",
			tags:  proxmox.TagList{"production", "database"},
			state: proxmox.TagList{"production"},
			expected: map[string]p.PropertyDiff{
				"tags": {Kind: p.Update},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.tags.DiffFrom("tags", tt.state))
		})
	}
}

func TestDiskListDiffFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		disks    proxmox.DiskList
		state    proxmox.DiskList
		expected map[string]p.PropertyDiff
	}{
		{
			name: "computed filename omitted from inputs does not change diff",
			disks: proxmox.DiskList{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Interface: "scsi0", Size: 20},
			},
			state: proxmox.DiskList{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
					Interface: "scsi0",
					Size:      20,
				},
			},
			expected: map[string]p.PropertyDiff{},
		},
		{
			name: "added disk uses input index",
			disks: proxmox.DiskList{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Interface: "scsi0", Size: 20},
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Interface: "scsi1", Size: 40},
			},
			state: proxmox.DiskList{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Interface: "scsi0", Size: 20},
			},
			expected: map[string]p.PropertyDiff{
				"disks[1]": {Kind: p.Add},
			},
		},
		{
			name: "removed disk uses state index",
			disks: proxmox.DiskList{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Interface: "scsi0", Size: 20},
			},
			state: proxmox.DiskList{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Interface: "scsi0", Size: 20},
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Interface: "scsi1", Size: 40},
			},
			expected: map[string]p.PropertyDiff{
				"disks[1]": {Kind: p.Delete},
			},
		},
		{
			name: "changed disk produces granular property diff",
			disks: proxmox.DiskList{
				{
					DiskBase:  proxmox.DiskBase{Storage: "fast-lvm"},
					Interface: "scsi0",
					Size:      40,
					Cache:     testutils.Ptr("writeback"),
				},
			},
			state: proxmox.DiskList{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Interface: "scsi0",
					Size:      20,
					Cache:     testutils.Ptr("none"),
				},
			},
			expected: map[string]p.PropertyDiff{
				"disks[0].cache":   {Kind: p.Update},
				"disks[0].size":    {Kind: p.Update},
				"disks[0].storage": {Kind: p.Update},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.disks.DiffFrom("disks", tt.state))
		})
	}
}

func TestEfiDiskDiffFrom(t *testing.T) {
	t.Parallel()

	var nilEfiDisk *proxmox.EfiDisk
	tests := []struct {
		name     string
		disk     *proxmox.EfiDisk
		state    *proxmox.EfiDisk
		expected map[string]p.PropertyDiff
	}{
		{
			name: "both absent has no diff",
			disk: nilEfiDisk,
		},
		{
			name: "added disk updates parent property",
			disk: &proxmox.EfiDisk{EfiType: proxmox.EfiType4M},
			expected: map[string]p.PropertyDiff{
				"efidisk": {Kind: p.Update},
			},
		},
		{
			name:  "removed disk updates parent property",
			state: &proxmox.EfiDisk{EfiType: proxmox.EfiType4M},
			expected: map[string]p.PropertyDiff{
				"efidisk": {Kind: p.Update},
			},
		},
		{
			name: "computed filename omitted from inputs does not change diff",
			disk: &proxmox.EfiDisk{EfiType: proxmox.EfiType4M, DiskBase: proxmox.DiskBase{Storage: "local-lvm"}},
			state: &proxmox.EfiDisk{
				EfiType:  proxmox.EfiType4M,
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
			},
			expected: map[string]p.PropertyDiff{},
		},
		{
			name: "changed fields produce granular diffs",
			disk: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType4M,
				PreEnrolledKeys: testutils.Ptr(true),
				DiskBase:        proxmox.DiskBase{Storage: "fast-lvm", FileID: testutils.Ptr("new-file")},
			},
			state: &proxmox.EfiDisk{
				EfiType:         proxmox.EfiType2M,
				PreEnrolledKeys: testutils.Ptr(false),
				DiskBase:        proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("old-file")},
			},
			expected: map[string]p.PropertyDiff{
				"efidisk.efitype":         {Kind: p.Update},
				"efidisk.fileId":          {Kind: p.Update},
				"efidisk.preEnrolledKeys": {Kind: p.Update},
				"efidisk.storage":         {Kind: p.Update},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.disk.DiffFrom("efidisk", tt.state))
		})
	}
}
