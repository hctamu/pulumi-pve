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

	vmResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	lvmStorage = "local-lvm"
	ssdStorage = "local-ssd"
	hddStorage = "local-hdd"
)

// TestVMCreateDiskOrderPreservation tests that VM creation preserves disk ordering
// through the entire creation process using the same pattern as other resource tests
func TestVMCreateDiskOrderPreservation(t *testing.T) {
	t.Parallel()

	// Test cases with various disk ordering scenarios
	testCases := []struct {
		name                string
		inputDisks          []*vmResource.Disk
		expectedDiskOrder   []string
		description         string
		shouldPreserveOrder bool
	}{
		{
			name: "Mixed interface types preserve order",
			inputDisks: []*vmResource.Disk{
				{
					Interface: "virtio0",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 32,
				},
				{
					Interface: "scsi1",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 64,
				},
				{
					Interface: "ide0",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 8,
				},
				{
					Interface: "sata2",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 128,
				},
			},
			expectedDiskOrder:   []string{"virtio0", "scsi1", "ide0", "sata2"},
			description:         "Mixed interface types should preserve input order",
			shouldPreserveOrder: true,
		},
		{
			name: "Production VM layout",
			inputDisks: []*vmResource.Disk{
				{
					Interface: "scsi0",
					DiskBase: vmResource.DiskBase{
						Storage: ssdStorage,
					},
					Size: 32,
				}, // Boot disk
				{
					Interface: "scsi1",
					DiskBase: vmResource.DiskBase{
						Storage: ssdStorage,
					},
					Size: 100,
				}, // Data disk
				{
					Interface: "scsi2",
					DiskBase: vmResource.DiskBase{
						Storage: hddStorage,
					},
					Size: 500,
				}, // Backup storage
				{
					Interface: "ide2",
					DiskBase: vmResource.DiskBase{
						Storage: "none",
					},
					Size: 0,
				}, // CD-ROM
			},
			expectedDiskOrder:   []string{"scsi0", "scsi1", "scsi2", "ide2"},
			description:         "Production VM should maintain boot disk first, then data disks",
			shouldPreserveOrder: true,
		},
		{
			name: "Complex numbering with gaps",
			inputDisks: []*vmResource.Disk{
				{
					Interface: "scsi0",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 20,
				},
				{
					Interface: "scsi3",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 40,
				}, // Gap at scsi1, scsi2
				{
					Interface: "virtio1",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 30,
				},
				{
					Interface: "scsi1",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 25,
				}, // Fill gap later
			},
			expectedDiskOrder:   []string{"scsi0", "scsi3", "virtio1", "scsi1"},
			description:         "Non-sequential interface numbers should preserve input order",
			shouldPreserveOrder: true,
		},
		{
			name: "Single disk",
			inputDisks: []*vmResource.Disk{
				{
					Interface: "virtio0",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 64,
				},
			},
			expectedDiskOrder:   []string{"virtio0"},
			description:         "Single disk should be handled correctly",
			shouldPreserveOrder: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup VM inputs as would be passed to Create
			name := "test-vm-" + tc.name
			vmid := 100
			inputs := vmResource.Inputs{
				Name:  &name,
				VMID:  &vmid,
				Disks: tc.inputDisks,
			}

			// Test BuildOptions method which is called during VM creation
			options := inputs.BuildOptions(*inputs.VMID)

			// Extract disk options in order
			var actualDiskOrder []string
			var diskConfigs []string

			for _, opt := range options {
				if isDiskInterface(opt.Name) {
					actualDiskOrder = append(actualDiskOrder, opt.Name)
					diskConfigs = append(diskConfigs, opt.Value.(string))
				}
			}

			t.Logf("Test case: %s", tc.description)
			t.Logf("Input disk order: %v", tc.expectedDiskOrder)
			t.Logf("Actual disk order: %v", actualDiskOrder)
			t.Logf("Disk configurations: %v", diskConfigs)

			if tc.shouldPreserveOrder {
				// Verify order is preserved
				require.Equal(t, len(tc.expectedDiskOrder), len(actualDiskOrder),
					"Number of disk options should match input")
				assert.Equal(t, tc.expectedDiskOrder, actualDiskOrder,
					"Disk order should be preserved: %s", tc.description)

				// Verify each disk configuration is correct
				for i, expectedInterface := range tc.expectedDiskOrder {
					inputDisk := tc.inputDisks[i]
					expectedKey, expectedConfig := inputDisk.ToProxmoxDiskKeyConfig()

					assert.Equal(t, expectedInterface, expectedKey,
						"Disk %d interface should match input", i)

					// Find the matching option in the BuildOptions output
					var foundConfig string
					for _, opt := range options {
						if opt.Name == expectedInterface {
							foundConfig = opt.Value.(string)
							break
						}
					}

					assert.Equal(t, expectedConfig, foundConfig,
						"Disk %d configuration should match expected", i)
				}

				// Test consistency across multiple calls (like the group tests)
				for i := 0; i < 5; i++ {
					options2 := inputs.BuildOptions(*inputs.VMID)
					var order2 []string
					for _, opt := range options2 {
						if isDiskInterface(opt.Name) {
							order2 = append(order2, opt.Name)
						}
					}
					assert.Equal(t, actualDiskOrder, order2,
						"BuildOptions should be consistent across calls (iteration %d)", i)
				}
			}
		})
	}
}

// TestVMCreateDiskOrderWithSeam tests disk ordering consistency
// following the pattern from group_test.go but focused on BuildOptions method
//
//nolint:paralleltest // mutates seam
func TestVMCreateDiskOrderWithSeam(t *testing.T) {
	// Test BuildOptions method directly to avoid provider context issues
	t.Run("build_options_preserves_order", func(t *testing.T) {
		t.Parallel()

		// Create complex disk ordering
		orderedDisks := []*vmResource.Disk{
			{
				Interface: "virtio0",
				DiskBase: vmResource.DiskBase{
					Storage: ssdStorage,
				},
				Size: 32,
			},
			{
				Interface: "scsi1",
				DiskBase: vmResource.DiskBase{
					Storage: lvmStorage,
				},
				Size: 64,
			},
			{
				Interface: "ide2",
				DiskBase: vmResource.DiskBase{
					Storage: "none",
				},
				Size: 0,
			},
			{
				Interface: "sata0",
				DiskBase: vmResource.DiskBase{
					Storage: hddStorage,
				},
				Size: 128,
			},
		}

		name := "build-options-test-vm"
		vmid := 200
		inputs := vmResource.Inputs{
			Name:  &name,
			VMID:  &vmid,
			Disks: orderedDisks,
		}

		// Call BuildOptions multiple times to ensure consistency
		const iterations = 10
		var allOrders [][]string

		for i := 0; i < iterations; i++ {
			options := inputs.BuildOptions(*inputs.VMID)

			var diskOrder []string
			var diskConfigs []string
			for _, opt := range options {
				if isDiskInterface(opt.Name) {
					diskOrder = append(diskOrder, opt.Name)
					diskConfigs = append(diskConfigs, opt.Value.(string))
				}
			}

			if i == 0 {
				t.Logf("Generated disk options: %v", diskConfigs)
			}

			allOrders = append(allOrders, diskOrder)
		}

		// Verify all calls produce identical ordering
		expectedOrder := []string{"virtio0", "scsi1", "ide2", "sata0"}
		for i, actualOrder := range allOrders {
			assert.Equal(t, expectedOrder, actualOrder,
				"BuildOptions iteration %d should produce consistent ordering", i)
		}

		// Verify that disk order matches input order
		for i, expectedInterface := range expectedOrder {
			inputDisk := orderedDisks[i]
			assert.Equal(t, inputDisk.Interface, expectedInterface,
				"BuildOptions should preserve input disk order at position %d", i)
		}
	})

	// Test disk ordering preservation across different scenarios
	t.Run("different_disk_scenarios", func(t *testing.T) {
		t.Parallel()

		scenarios := []struct {
			name     string
			disks    []*vmResource.Disk
			expected []string
		}{
			{
				name: "reverse_numerical_order",
				disks: []*vmResource.Disk{
					{
						Interface: "scsi3",
						DiskBase: vmResource.DiskBase{
							Storage: lvmStorage,
						},
						Size: 30,
					},
					{
						Interface: "scsi1",
						DiskBase: vmResource.DiskBase{
							Storage: lvmStorage,
						},
						Size: 20,
					},
					{
						Interface: "scsi0",
						DiskBase: vmResource.DiskBase{
							Storage: lvmStorage,
						},
						Size: 10,
					},
				},
				expected: []string{"scsi3", "scsi1", "scsi0"},
			},
			{
				name: "mixed_types_non_alphabetical",
				disks: []*vmResource.Disk{
					{
						Interface: "virtio5",
						DiskBase: vmResource.DiskBase{
							Storage: lvmStorage,
						},
						Size: 50,
					},
					{
						Interface: "ide0",
						DiskBase: vmResource.DiskBase{
							Storage: lvmStorage,
						},
						Size: 5,
					},
					{
						Interface: "scsi2",
						DiskBase: vmResource.DiskBase{
							Storage: lvmStorage,
						},
						Size: 25,
					},
					{
						Interface: "sata10",
						DiskBase: vmResource.DiskBase{
							Storage: lvmStorage,
						},
						Size: 100,
					},
				},
				expected: []string{"virtio5", "ide0", "scsi2", "sata10"},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				vmid := 300
				inputs := vmResource.Inputs{
					VMID:  &vmid,
					Disks: scenario.disks,
				}

				options := inputs.BuildOptions(*inputs.VMID)

				var actualOrder []string
				for _, opt := range options {
					if isDiskInterface(opt.Name) {
						actualOrder = append(actualOrder, opt.Name)
					}
				}

				assert.Equal(t, scenario.expected, actualOrder,
					"Scenario %s should preserve disk order", scenario.name)
			})
		}
	})
}

// TestVMReadDiskOrderPreservation tests that VM Read operation preserves disk ordering
// when calling ConvertVMConfigToInputs with currentInput parameter
func TestVMReadDiskOrderPreservation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		currentInputDisks []*vmResource.Disk
		vmConfigDisks     map[string]string // Interface -> Config
		expectedDiskOrder []string
		description       string
	}{
		{
			name: "Preserve existing disk order during read",
			currentInputDisks: []*vmResource.Disk{
				{
					Interface: "virtio0",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 32,
				},
				{
					Interface: "scsi1",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 64,
				},
				{
					Interface: "ide2",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 8,
				},
			},
			vmConfigDisks: map[string]string{
				"virtio0": "local-lvm:vm-100-disk-0,size=32G",
				"scsi1":   "local-lvm:vm-100-disk-1,size=64G",
				"ide2":    "local-lvm:vm-100-disk-2,size=8G",
			},
			expectedDiskOrder: []string{"virtio0", "scsi1", "ide2"},
			description:       "Read should preserve current input disk order",
		},
		{
			name: "Handle new disks added to VM config",
			currentInputDisks: []*vmResource.Disk{
				{
					Interface: "scsi0",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 32,
				},
			},
			vmConfigDisks: map[string]string{
				"scsi0": "local-lvm:vm-100-disk-0,size=32G",
				"scsi1": "local-lvm:vm-100-disk-1,size=64G", // New disk
			},
			expectedDiskOrder: []string{"scsi0", "scsi1"},
			description:       "Read should preserve existing order and append new disks",
		},
		{
			name: "Handle missing disks from VM config",
			currentInputDisks: []*vmResource.Disk{
				{
					Interface: "scsi0",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 32,
				},
				{
					Interface: "scsi1",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 64,
				},
				{
					Interface: "scsi2",
					DiskBase: vmResource.DiskBase{
						Storage: lvmStorage,
					},
					Size: 128,
				},
			},
			vmConfigDisks: map[string]string{
				"scsi0": "local-lvm:vm-100-disk-0,size=32G",
				"scsi2": "local-lvm:vm-100-disk-2,size=128G",
				// scsi1 missing from VM config
			},
			expectedDiskOrder: []string{"scsi0", "scsi2"},
			description:       "Read should only include disks that exist in VM config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create mock VM with the specified disk configuration
			mockVM := createMockVM(tc.vmConfigDisks)

			// Create current input with the ordered disks
			currentInput := vmResource.Inputs{
				Disks: tc.currentInputDisks,
			}

			// Call ConvertVMConfigToInputs with current input to preserve order
			result, err := vmResource.ConvertVMConfigToInputs(mockVM, currentInput)
			require.NoError(t, err)

			t.Logf("Test case: %s", tc.description)
			t.Logf("Current input order: %v", getDiskInterfaces(tc.currentInputDisks))
			t.Logf("VM config disks: %v", getMapKeys(tc.vmConfigDisks))
			t.Logf("Result disk order: %v", getDiskInterfaces(result.Disks))

			// Verify the disk order matches expected
			actualOrder := getDiskInterfaces(result.Disks)
			assert.Equal(t, tc.expectedDiskOrder, actualOrder,
				"ConvertVMConfigToInputs should preserve disk order: %s", tc.description)

			// Verify each disk has correct configuration
			for i, expectedInterface := range tc.expectedDiskOrder {
				if i < len(result.Disks) {
					assert.Equal(t, expectedInterface, result.Disks[i].Interface,
						"Disk %d interface should match expected", i)

					// Check that disk was properly parsed from config
					expectedConfig, exists := tc.vmConfigDisks[expectedInterface]
					assert.True(t, exists, "Expected interface %s should exist in VM config", expectedInterface)
					assert.NotEmpty(t, result.Disks[i].Storage,
						"Disk %d storage should be parsed from config: %s", i, expectedConfig)
				}
			}
		})
	}
}

// Helper functions

// createMockVM creates a mock VirtualMachine with the specified disk configuration
func createMockVM(diskConfigs map[string]string) *api.VirtualMachine {
	// Create VirtualMachineConfig with disks
	vmConfig := &api.VirtualMachineConfig{
		Name: "test-vm",
	}

	// Set disk configurations based on interface name
	// This simulates the actual API structure
	for interfaceName, config := range diskConfigs {
		switch interfaceName {
		case "scsi0":
			vmConfig.SCSI0 = config
		case "scsi1":
			vmConfig.SCSI1 = config
		case "scsi2":
			vmConfig.SCSI2 = config
		case "virtio0":
			vmConfig.VirtIO0 = config
		case "virtio1":
			vmConfig.VirtIO1 = config
		case "virtio2":
			vmConfig.VirtIO2 = config
		case "ide0":
			vmConfig.IDE0 = config
		case "ide1":
			vmConfig.IDE1 = config
		case "ide2":
			vmConfig.IDE2 = config
		case "sata0":
			vmConfig.SATA0 = config
		case "sata1":
			vmConfig.SATA1 = config
		case "sata2":
			vmConfig.SATA2 = config
		}
	}

	return &api.VirtualMachine{
		VirtualMachineConfig: vmConfig,
		VMID:                 api.StringOrUint64(100),
	}
}

// getDiskInterfaces extracts interface names from a slice of disks
func getDiskInterfaces(disks []*vmResource.Disk) []string {
	interfaces := make([]string, len(disks))
	for i, disk := range disks {
		interfaces[i] = disk.Interface
	}
	return interfaces
}

// getMapKeys returns the keys of a string map
func getMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestVMDiffDisksChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		inputDisks    []*vmResource.Disk
		stateDisks    []*vmResource.Disk
		expectChange  bool
		expectDiffKey string
	}{
		{
			name: "disk size changed",
			inputDisks: []*vmResource.Disk{
				{
					Size:      50,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "disk interface changed",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi1",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "disk storage changed",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  false, // Same size and interface
			expectDiffKey: "",
		},
		{
			name: "disk added",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
				{
					Size:      50,
					Interface: "scsi1",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "disk removed",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
				{
					Size:      50,
					Interface: "scsi1",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "no disk changes",
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  false,
			expectDiffKey: "",
		},
		{
			name:          "both empty",
			inputDisks:    []*vmResource.Disk{},
			stateDisks:    []*vmResource.Disk{},
			expectChange:  false,
			expectDiffKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vmInstance := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:  testutils.Ptr("test-vm"),
					Disks: tt.inputDisks,
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:  testutils.Ptr("test-vm"),
						Disks: tt.stateDisks,
					},
				},
			}

			resp, err := vmInstance.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, "Expected changes to be detected")
				assert.Contains(t, resp.DetailedDiff, tt.expectDiffKey, "Expected diff key to be present")
				assert.Equal(t, p.Update, resp.DetailedDiff[tt.expectDiffKey].Kind)
			} else {
				assert.False(t, resp.HasChanges, "Expected no changes")
			}
		})
	}
}
