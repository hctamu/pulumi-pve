/*
Copyright 2025, Pulumi Corporation.

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

// This file contains comprehensive tests for VM creation disk ordering functionality.
//
// Test Coverage:
// - BuildOptions disk ordering preservation across various interface types
// - Consistency of disk ordering across multiple BuildOptions calls
// - Performance benchmarks for disk ordering with various disk counts
// - End-to-end integration testing simulating real VM creation scenarios
// - Edge cases: empty disks, single disk, mixed interface types, gaps in numbering
// - Production scenarios: boot disks, data disks, CD-ROM, backup storage
//
// These tests ensure that:
// 1. Disk ordering specified by users is always preserved during VM creation
// 2. The BuildOptions method used in VM.Create maintains deterministic ordering
// 3. Performance remains excellent even with large numbers of disks
// 4. Real-world VM configurations work correctly with proper disk ordering

package vm_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildOptionsDiskOrdering verifies that BuildOptions preserves disk ordering
func TestBuildOptionsDiskOrdering(t *testing.T) {
	t.Parallel()

	// Test case with multiple disks in specific order
	testCases := []struct {
		name          string
		disks         []*vm.Disk
		expectedOrder []string
		description   string
	}{
		{
			name: "SATA and IDE disks mixed order",
			disks: []*vm.Disk{
				createTestDisk("sata0", "local-lvm", 20),
				createTestDisk("ide0", "local-lvm", 10),
				createTestDisk("sata1", "local-lvm", 30),
				createTestDisk("ide2", "local-lvm", 15),
			},
			expectedOrder: []string{"sata0", "ide0", "sata1", "ide2"},
			description:   "Mixed SATA and IDE interfaces should preserve input order",
		},
		{
			name: "SCSI disks with gaps",
			disks: []*vm.Disk{
				createTestDisk("scsi0", "local-lvm", 20),
				createTestDisk("scsi2", "local-lvm", 30), // Gap at scsi1
				createTestDisk("scsi5", "local-lvm", 40),
				createTestDisk("scsi1", "local-lvm", 25), // Fill the gap
			},
			expectedOrder: []string{"scsi0", "scsi2", "scsi5", "scsi1"},
			description:   "SCSI disks with non-sequential numbers should preserve input order",
		},
		{
			name: "VirtIO disks with different sizes",
			disks: []*vm.Disk{
				createTestDisk("virtio0", "local-lvm", 100),
				createTestDisk("virtio1", "local-lvm", 50),
				createTestDisk("virtio2", "local-lvm", 200),
			},
			expectedOrder: []string{"virtio0", "virtio1", "virtio2"},
			description:   "VirtIO disks should maintain input order regardless of size",
		},
		{
			name: "Single disk",
			disks: []*vm.Disk{
				createTestDisk("sata0", "local-lvm", 20),
			},
			expectedOrder: []string{"sata0"},
			description:   "Single disk should be handled correctly",
		},
		{
			name: "All interface types mixed",
			disks: []*vm.Disk{
				createTestDisk("virtio0", "local-lvm", 40),
				createTestDisk("scsi1", "local-lvm", 30),
				createTestDisk("ide2", "local-lvm", 20),
				createTestDisk("sata3", "local-lvm", 50),
				createTestDisk("virtio1", "local-lvm", 60),
			},
			expectedOrder: []string{"virtio0", "scsi1", "ide2", "sata3", "virtio1"},
			description:   "All interface types mixed should preserve exact input order",
		},
		{
			name: "CD-ROM and data disks mixed",
			disks: []*vm.Disk{
				createTestDisk("scsi0", "local-lvm", 40),
				createTestCDROMDisk("ide2", "none"),
				createTestDisk("scsi1", "local-lvm", 30),
			},
			expectedOrder: []string{"scsi0", "ide2", "scsi1"},
			description:   "CD-ROM disks mixed with data disks should preserve order",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create VM inputs with the test disks
			inputs := vm.Inputs{
				Disks: tc.disks,
			}
			vmID := 100

			// Build options
			options := inputs.BuildOptions(vmID)

			// Extract disk options in the order they appear
			var actualOrder []string
			for _, opt := range options {
				// Check if this is a disk option (matches disk interface patterns)
				if isDiskInterface(opt.Name) {
					actualOrder = append(actualOrder, opt.Name)
				}
			}

			// Verify the order matches expected
			require.Equal(t, len(tc.expectedOrder), len(actualOrder),
				"Number of disk options should match input disks")
			assert.Equal(t, tc.expectedOrder, actualOrder,
				"Disk order in BuildOptions should match input order: %s", tc.description)

			// Verify each disk option has correct content
			diskOptionMap := make(map[string]string)
			for _, opt := range options {
				if isDiskInterface(opt.Name) {
					diskOptionMap[opt.Name] = opt.Value.(string)
				}
			}

			for i, expectedInterface := range tc.expectedOrder {
				actualValue, found := diskOptionMap[expectedInterface]
				require.True(t, found, "Disk interface %s should be found in options", expectedInterface)

				// Verify the disk configuration matches the input disk
				inputDisk := tc.disks[i]
				expectedKey, expectedValue := inputDisk.ToProxmoxDiskKeyConfig()
				assert.Equal(t, expectedInterface, expectedKey, "Disk interface should match")
				assert.Equal(t, expectedValue, actualValue, "Disk configuration should match")
			}
		})
	}
}

// TestBuildOptionsConsistentOrdering verifies that multiple calls to BuildOptions
// produce the same disk ordering
func TestBuildOptionsConsistentOrdering(t *testing.T) {
	t.Parallel()

	// Create a complex disk configuration
	disks := []*vm.Disk{
		createTestDisk("virtio2", "local-lvm", 40),
		createTestDisk("scsi0", "local-lvm", 30),
		createTestDisk("ide1", "local-lvm", 20),
		createTestDisk("sata5", "local-lvm", 50),
		createTestCDROMDisk("ide2", "none"),
		createTestDisk("virtio0", "local-lvm", 60),
	}

	inputs := vm.Inputs{
		Disks: disks,
	}
	vmID := 200

	// Build options multiple times
	const numIterations = 10
	var allOrders [][]string

	for i := 0; i < numIterations; i++ {
		options := inputs.BuildOptions(vmID)

		var order []string
		for _, opt := range options {
			if isDiskInterface(opt.Name) {
				order = append(order, opt.Name)
			}
		}
		allOrders = append(allOrders, order)
	}

	// Verify all orders are identical
	expectedOrder := allOrders[0]
	for i := 1; i < numIterations; i++ {
		assert.Equal(t, expectedOrder, allOrders[i],
			"BuildOptions should produce consistent ordering across multiple calls (iteration %d)", i)
	}

	// Verify the order matches the input disk order
	expectedInterfaces := make([]string, len(disks))
	for i, disk := range disks {
		expectedInterfaces[i], _ = disk.ToProxmoxDiskKeyConfig()
	}
	assert.Equal(t, expectedInterfaces, expectedOrder,
		"Consistent order should match input disk order")
}

// TestBuildOptionsEmptyDisks verifies behavior with no disks
func TestBuildOptionsEmptyDisks(t *testing.T) {
	t.Parallel()

	inputs := vm.Inputs{
		Disks: nil, // No disks
	}
	vmID := 300

	options := inputs.BuildOptions(vmID)

	// Should not contain any disk options
	for _, opt := range options {
		assert.False(t, isDiskInterface(opt.Name),
			"Should not contain disk options when no disks are specified, but found: %s", opt.Name)
	}
}

// TestBuildOptionsDiskConfiguration verifies that disk configurations are correctly built
func TestBuildOptionsDiskConfiguration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		disk          *vm.Disk
		expectedKey   string
		expectedValue string
	}{
		{
			name:          "Basic SCSI disk",
			disk:          createTestDisk("scsi0", "local-lvm", 32),
			expectedKey:   "scsi0",
			expectedValue: "file=local-lvm:32,size=32",
		},
		{
			name: "VirtIO disk with FileID",
			disk: &vm.Disk{
				Interface: "virtio1",
				Storage:   "local-lvm",
				Size:      64,
				FileID:    "vm-100-disk-1",
			},
			expectedKey:   "virtio1",
			expectedValue: "file=local-lvm:vm-100-disk-1,size=64",
		},
		{
			name: "CD-ROM disk",
			disk: &vm.Disk{
				Interface: "ide2",
				Storage:   "none",
				Size:      0,
			},
			expectedKey:   "ide2",
			expectedValue: "file=none:0,size=0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualKey, actualValue := tc.disk.ToProxmoxDiskKeyConfig()
			assert.Equal(t, tc.expectedKey, actualKey, "Disk key should match expected")
			assert.Equal(t, tc.expectedValue, actualValue, "Disk configuration should match expected")
		})
	}
}

// Helper functions

// createTestDisk creates a test disk with the specified interface, storage, and size
func createTestDisk(iface, storage string, size int) *vm.Disk {
	return &vm.Disk{
		Interface: iface,
		Storage:   storage,
		Size:      size,
	}
}

// createTestCDROMDisk creates a test CD-ROM disk
func createTestCDROMDisk(iface, storage string) *vm.Disk {
	return &vm.Disk{
		Interface: iface,
		Storage:   storage,
		Size:      0,
	}
}

// isDiskInterface checks if the given string is a disk interface name
func isDiskInterface(name string) bool {
	// Check for common Proxmox disk interfaces
	diskPrefixes := []string{"scsi", "virtio", "ide", "sata"}
	for _, prefix := range diskPrefixes {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			// Check if the remaining characters are digits
			for _, r := range name[len(prefix):] {
				if r < '0' || r > '9' {
					return false
				}
			}
			return true
		}
	}
	return false
}

// TestVMCreateDiskOrderingIntegration verifies that VM.Create preserves disk ordering
// when calling BuildOptions during the creation process
func TestVMCreateDiskOrderingIntegration(t *testing.T) {
	t.Parallel()

	// Test that simulates how Create function processes disk ordering
	testCases := []struct {
		name          string
		disks         []*vm.Disk
		expectedOrder []string
		description   string
	}{
		{
			name: "Mixed interface types during create",
			disks: []*vm.Disk{
				createTestDisk("virtio0", "local-lvm", 40),
				createTestDisk("scsi2", "local-lvm", 30),
				createTestDisk("ide0", "local-lvm", 20),
				createTestCDROMDisk("ide1", "none"),
				createTestDisk("sata0", "local-lvm", 50),
			},
			expectedOrder: []string{"virtio0", "scsi2", "ide0", "ide1", "sata0"},
			description:   "Create should preserve complex disk ordering",
		},
		{
			name: "Real-world scenario",
			disks: []*vm.Disk{
				createTestDisk("scsi0", "local-lvm", 32),  // Boot disk
				createTestDisk("scsi1", "local-lvm", 100), // Data disk 1
				createTestDisk("scsi2", "local-lvm", 200), // Data disk 2
				createTestCDROMDisk("ide2", "none"),       // CD-ROM
			},
			expectedOrder: []string{"scsi0", "scsi1", "scsi2", "ide2"},
			description:   "Typical VM setup with boot, data disks and CD-ROM",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create inputs similar to what Create function receives
			name := "test-vm"
			vmid := 100
			inputs := vm.Inputs{
				Name:  &name,
				VMID:  &vmid,
				Disks: tc.disks,
			}

			// Call BuildOptions just like the Create function does (line 90 in vm.go)
			options := inputs.BuildOptions(*inputs.VMID)

			// Extract disk options in the order they appear
			var actualOrder []string
			var diskOptions []string
			for _, opt := range options {
				if isDiskInterface(opt.Name) {
					actualOrder = append(actualOrder, opt.Name)
					diskOptions = append(diskOptions, fmt.Sprintf("%s=%s", opt.Name, opt.Value))
				}
			}

			// Verify the order matches expected
			require.Equal(t, len(tc.expectedOrder), len(actualOrder),
				"Number of disk options should match input disks")
			assert.Equal(t, tc.expectedOrder, actualOrder,
				"Create disk ordering should match input order: %s", tc.description)

			// Log the actual disk options for debugging
			t.Logf("Generated disk options (in order): %v", diskOptions)

			// Verify that the ordering is deterministic by calling multiple times
			for i := 0; i < 3; i++ {
				options2 := inputs.BuildOptions(*inputs.VMID)
				var order2 []string
				for _, opt := range options2 {
					if isDiskInterface(opt.Name) {
						order2 = append(order2, opt.Name)
					}
				}
				assert.Equal(t, actualOrder, order2,
					"BuildOptions should be deterministic (iteration %d)", i)
			}
		})
	}
}

// TestVMCreateDiskOptionsConsistency verifies that disk options are consistently
// ordered across multiple BuildOptions calls during VM creation
func TestVMCreateDiskOptionsConsistency(t *testing.T) {
	t.Parallel()

	// Complex disk configuration similar to real-world scenarios
	disks := []*vm.Disk{
		createTestDisk("virtio0", "local-lvm", 32),     // Primary disk
		createTestDisk("virtio1", "local-ssd", 64),     // Secondary SSD
		createTestDisk("scsi0", "local-hdd", 500),      // Large storage
		createTestCDROMDisk("ide2", "none"),            // CD-ROM
		createTestDisk("sata0", "backup-storage", 100), // Backup disk
	}

	name := "test-vm-consistency"
	vmid := 200
	inputs := vm.Inputs{
		Name:  &name,
		VMID:  &vmid,
		Disks: disks,
	}

	// Multiple calls should produce identical ordering
	const iterations = 50
	var allOrders [][]string

	for i := 0; i < iterations; i++ {
		options := inputs.BuildOptions(*inputs.VMID)
		var order []string
		for _, opt := range options {
			if isDiskInterface(opt.Name) {
				order = append(order, opt.Name)
			}
		}
		allOrders = append(allOrders, order)
	}

	// All orders should be identical
	expectedOrder := allOrders[0]
	for i := 1; i < iterations; i++ {
		assert.Equal(t, expectedOrder, allOrders[i],
			"VM create disk ordering should be consistent (iteration %d)", i)
	}

	// Expected order should match input order
	expectedInterfaces := make([]string, len(disks))
	for i, disk := range disks {
		expectedInterfaces[i] = disk.Interface
	}
	assert.Equal(t, expectedInterfaces, expectedOrder,
		"VM create should preserve input disk order")
}

// Benchmark tests for VM creation disk ordering

// BenchmarkBuildOptionsDiskOrdering benchmarks the BuildOptions method with various disk counts
func BenchmarkBuildOptionsDiskOrdering(b *testing.B) {
	benchmarks := []struct {
		name      string
		diskCount int
	}{
		{"1_disk", 1},
		{"5_disks", 5},
		{"10_disks", 10},
		{"20_disks", 20},
		{"50_disks", 50},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Create test disks
			disks := make([]*vm.Disk, bm.diskCount)
			for i := 0; i < bm.diskCount; i++ {
				// Alternate between different interface types
				var iface string
				switch i % 4 {
				case 0:
					iface = fmt.Sprintf("scsi%d", i)
				case 1:
					iface = fmt.Sprintf("virtio%d", i)
				case 2:
					iface = fmt.Sprintf("ide%d", i)
				case 3:
					iface = fmt.Sprintf("sata%d", i)
				}
				disks[i] = createTestDisk(iface, "local-lvm", 32)
			}

			name := "benchmark-vm"
			vmid := 1000 + bm.diskCount
			inputs := vm.Inputs{
				Name:  &name,
				VMID:  &vmid,
				Disks: disks,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = inputs.BuildOptions(*inputs.VMID)
			}
		})
	}
}

// BenchmarkBuildOptionsConsistency benchmarks multiple BuildOptions calls for consistency
func BenchmarkBuildOptionsConsistency(b *testing.B) {
	// Create a realistic disk configuration
	disks := []*vm.Disk{
		createTestDisk("virtio0", "local-lvm", 32),
		createTestDisk("virtio1", "local-ssd", 64),
		createTestDisk("scsi0", "local-hdd", 500),
		createTestCDROMDisk("ide2", "none"),
		createTestDisk("sata0", "backup-storage", 100),
		createTestDisk("scsi1", "local-lvm", 200),
		createTestDisk("virtio2", "local-ssd", 128),
		createTestDisk("ide0", "local-lvm", 16),
	}

	name := "benchmark-consistency-vm"
	vmid := 2000
	inputs := vm.Inputs{
		Name:  &name,
		VMID:  &vmid,
		Disks: disks,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		options := inputs.BuildOptions(*inputs.VMID)

		// Extract disk options to simulate real usage
		var diskOrder []string
		for _, opt := range options {
			if isDiskInterface(opt.Name) {
				diskOrder = append(diskOrder, opt.Name)
			}
		}

		// Use the result to prevent optimization
		_ = len(diskOrder)
	}
}

// TestVMCreateDiskOrderingEndToEnd provides a comprehensive test of disk ordering
// throughout the entire VM creation process
func TestVMCreateDiskOrderingEndToEnd(t *testing.T) {
	t.Parallel()

	// This test simulates a real-world VM configuration that would be used
	// in production environments to ensure disk ordering is preserved correctly
	testCase := struct {
		name        string
		description string
		disks       []*vm.Disk
		expected    []string
	}{
		name:        "Production VM with complex disk setup",
		description: "Boot disk, data disks, CD-ROM, and backup storage in specific order",
		disks: []*vm.Disk{
			// Boot disk - always first
			createTestDisk("scsi0", "local-ssd", 32),
			// Application data - high performance storage
			createTestDisk("scsi1", "local-ssd", 100),
			// Database storage - separate disk for reliability
			createTestDisk("scsi2", "local-ssd", 200),
			// CD-ROM for installations
			createTestCDROMDisk("ide2", "none"),
			// Backup storage - lower tier storage
			createTestDisk("sata0", "local-hdd", 500),
			// Log storage - separate from main data
			createTestDisk("virtio0", "local-lvm", 50),
			// Temp storage - high performance for temp files
			createTestDisk("virtio1", "local-ssd", 64),
		},
		expected: []string{"scsi0", "scsi1", "scsi2", "ide2", "sata0", "virtio0", "virtio1"},
	}

	t.Run(testCase.name, func(t *testing.T) {
		t.Parallel()

		// Step 1: Create VM inputs as they would be in real usage
		name := "production-vm"
		vmid := 500
		inputs := vm.Inputs{
			Name:  &name,
			VMID:  &vmid,
			Disks: testCase.disks,
		}

		t.Logf("Test case: %s", testCase.description)
		t.Logf("Input disk order: %v", testCase.expected)

		// Step 2: Call BuildOptions multiple times to ensure consistency
		// This simulates what would happen during VM creation, updates, etc.
		const numCalls = 10
		var allOrders [][]string

		for i := 0; i < numCalls; i++ {
			options := inputs.BuildOptions(*inputs.VMID)

			var diskOrder []string
			var diskDetails []string
			for _, opt := range options {
				if isDiskInterface(opt.Name) {
					diskOrder = append(diskOrder, opt.Name)
					diskDetails = append(diskDetails, fmt.Sprintf("%s=%s", opt.Name, opt.Value))
				}
			}
			allOrders = append(allOrders, diskOrder)

			if i == 0 {
				t.Logf("Generated disk options: %v", diskDetails)
			}
		}

		// Step 3: Verify all calls produce identical ordering
		firstOrder := allOrders[0]
		for i := 1; i < numCalls; i++ {
			assert.Equal(t, firstOrder, allOrders[i],
				"BuildOptions call %d should produce consistent ordering", i)
		}

		// Step 4: Verify the order matches the expected input order
		assert.Equal(t, testCase.expected, firstOrder,
			"Disk ordering should exactly match input order")

		// Step 5: Verify each disk configuration is correct
		options := inputs.BuildOptions(*inputs.VMID)
		diskOptionMap := make(map[string]string)
		for _, opt := range options {
			if isDiskInterface(opt.Name) {
				diskOptionMap[opt.Name] = opt.Value.(string)
			}
		}

		for i, expectedInterface := range testCase.expected {
			actualValue, found := diskOptionMap[expectedInterface]
			require.True(t, found, "Disk interface %s should be found", expectedInterface)

			// Verify the disk configuration matches the input disk
			inputDisk := testCase.disks[i]
			expectedKey, expectedValue := inputDisk.ToProxmoxDiskKeyConfig()
			assert.Equal(t, expectedInterface, expectedKey,
				"Disk %d interface should match", i)
			assert.Equal(t, expectedValue, actualValue,
				"Disk %d configuration should match", i)
		}

		// Step 6: Performance verification - ensure ordering doesn't impact performance
		start := time.Now()
		for i := 0; i < 1000; i++ {
			_ = inputs.BuildOptions(*inputs.VMID)
		}
		duration := time.Since(start)

		t.Logf("1000 BuildOptions calls took %v (avg: %v per call)",
			duration, duration/1000)

		// Should complete 1000 calls in under 100ms (very generous threshold)
		assert.Less(t, duration.Milliseconds(), int64(100),
			"Disk ordering should not significantly impact performance")
	})
}
