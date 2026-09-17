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

package proxmox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoerceDiskMapState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		state    any
		expected DiskMap
	}{
		{
			name:     "nil returns empty map",
			state:    nil,
			expected: DiskMap{},
		},
		{
			name: "disk map passthrough",
			state: DiskMap{
				"disk-0": {Interface: "scsi0", DiskBase: DiskBase{Storage: "local"}, Size: 20},
			},
			expected: DiskMap{
				"disk-0": {Interface: "scsi0", DiskBase: DiskBase{Storage: "local"}, Size: 20},
			},
		},
		{
			name: "legacy []disk coerces to logical map",
			state: []*Disk{
				{Interface: "scsi0", DiskBase: DiskBase{Storage: "local"}, Size: 20},
			},
			expected: DiskMap{
				"disk-0": {Interface: "scsi0", DiskBase: DiskBase{Storage: "local"}, Size: 20},
			},
		},
		{
			name:     "unsupported type returns empty map",
			state:    42,
			expected: DiskMap{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, coerceDiskMapState(tt.state))
		})
	}
}

func TestDiskIfaceType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		iface     string
		expected  string
		wantError bool
	}{
		{name: "scsi iface", iface: "scsi0", expected: "scsi"},
		{name: "virtio iface", iface: "virtio5", expected: "virtio"},
		{name: "sata iface", iface: "sata2", expected: "sata"},
		{name: "ide iface", iface: "ide1", expected: "ide"},
		{name: "unsupported iface", iface: "nvme0", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual, err := diskIfaceType(tt.iface)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
