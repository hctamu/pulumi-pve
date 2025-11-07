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
	"math"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertVMConfigToInputs_DiskOrdering tests that disks are returned in a consistent
// sorted order regardless of the order they appear in the VM configuration map.
func TestConvertVMConfigToInputs_DiskOrdering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		vmConfig      api.VirtualMachineConfig
		expectedOrder []string // Expected disk interface order
	}{
		{
			name: "Mixed interface types in random order",
			vmConfig: api.VirtualMachineConfig{
				Name: "test-vm",
				// Disk interfaces in intentionally random order to test sorting
				SCSI3:   "ceph-ha:vm-test-disk-3,size=30G",
				IDE1:    "local:vm-test-disk-1,size=10G",
				SCSI0:   "ceph-ha:vm-test-disk-0,size=20G",
				VirtIO2: "local-lvm:vm-test-disk-2,size=15G",
				SATA1:   "ceph-ha:vm-test-disk-4,size=25G",
			},
			// Should be sorted alphabetically: ide1, sata1, scsi0, scsi3, virtio2
			expectedOrder: []string{"ide1", "sata1", "scsi0", "scsi3", "virtio2"},
		},
		{
			name: "Only SCSI disks in reverse numerical order",
			vmConfig: api.VirtualMachineConfig{
				Name:  "test-vm",
				SCSI5: "storage:vm-test-disk-5,size=50G",
				SCSI2: "storage:vm-test-disk-2,size=20G",
				SCSI0: "storage:vm-test-disk-0,size=10G",
				SCSI3: "storage:vm-test-disk-3,size=30G",
				SCSI1: "storage:vm-test-disk-1,size=15G",
			},
			// Should be sorted: scsi0, scsi1, scsi2, scsi3, scsi5
			expectedOrder: []string{"scsi0", "scsi1", "scsi2", "scsi3", "scsi5"},
		},
		{
			name: "Mixed types with gaps in numbering",
			vmConfig: api.VirtualMachineConfig{
				Name:    "test-vm",
				SCSI10:  "storage:vm-test-disk-10,size=100G",
				IDE0:    "storage:vm-test-disk-ide0,size=5G",
				SATA5:   "storage:vm-test-disk-sata5,size=75G",
				VirtIO1: "storage:vm-test-disk-virtio1,size=40G",
				SCSI1:   "storage:vm-test-disk-1,size=10G",
			},
			// Should be sorted: ide0, sata5, scsi1, scsi10, virtio1
			expectedOrder: []string{"ide0", "sata5", "scsi1", "scsi10", "virtio1"},
		},
		{
			name: "Single disk",
			vmConfig: api.VirtualMachineConfig{
				Name:  "test-vm",
				SCSI0: "storage:vm-test-disk-0,size=20G",
			},
			expectedOrder: []string{"scsi0"},
		},
		{
			name: "No disks",
			vmConfig: api.VirtualMachineConfig{
				Name: "test-vm",
			},
			expectedOrder: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a mock VirtualMachine with the test config
			mockVM := &api.VirtualMachine{
				VirtualMachineConfig: &tt.vmConfig,
				VMID:                 api.StringOrUint64(123),
			}

			// Convert VM config to inputs
			inputs, err := vm.ConvertVMConfigToInputs(mockVM)
			require.NoError(t, err)

			// Verify the number of disks matches expectations
			assert.Len(t, inputs.Disks, len(tt.expectedOrder), "Number of disks should match expected")

			// Verify the disk interfaces are in the expected sorted order
			actualOrder := make([]string, len(inputs.Disks))
			for i, disk := range inputs.Disks {
				actualOrder[i] = disk.Interface
			}

			assert.Equal(t, tt.expectedOrder, actualOrder, "Disk interfaces should be in sorted order")

			// Additional verification: ensure the disks are properly parsed
			for i, disk := range inputs.Disks {
				assert.NotEmpty(t, disk.Interface, "Disk %d should have an interface", i)
				assert.NotEmpty(t, disk.Storage, "Disk %d should have storage", i)
				assert.Greater(t, disk.Size, 0, "Disk %d should have a positive size", i)
			}
		})
	}
}

// TestConvertVMConfigToInputs_ConsistentOrdering tests that the same VM configuration
// always produces the same disk ordering across multiple calls.
func TestConvertVMConfigToInputs_ConsistentOrdering(t *testing.T) {
	t.Parallel()

	// VM config with disks in a specific order
	vmConfig := api.VirtualMachineConfig{
		Name:    "consistency-test",
		SCSI7:   "storage:disk-7,size=70G",
		IDE2:    "storage:disk-ide2,size=20G",
		SCSI1:   "storage:disk-1,size=10G",
		VirtIO0: "storage:disk-virtio0,size=50G",
		SATA3:   "storage:disk-sata3,size=30G",
		SCSI12:  "storage:disk-12,size=120G",
	}

	mockVM := &api.VirtualMachine{
		VirtualMachineConfig: &vmConfig,
		VMID:                 api.StringOrUint64(456),
	}

	// Convert multiple times and ensure consistent ordering
	var previousOrder []string
	for i := 0; i < 5; i++ {
		inputs, err := vm.ConvertVMConfigToInputs(mockVM)
		require.NoError(t, err)

		currentOrder := make([]string, len(inputs.Disks))
		for j, disk := range inputs.Disks {
			currentOrder[j] = disk.Interface
		}

		if i == 0 {
			previousOrder = currentOrder
			// Verify it's actually sorted (alphabetical, not numerical)
			assert.Equal(t, []string{"ide2", "sata3", "scsi1", "scsi12", "scsi7", "virtio0"}, currentOrder)
		} else {
			assert.Equal(t, previousOrder, currentOrder, "Disk order should be consistent across multiple calls")
		}
	}
}

// TestDiskParsing tests that individual disk configurations are parsed correctly
// while maintaining proper ordering.
func TestDiskParsing(t *testing.T) {
	t.Parallel()

	vmConfig := api.VirtualMachineConfig{
		Name:  "parse-test",
		SCSI0: "ceph-ha:vm-100-disk-0,size=32G",
		SCSI1: "local-lvm:vm-100-disk-1,size=20G",
		IDE0:  "local:vm-100-disk-2,size=5G",
	}

	mockVM := &api.VirtualMachine{
		VirtualMachineConfig: &vmConfig,
		VMID:                 api.StringOrUint64(100),
	}

	inputs, err := vm.ConvertVMConfigToInputs(mockVM)
	require.NoError(t, err)
	require.Len(t, inputs.Disks, 3)

	// Check that disks are in sorted order: ide0, scsi0, scsi1
	expectedDisks := []*vm.Disk{
		{Interface: "ide0", Storage: "local", FileID: "vm-100-disk-2", Size: 5},
		{Interface: "scsi0", Storage: "ceph-ha", FileID: "vm-100-disk-0", Size: 32},
		{Interface: "scsi1", Storage: "local-lvm", FileID: "vm-100-disk-1", Size: 20},
	}

	for i, expectedDisk := range expectedDisks {
		actualDisk := inputs.Disks[i]
		assert.Equal(t, expectedDisk.Interface, actualDisk.Interface, "Disk %d interface mismatch", i)
		assert.Equal(t, expectedDisk.Storage, actualDisk.Storage, "Disk %d storage mismatch", i)
		if expectedDisk.FileID != "" {
			assert.Equal(t, expectedDisk.FileID, actualDisk.FileID, "Disk %d fileID mismatch", i)
		}
		if expectedDisk.Size > 0 {
			assert.Equal(t, expectedDisk.Size, actualDisk.Size, "Disk %d size mismatch", i)
		}
	}
}

// TestConvertVMConfigToInputs_VMIDHandling tests VMID conversion and overflow handling.
func TestConvertVMConfigToInputs_VMIDHandling(t *testing.T) {
	t.Parallel()

	t.Run("normal VMID", func(t *testing.T) {
		vmConfig := api.VirtualMachineConfig{Name: "test"}
		mockVM := &api.VirtualMachine{
			VirtualMachineConfig: &vmConfig,
			VMID:                 api.StringOrUint64(12345),
		}

		inputs, err := vm.ConvertVMConfigToInputs(mockVM)
		require.NoError(t, err)
		require.NotNil(t, inputs.VMID)
		assert.Equal(t, 12345, *inputs.VMID)
	})

	t.Run("VMID overflow", func(t *testing.T) {
		vmConfig := api.VirtualMachineConfig{Name: "test"}
		mockVM := &api.VirtualMachine{
			VirtualMachineConfig: &vmConfig,
			VMID:                 api.StringOrUint64(uint64(math.MaxInt) + 1), // This should overflow
		}

		_, err := vm.ConvertVMConfigToInputs(mockVM)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "VMID")
		assert.Contains(t, err.Error(), "overflows")
	})
}

// BenchmarkDiskOrdering benchmarks the disk ordering performance with various numbers of disks.
func BenchmarkDiskOrdering(b *testing.B) {
	// Create a VM config with many disks for performance testing
	vmConfig := api.VirtualMachineConfig{
		Name:    "benchmark-vm",
		SCSI0:   "storage:disk-0,size=10G",
		SCSI1:   "storage:disk-1,size=10G",
		SCSI2:   "storage:disk-2,size=10G",
		SCSI3:   "storage:disk-3,size=10G",
		SCSI4:   "storage:disk-4,size=10G",
		SCSI5:   "storage:disk-5,size=10G",
		SCSI6:   "storage:disk-6,size=10G",
		SCSI7:   "storage:disk-7,size=10G",
		SCSI8:   "storage:disk-8,size=10G",
		SCSI9:   "storage:disk-9,size=10G",
		SCSI10:  "storage:disk-10,size=10G",
		SCSI11:  "storage:disk-11,size=10G",
		SCSI12:  "storage:disk-12,size=10G",
		SCSI13:  "storage:disk-13,size=10G",
		SCSI14:  "storage:disk-14,size=10G",
		SCSI15:  "storage:disk-15,size=10G",
		IDE0:    "storage:vm-999-cdrom0,size=1G",
		IDE1:    "storage:vm-999-cdrom1,size=1G",
		IDE2:    "storage:vm-999-cdrom2,size=1G",
		IDE3:    "storage:vm-999-cdrom3,size=1G",
		SATA0:   "storage:sata-0,size=50G",
		SATA1:   "storage:sata-1,size=50G",
		SATA2:   "storage:sata-2,size=50G",
		SATA3:   "storage:sata-3,size=50G",
		SATA4:   "storage:sata-4,size=50G",
		SATA5:   "storage:sata-5,size=50G",
		VirtIO0: "storage:virtio-0,size=100G",
		VirtIO1: "storage:virtio-1,size=100G",
		VirtIO2: "storage:virtio-2,size=100G",
		VirtIO3: "storage:virtio-3,size=100G",
		VirtIO4: "storage:virtio-4,size=100G",
		VirtIO5: "storage:virtio-5,size=100G",
		VirtIO6: "storage:virtio-6,size=100G",
		VirtIO7: "storage:virtio-7,size=100G",
		VirtIO8: "storage:virtio-8,size=100G",
		VirtIO9: "storage:virtio-9,size=100G",
	}

	mockVM := &api.VirtualMachine{
		VirtualMachineConfig: &vmConfig,
		VMID:                 api.StringOrUint64(999),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := vm.ConvertVMConfigToInputs(mockVM)
		if err != nil {
			b.Fatal(err)
		}
	}
}
