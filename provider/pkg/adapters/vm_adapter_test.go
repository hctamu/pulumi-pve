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

package adapters_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

const (
	lvmStorage = "local-lvm"
	ssdStorage = "local-ssd"
	hddStorage = "local-hdd"
)

func cpuBase(typ string) *proxmox.CPU {
	return &proxmox.CPU{Type: testutils.Ptr(typ)}
}

// cpuWith creates a CPU config with multiple fields set via a map for concise test case definitions
func cpuWith(typ string, fields map[string]interface{}) *proxmox.CPU {
	var cpu *proxmox.CPU
	if typ != "" {
		cpu = cpuBase(typ)
	} else {
		cpu = &proxmox.CPU{}
	}
	for k, v := range fields {
		switch k {
		case "cores":
			cpu.Cores = testutils.Ptr(v.(int))
		case "sockets":
			cpu.Sockets = testutils.Ptr(v.(int))
		case "limit":
			cpu.Limit = testutils.Ptr(v.(float64))
		case "units":
			cpu.Units = testutils.Ptr(v.(int))
		case "vcpus":
			cpu.Vcpus = testutils.Ptr(v.(int))
		case "numa":
			cpu.Numa = testutils.Ptr(v.(bool))
		case "hidden":
			cpu.Hidden = testutils.Ptr(v.(bool))
		case "flags+":
			cpu.FlagsEnabled = v.([]string)
		case "flags-":
			cpu.FlagsDisabled = v.([]string)
		case "hv-vendor-id":
			cpu.HVVendorID = testutils.Ptr(v.(string))
		case "phys-bits":
			cpu.PhysBits = testutils.Ptr(v.(string))
		case "numa-nodes":
			cpu.NumaNodes = v.([]proxmox.NumaNode)
		}
	}
	return cpu
}

// inputsWithCPU wraps a CPU config in an Inputs struct for test convenience
func inputsWithCPU(cpu *proxmox.CPU) proxmox.VMInputs {
	return proxmox.VMInputs{CPU: cpu}
}

// opt creates a VirtualMachineOption concisely for expected test results
func opt(name string, value interface{}) api.VirtualMachineOption {
	return api.VirtualMachineOption{Name: name, Value: value}
}

// filterCPUOptions filters VirtualMachineOptions to only CPU-related options
func filterCPUOptions(options []api.VirtualMachineOption) []api.VirtualMachineOption {
	cpuRelated := []string{
		"cpu", "cores", "sockets", "cpulimit", "cpuunits", "vcpus", "numa",
		"numa0", "numa1", "numa2", "numa3", "numa4", "numa5", "numa6", "numa7", "numa8", "numa9",
	}

	filtered := []api.VirtualMachineOption{}
	for _, opt := range options {
		for _, cpuOpt := range cpuRelated {
			if opt.Name == cpuOpt {
				filtered = append(filtered, opt)
				break
			}
		}
	}
	return filtered
}

// createTestDisk creates a test disk with the specified interface, storage, and size
func createTestDisk(iface, storage string, size int) *proxmox.Disk {
	return &proxmox.Disk{
		Interface: iface,
		DiskBase: proxmox.DiskBase{
			Storage: storage,
		},
		Size: size,
	}
}

// createTestCDROMDisk creates a test CD-ROM disk
func createTestCDROMDisk(iface, storage string) *proxmox.Disk {
	return &proxmox.Disk{
		Interface: iface,
		DiskBase: proxmox.DiskBase{
			Storage: storage,
		},
		Size: 0,
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
func getDiskInterfaces(disks []*proxmox.Disk) []string {
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

func TestParseCPU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    *proxmox.CPU
		expectError bool
		description string
	}{
		{
			name:        "empty string returns nil",
			input:       "",
			expected:    nil,
			expectError: false,
		},
		{
			name:  "simple CPU type - host",
			input: "host",
			expected: &proxmox.CPU{
				Type: testutils.Ptr("host"),
			},
			expectError: false,
		},
		{
			name:  "simple CPU type - kvm64",
			input: "kvm64",
			expected: &proxmox.CPU{
				Type: testutils.Ptr("kvm64"),
			},
			expectError: false,
		},
		{
			name:  "CPU type with cputype key",
			input: "cputype=x86-64-v2-AES",
			expected: &proxmox.CPU{
				Type: testutils.Ptr("x86-64-v2-AES"),
			},
			expectError: false,
		},
		{
			name:  "CPU with single enabled flag",
			input: "host,flags=+aes",
			expected: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "CPU with single disabled flag",
			input: "host,flags=-pcid",
			expected: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsDisabled: []string{"pcid"},
			},
			expectError: false,
		},
		{
			name:  "CPU with mixed enabled and disabled flags",
			input: "host,flags=+aes;-pcid;+avx2",
			expected: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes", "avx2"},
				FlagsDisabled: []string{"pcid"},
			},
			expectError: false,
		},
		{
			name:  "CPU with flag without prefix (treated as enabled)",
			input: "host,flags=aes",
			expected: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "CPU with mixed prefixed and unprefixed flags",
			input: "host,flags=aes;-pcid;+avx2;spec-ctrl",
			expected: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes", "avx2", "spec-ctrl"},
				FlagsDisabled: []string{"pcid"},
			},
			expectError: false,
		},
		{
			name:  "CPU with hidden=1",
			input: "host,hidden=1",
			expected: &proxmox.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(true),
			},
			expectError: false,
		},
		{
			name:  "CPU with hidden=0",
			input: "host,hidden=0",
			expected: &proxmox.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(false),
			},
			expectError: false,
		},
		{
			name:  "CPU with hv-vendor-id",
			input: "host,hv-vendor-id=AuthenticAMD",
			expected: &proxmox.CPU{
				Type:       testutils.Ptr("host"),
				HVVendorID: testutils.Ptr("AuthenticAMD"),
			},
			expectError: false,
		},
		{
			name:  "CPU with phys-bits",
			input: "host,phys-bits=40",
			expected: &proxmox.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("40"),
			},
			expectError: false,
		},
		{
			name:  "CPU with phys-bits=host",
			input: "host,phys-bits=host",
			expected: &proxmox.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("host"),
			},
			expectError: false,
		},
		{
			name:  "comprehensive CPU config",
			input: "host,flags=+aes;-pcid;+avx2,hidden=1,hv-vendor-id=GenuineIntel,phys-bits=42",
			expected: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes", "avx2"},
				FlagsDisabled: []string{"pcid"},
				Hidden:        testutils.Ptr(true),
				HVVendorID:    testutils.Ptr("GenuineIntel"),
				PhysBits:      testutils.Ptr("42"),
			},
			expectError: false,
		},
		{
			name:  "empty flags value",
			input: "host,flags=",
			expected: &proxmox.CPU{
				Type: testutils.Ptr("host"),
			},
			expectError: false,
		},
		{
			name:  "flags with empty segments",
			input: "host,flags=+aes;;-pcid",
			expected: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes"},
				FlagsDisabled: []string{"pcid"},
			},
			expectError: false,
		},
		{
			name:  "multiple commas in config",
			input: "host,,flags=+aes",
			expected: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "unknown keys are ignored",
			input: "host,unknown=value,flags=+aes",
			expected: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "segment without equals sign after first",
			input: "host,someflag,flags=+aes",
			expected: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "hidden with invalid value",
			input: "host,hidden=invalid",
			expected: &proxmox.CPU{
				Type: testutils.Ptr("host"),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := adapters.ParseCPU(tt.input)

			if tt.expectError {
				require.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				if tt.expected == nil {
					assert.Nil(t, result, "Expected nil result for: %s", tt.description)
				} else {
					require.NotNil(t, result, "Expected non-nil result for: %s", tt.description)
					assert.Equal(t, tt.expected.Type, result.Type, "Type mismatch")
					assert.Equal(t, tt.expected.FlagsEnabled, result.FlagsEnabled, "FlagsEnabled mismatch")
					assert.Equal(t, tt.expected.FlagsDisabled, result.FlagsDisabled, "FlagsDisabled mismatch")
					assert.Equal(t, tt.expected.Hidden, result.Hidden, "Hidden mismatch")
					assert.Equal(t, tt.expected.HVVendorID, result.HVVendorID, "HVVendorID mismatch")
					assert.Equal(t, tt.expected.PhysBits, result.PhysBits, "PhysBits mismatch")
				}
			}
		})
	}
}

// TestParseNumaNode verifies that ParseNumaNode correctly parses NUMA node configuration strings
func TestParseNumaNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    *proxmox.NumaNode
		expectError bool
		description string
	}{
		{
			name:        "empty string returns nil",
			input:       "",
			expected:    nil,
			expectError: false,
		},
		{
			name:  "complete NUMA node config",
			input: "cpus=0-3,hostnodes=0,memory=2048,policy=bind",
			expected: &proxmox.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0"),
				Memory:    testutils.Ptr(2048),
				Policy:    testutils.Ptr("bind"),
			},
			expectError: false,
		},
		{
			name:  "only required cpus field",
			input: "cpus=0-1",
			expected: &proxmox.NumaNode{
				Cpus: "0-1",
			},
			expectError: false,
		},
		{
			name:  "cpus with hostnodes",
			input: "cpus=0-3,hostnodes=0-1",
			expected: &proxmox.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0-1"),
			},
			expectError: false,
		},
		{
			name:  "cpus with memory",
			input: "cpus=0-3,memory=4096",
			expected: &proxmox.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(4096),
			},
			expectError: false,
		},
		{
			name:  "cpus with policy",
			input: "cpus=0-3,policy=preferred",
			expected: &proxmox.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("preferred"),
			},
			expectError: false,
		},
		{
			name:  "single CPU core",
			input: "cpus=0",
			expected: &proxmox.NumaNode{
				Cpus: "0",
			},
			expectError: false,
		},
		{
			name:  "complex CPU range format",
			input: "cpus=0-3;5-7",
			expected: &proxmox.NumaNode{
				Cpus: "0-3;5-7",
			},
			expectError: false,
		},
		{
			name:  "multiple host nodes",
			input: "cpus=0-7,hostnodes=0-2",
			expected: &proxmox.NumaNode{
				Cpus:      "0-7",
				HostNodes: testutils.Ptr("0-2"),
			},
			expectError: false,
		},
		{
			name:  "policy interleave",
			input: "cpus=0-3,policy=interleave",
			expected: &proxmox.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("interleave"),
			},
			expectError: false,
		},
		{
			name:  "empty segments ignored",
			input: "cpus=0-3,,memory=2048",
			expected: &proxmox.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(2048),
			},
			expectError: false,
		},
		{
			name:  "unknown keys ignored",
			input: "cpus=0-3,unknown=value,memory=2048",
			expected: &proxmox.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(2048),
			},
			expectError: false,
		},
		{
			name:  "segments without equals sign ignored",
			input: "cpus=0-3,someflag,memory=2048",
			expected: &proxmox.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(2048),
			},
			expectError: false,
		},
		{
			name:        "missing required cpus field",
			input:       "memory=2048,policy=bind",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "empty cpus value",
			input:       "cpus=,memory=2048",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid memory value - non-numeric",
			input:       "cpus=0-3,memory=invalid",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid memory value - float",
			input:       "cpus=0-3,memory=2048.5",
			expected:    nil,
			expectError: true,
		},
		{
			name:  "negative memory value",
			input: "cpus=0-3,memory=-1024",
			expected: &proxmox.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(-1024),
			},
			expectError: false,
		},
		{
			name:  "large memory value",
			input: "cpus=0-15,memory=65536",
			expected: &proxmox.NumaNode{
				Cpus:   "0-15",
				Memory: testutils.Ptr(65536),
			},
			expectError: false,
		},
		{
			name:  "all fields in different order",
			input: "policy=bind,memory=4096,hostnodes=0-1,cpus=0-7",
			expected: &proxmox.NumaNode{
				Cpus:      "0-7",
				HostNodes: testutils.Ptr("0-1"),
				Memory:    testutils.Ptr(4096),
				Policy:    testutils.Ptr("bind"),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := adapters.ParseNumaNode(tt.input)

			if tt.expectError {
				require.Error(t, err, tt.description)
				assert.Nil(t, result, "Result should be nil on error")
			} else {
				require.NoError(t, err, tt.description)
				if tt.expected == nil {
					assert.Nil(t, result, "Expected nil result for: %s", tt.description)
				} else {
					require.NotNil(t, result, "Expected non-nil result for: %s", tt.description)
					assert.Equal(t, tt.expected.Cpus, result.Cpus, "Cpus mismatch")
					assert.Equal(t, tt.expected.HostNodes, result.HostNodes, "HostNodes mismatch")
					assert.Equal(t, tt.expected.Memory, result.Memory, "Memory mismatch")
					assert.Equal(t, tt.expected.Policy, result.Policy, "Policy mismatch")
				}
			}
		})
	}
}

// TestNumaNodesEqual verifies that numaNodesEqual correctly compares NUMA node slices
func TestNumaNodesEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		inputA   []proxmox.NumaNode
		inputB   []proxmox.NumaNode
		expected bool
	}{
		{
			name:     "both nil slices",
			inputA:   nil,
			inputB:   nil,
			expected: true,
		},
		{
			name:     "both empty slices",
			inputA:   []proxmox.NumaNode{},
			inputB:   []proxmox.NumaNode{},
			expected: true,
		},
		{
			name: "equal single node - all fields",
			inputA: []proxmox.NumaNode{
				{
					Cpus:      "0-3",
					HostNodes: testutils.Ptr("0-1"),
					Memory:    testutils.Ptr(2048),
					Policy:    testutils.Ptr("bind"),
				},
			},
			inputB: []proxmox.NumaNode{
				{
					Cpus:      "0-3",
					HostNodes: testutils.Ptr("0-1"),
					Memory:    testutils.Ptr(2048),
					Policy:    testutils.Ptr("bind"),
				},
			},
			expected: true,
		},
		{
			name: "equal single node - minimal",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3"},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3"},
			},
			expected: true,
		},
		{
			name: "equal multiple nodes",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: testutils.Ptr(2048)},
				{Cpus: "4-7", Memory: testutils.Ptr(4096)},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: testutils.Ptr(2048)},
				{Cpus: "4-7", Memory: testutils.Ptr(4096)},
			},
			expected: true,
		},
		{
			name: "different lengths - first longer",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3"},
				{Cpus: "4-7"},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3"},
			},
			expected: false,
		},
		{
			name: "different lengths - second longer",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3"},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3"},
				{Cpus: "4-7"},
			},
			expected: false,
		},
		{
			name: "different Cpus field",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3"},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-7"},
			},
			expected: false,
		},
		{
			name: "different HostNodes - first nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: nil},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: testutils.Ptr("0-1")},
			},
			expected: false,
		},
		{
			name: "different HostNodes - second nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: testutils.Ptr("0-1")},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: nil},
			},
			expected: false,
		},
		{
			name: "different HostNodes - both non-nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: testutils.Ptr("0-1")},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: testutils.Ptr("0-2")},
			},
			expected: false,
		},
		{
			name: "equal HostNodes - both nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: nil},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: nil},
			},
			expected: true,
		},
		{
			name: "different Memory - first nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: nil},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: testutils.Ptr(2048)},
			},
			expected: false,
		},
		{
			name: "different Memory - second nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: testutils.Ptr(2048)},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: nil},
			},
			expected: false,
		},
		{
			name: "different Memory - both non-nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: testutils.Ptr(2048)},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: testutils.Ptr(4096)},
			},
			expected: false,
		},
		{
			name: "equal Memory - both nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: nil},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: nil},
			},
			expected: true,
		},
		{
			name: "different Policy - first nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Policy: nil},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Policy: testutils.Ptr("bind")},
			},
			expected: false,
		},
		{
			name: "different Policy - second nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Policy: testutils.Ptr("bind")},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Policy: nil},
			},
			expected: false,
		},
		{
			name: "different Policy - both non-nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Policy: testutils.Ptr("bind")},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Policy: testutils.Ptr("interleave")},
			},
			expected: false,
		},
		{
			name: "equal Policy - both nil",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Policy: nil},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Policy: nil},
			},
			expected: true,
		},
		{
			name: "complex - multiple nodes with all equal",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: testutils.Ptr("0"), Memory: testutils.Ptr(2048), Policy: testutils.Ptr("bind")},
				{Cpus: "4-7", HostNodes: testutils.Ptr("1"), Memory: testutils.Ptr(2048), Policy: testutils.Ptr("bind")},
				{Cpus: "8-11", HostNodes: nil, Memory: nil, Policy: nil},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", HostNodes: testutils.Ptr("0"), Memory: testutils.Ptr(2048), Policy: testutils.Ptr("bind")},
				{Cpus: "4-7", HostNodes: testutils.Ptr("1"), Memory: testutils.Ptr(2048), Policy: testutils.Ptr("bind")},
				{Cpus: "8-11", HostNodes: nil, Memory: nil, Policy: nil},
			},
			expected: true,
		},
		{
			name: "complex - difference in second node",
			inputA: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: testutils.Ptr(2048)},
				{Cpus: "4-7", Memory: testutils.Ptr(2048)},
			},
			inputB: []proxmox.NumaNode{
				{Cpus: "0-3", Memory: testutils.Ptr(2048)},
				{Cpus: "4-7", Memory: testutils.Ptr(4096)},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := adapters.NumaNodesEqual(tt.inputA, tt.inputB)
			assert.Equal(t, tt.expected, result)

			// Verify symmetry - adapters.NumaNodesEqual(a, b) should equal adapters.NumaNodesEqual(b, a)
			resultReverse := adapters.NumaNodesEqual(tt.inputB, tt.inputA)
			assert.Equal(t, tt.expected, resultReverse, "NumaNodesEqual should be symmetric")
		})
	}
}

// TestCPUToProxmoxString verifies that ToProxmoxString correctly serializes CPU struct to Proxmox format
func TestCPURoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		description string
	}{
		{
			name:  "simple CPU type",
			input: "host",
		},
		{
			name:  "CPU type kvm64",
			input: "kvm64",
		},
		{
			name:  "CPU with single enabled flag",
			input: "host,flags=+aes",
		},
		{
			name:  "CPU with single disabled flag",
			input: "host,flags=-pcid",
		},
		{
			name:  "CPU with mixed flags",
			input: "host,flags=+aes;+avx2;-pcid",
		},
		{
			name:  "CPU with hidden true",
			input: "host,hidden=1",
		},
		{
			name:  "CPU with hidden false",
			input: "host,hidden=0",
		},
		{
			name:  "CPU with hv-vendor-id",
			input: "host,hv-vendor-id=AuthenticAMD",
		},
		{
			name:  "CPU with phys-bits numeric",
			input: "host,phys-bits=40",
		},
		{
			name:  "CPU with phys-bits host",
			input: "host,phys-bits=host",
		},
		{
			name:  "comprehensive CPU config",
			input: "host,flags=+aes;+avx2;-pcid,hidden=1,hv-vendor-id=GenuineIntel,phys-bits=42",
		},
		{
			name:  "CPU with multiple enabled flags",
			input: "host,flags=+aes;+avx2;+spec-ctrl",
		},
		{
			name:  "CPU with multiple disabled flags",
			input: "host,flags=-pcid;-spec-ctrl",
		},
		{
			name:  "complex CPU type",
			input: "x86-64-v2-AES",
		},
		{
			name:  "CPU with hidden and flags",
			input: "host,flags=+aes,hidden=0",
		},
		{
			name:  "CPU with all optional fields",
			input: "kvm64,flags=+aes;-pcid,hidden=1,hv-vendor-id=GenuineIntel,phys-bits=host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed1, err := adapters.ParseCPU(tt.input)
			require.NoError(t, err, "First parse should not error")
			require.NotNil(t, parsed1, "First parse should return non-nil CPU")

			serialized := adapters.CPUToProxmoxString(parsed1)
			require.NotEmpty(t, serialized, "Serialization should produce non-empty string")

			parsed2, err := adapters.ParseCPU(serialized)
			require.NoError(t, err, "Second parse should not error")
			require.NotNil(t, parsed2, "Second parse should return non-nil CPU")

			assert.Equal(t, parsed1.Type, parsed2.Type, "CPU Type should match after round-trip")
			assert.Equal(t, parsed1.FlagsEnabled, parsed2.FlagsEnabled, "FlagsEnabled should match after round-trip")
			assert.Equal(t, parsed1.FlagsDisabled, parsed2.FlagsDisabled, "FlagsDisabled should match after round-trip")
			assert.Equal(t, parsed1.Hidden, parsed2.Hidden, "Hidden should match after round-trip")
			assert.Equal(t, parsed1.HVVendorID, parsed2.HVVendorID, "HVVendorID should match after round-trip")
			assert.Equal(t, parsed1.PhysBits, parsed2.PhysBits, "PhysBits should match after round-trip")

			serialized2 := adapters.CPUToProxmoxString(parsed2)
			assert.Equal(t, serialized, serialized2, "Second serialization should match first serialization")
		})
	}
}

// TestBuildOptionsCPUFields verifies that CPU fields generate correct VirtualMachineOptions
func TestBuildOptionsCPUFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputs      proxmox.VMInputs
		expected    []api.VirtualMachineOption
		description string
	}{
		{
			name: "basic CPU with cores and sockets",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Cores:   testutils.Ptr(4),
					Sockets: testutils.Ptr(2),
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "cores", Value: testutils.Ptr(4)},
				{Name: "sockets", Value: testutils.Ptr(2)},
			},
		},
		{
			name: "CPU with type only",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Type: testutils.Ptr("host"),
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "cpu", Value: "host"},
			},
		},
		{
			name: "CPU with limit, units, and vcpus",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Limit: testutils.Ptr(2.5),
					Units: testutils.Ptr(2048),
					Vcpus: testutils.Ptr(8),
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "cpulimit", Value: testutils.Ptr(2.5)},
				{Name: "cpuunits", Value: testutils.Ptr(2048)},
				{Name: "vcpus", Value: testutils.Ptr(8)},
			},
		},
		{
			name: "CPU with NUMA enabled",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Numa: testutils.Ptr(true),
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "numa", Value: 1},
			},
		},
		{
			name: "CPU with NUMA disabled",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Numa: testutils.Ptr(false),
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "numa", Value: 0},
			},
		},
		{
			name: "CPU with single NUMA node",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					NumaNodes: []proxmox.NumaNode{
						{
							Cpus:      "0-3",
							HostNodes: testutils.Ptr("0"),
							Memory:    testutils.Ptr(2048),
							Policy:    testutils.Ptr("bind"),
						},
					},
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "numa0", Value: "cpus=0-3,hostnodes=0,memory=2048,policy=bind"},
			},
		},
		{
			name: "CPU with multiple NUMA nodes",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					NumaNodes: []proxmox.NumaNode{
						{Cpus: "0-1", Memory: testutils.Ptr(1024)},
						{Cpus: "2-3", Memory: testutils.Ptr(1024)},
						{Cpus: "4-5", Memory: testutils.Ptr(2048)},
					},
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "numa0", Value: "cpus=0-1,memory=1024"},
				{Name: "numa1", Value: "cpus=2-3,memory=1024"},
				{Name: "numa2", Value: "cpus=4-5,memory=2048"},
			},
		},
		{
			name: "CPU with all fields populated",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Type:          testutils.Ptr("host"),
					FlagsEnabled:  []string{"aes", "avx2"},
					FlagsDisabled: []string{"pcid"},
					Hidden:        testutils.Ptr(true),
					HVVendorID:    testutils.Ptr("GenuineIntel"),
					PhysBits:      testutils.Ptr("42"),
					Cores:         testutils.Ptr(4),
					Sockets:       testutils.Ptr(2),
					Limit:         testutils.Ptr(1.5),
					Units:         testutils.Ptr(1024),
					Vcpus:         testutils.Ptr(6),
					Numa:          testutils.Ptr(true),
					NumaNodes: []proxmox.NumaNode{
						{Cpus: "0-3", Memory: testutils.Ptr(4096)},
					},
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "cpu", Value: "host,flags=+aes;+avx2;-pcid,hidden=1,hv-vendor-id=GenuineIntel,phys-bits=42"},
				{Name: "cores", Value: testutils.Ptr(4)},
				{Name: "sockets", Value: testutils.Ptr(2)},
				{Name: "cpulimit", Value: testutils.Ptr(1.5)},
				{Name: "cpuunits", Value: testutils.Ptr(1024)},
				{Name: "vcpus", Value: testutils.Ptr(6)},
				{Name: "numa", Value: 1},
				{Name: "numa0", Value: "cpus=0-3,memory=4096"},
			},
		},
		{
			name: "nil CPU",
			inputs: proxmox.VMInputs{
				CPU: nil,
			},
			expected: []api.VirtualMachineOption{},
		},
		{
			name: "empty CPU struct",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{},
			},
			expected: []api.VirtualMachineOption{},
		},
		{
			name: "CPU with empty type string",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Type: testutils.Ptr(""),
				},
			},
			expected: []api.VirtualMachineOption{},
		},
		{
			name: "CPU with flags only",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					FlagsEnabled: []string{"aes", "avx2"},
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "cpu", Value: "flags=+aes;+avx2"},
			},
		},
		{
			name: "CPU with type and cores",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Type:  testutils.Ptr("kvm64"),
					Cores: testutils.Ptr(2),
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "cpu", Value: "kvm64"},
				{Name: "cores", Value: testutils.Ptr(2)},
			},
		},
		{
			name: "CPU with empty NUMA nodes slice",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					NumaNodes: []proxmox.NumaNode{},
				},
			},
			expected: []api.VirtualMachineOption{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Call BuildVMOptions with a dummy VMID
			options := adapters.BuildVMOptions(tt.inputs, 100)

			// Filter to only CPU-related options for easier comparison
			cpuOptions := filterCPUOptions(options)

			// Compare
			assert.Equal(t, len(tt.expected), len(cpuOptions),
				"Number of CPU options mismatch for: %s", tt.description)

			// Check each expected option exists with correct value
			for _, expected := range tt.expected {
				found := false
				for _, actual := range cpuOptions {
					if actual.Name == expected.Name {
						found = true
						assert.Equal(t, expected.Value, actual.Value,
							"Value mismatch for option %s: %s", expected.Name, tt.description)
						break
					}
				}
				assert.True(t, found, "Expected option %s not found: %s", expected.Name, tt.description)
			}
		})
	}
}

// TestBuildOptionsCPUConsistency verifies that BuildOptions produces deterministic output
func TestBuildOptionsCPUConsistency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputs      proxmox.VMInputs
		description string
	}{
		{
			name: "simple CPU config",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Type:    testutils.Ptr("host"),
					Cores:   testutils.Ptr(4),
					Sockets: testutils.Ptr(2),
				},
			},
		},
		{
			name: "CPU with all fields",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Type:          testutils.Ptr("host"),
					FlagsEnabled:  []string{"aes", "avx2", "spec-ctrl"},
					FlagsDisabled: []string{"pcid"},
					Hidden:        testutils.Ptr(true),
					HVVendorID:    testutils.Ptr("GenuineIntel"),
					PhysBits:      testutils.Ptr("42"),
					Cores:         testutils.Ptr(8),
					Sockets:       testutils.Ptr(2),
					Limit:         testutils.Ptr(2.5),
					Units:         testutils.Ptr(2048),
					Vcpus:         testutils.Ptr(12),
					Numa:          testutils.Ptr(true),
				},
			},
		},
		{
			name: "CPU with NUMA nodes",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Type:  testutils.Ptr("kvm64"),
					Cores: testutils.Ptr(8),
					Numa:  testutils.Ptr(true),
					NumaNodes: []proxmox.NumaNode{
						{Cpus: "0-1", Memory: testutils.Ptr(1024), Policy: testutils.Ptr("bind")},
						{Cpus: "2-3", Memory: testutils.Ptr(1024), Policy: testutils.Ptr("bind")},
						{Cpus: "4-5", Memory: testutils.Ptr(2048), Policy: testutils.Ptr("preferred")},
						{Cpus: "6-7", Memory: testutils.Ptr(2048), HostNodes: testutils.Ptr("1")},
					},
				},
			},
		},
		{
			name: "minimal CPU",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Type: testutils.Ptr("host"),
				},
			},
		},
		{
			name: "CPU with only numeric fields",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Cores:   testutils.Ptr(16),
					Sockets: testutils.Ptr(4),
					Limit:   testutils.Ptr(3.0),
					Units:   testutils.Ptr(4096),
					Vcpus:   testutils.Ptr(32),
				},
			},
		},
		{
			name: "CPU with flags and hidden",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{
					Type:          testutils.Ptr("x86-64-v2-AES"),
					FlagsEnabled:  []string{"aes", "avx", "avx2", "sse4.1", "sse4.2"},
					FlagsDisabled: []string{"pcid", "spec-ctrl", "ssbd"},
					Hidden:        testutils.Ptr(false),
					HVVendorID:    testutils.Ptr("AuthenticAMD"),
				},
			},
		},
		{
			name: "empty CPU struct",
			inputs: proxmox.VMInputs{
				CPU: &proxmox.CPU{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Call BuildVMOptions multiple times
			const iterations = 10
			var results [][]api.VirtualMachineOption

			for i := 0; i < iterations; i++ {
				options := adapters.BuildVMOptions(tt.inputs, 100+i) // Use different VMIDs
				cpuOptions := filterCPUOptions(options)
				results = append(results, cpuOptions)
			}

			// Verify all results are identical
			firstResult := results[0]
			for i := 1; i < iterations; i++ {
				assert.Equal(t, len(firstResult), len(results[i]),
					"Result %d has different length than first result: %s", i, tt.description)

				// Check each option matches
				for j, expectedOpt := range firstResult {
					if j >= len(results[i]) {
						t.Errorf("Result %d missing option at index %d: %s", i, j, tt.description)
						continue
					}

					actualOpt := results[i][j]
					assert.Equal(t, expectedOpt.Name, actualOpt.Name,
						"Result %d option %d name mismatch: %s", i, j, tt.description)
					assert.Equal(t, expectedOpt.Value, actualOpt.Value,
						"Result %d option %d value mismatch: %s", i, j, tt.description)
				}
			}

			// Verify option order is consistent
			// Check that specific CPU options appear in expected order
			if len(firstResult) > 0 {
				// Build a map of option positions
				optionOrder := make(map[string]int)
				for idx, opt := range firstResult {
					optionOrder[opt.Name] = idx
				}

				// Verify logical ordering: cpu comes before cores/sockets/etc
				if cpuIdx, hasCPU := optionOrder["cpu"]; hasCPU {
					if coresIdx, hasCores := optionOrder["cores"]; hasCores {
						assert.Less(t, cpuIdx, coresIdx,
							"cpu option should come before cores: %s", tt.description)
					}
					if socketsIdx, hasSockets := optionOrder["sockets"]; hasSockets {
						assert.Less(t, cpuIdx, socketsIdx,
							"cpu option should come before sockets: %s", tt.description)
					}
				}

				// Verify NUMA nodes are ordered sequentially (numa0 before numa1, etc.)
				// Check if numa0, numa1, numa2, etc. appear in order
				lastNumaIdx := -1
				for numaNum := 0; numaNum <= 9; numaNum++ {
					numaName := "numa" + string('0'+byte(numaNum))
					if idx, exists := optionOrder[numaName]; exists {
						if lastNumaIdx >= 0 {
							assert.Less(t, lastNumaIdx, idx,
								"NUMA nodes should appear in sequential order (%s should come after previous numa): %s",
								numaName, tt.description)
						}
						lastNumaIdx = idx
					}
				}
			}
		})
	}
}

// TestAddCPUDiff verifies that addCPUDiff generates correct diff options
func TestAddCPUDiff(t *testing.T) {
	t.Parallel()

	host := cpuWith("host", map[string]interface{}{"cores": 4, "sockets": 2})

	tests := []struct {
		name      string
		newInputs proxmox.VMInputs
		oldInputs proxmox.VMInputs
		expected  []api.VirtualMachineOption
	}{
		{
			name:      "no changes - identical CPU configs",
			newInputs: inputsWithCPU(host),
			oldInputs: inputsWithCPU(host),
			expected:  []api.VirtualMachineOption{},
		},
		{
			name:      "type changed",
			newInputs: inputsWithCPU(cpuBase("kvm64")),
			oldInputs: inputsWithCPU(cpuBase("host")),
			expected:  []api.VirtualMachineOption{opt("cpu", "kvm64")},
		},
		{
			name:      "cores changed",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"cores": 8})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"cores": 4})),
			expected:  []api.VirtualMachineOption{opt("cores", testutils.Ptr(8))},
		},
		{
			name:      "sockets changed",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"sockets": 4})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"sockets": 2})),
			expected:  []api.VirtualMachineOption{opt("sockets", testutils.Ptr(4))},
		},
		{
			name:      "limit changed",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"limit": 3.0})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"limit": 2.0})),
			expected:  []api.VirtualMachineOption{opt("cpulimit", testutils.Ptr(3.0))},
		},
		{
			name:      "units changed",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"units": 2048})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"units": 1024})),
			expected:  []api.VirtualMachineOption{opt("cpuunits", testutils.Ptr(2048))},
		},
		{
			name:      "vcpus changed",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"vcpus": 16})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"vcpus": 8})),
			expected:  []api.VirtualMachineOption{opt("vcpus", testutils.Ptr(16))},
		},
		{
			name:      "numa enabled - false to true",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"numa": true})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"numa": false})),
			expected:  []api.VirtualMachineOption{opt("numa", 1)},
		},
		{
			name:      "numa disabled - true to false",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"numa": false})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"numa": true})),
			expected:  []api.VirtualMachineOption{opt("numa", 0)},
		},
		{
			name:      "flags changed",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"flags+": []string{"aes", "avx2"}})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"flags+": []string{"aes"}})),
			expected:  []api.VirtualMachineOption{opt("cpu", "host,flags=+aes;+avx2")},
		},
		{
			name:      "hidden changed",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"hidden": true})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"hidden": false})),
			expected:  []api.VirtualMachineOption{opt("cpu", "host,hidden=1")},
		},
		{
			name:      "hv-vendor-id changed",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"hv-vendor-id": "GenuineIntel"})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"hv-vendor-id": "AuthenticAMD"})),
			expected:  []api.VirtualMachineOption{opt("cpu", "host,hv-vendor-id=GenuineIntel")},
		},
		{
			name:      "phys-bits changed",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"phys-bits": "42"})),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"phys-bits": "40"})),
			expected:  []api.VirtualMachineOption{opt("cpu", "host,phys-bits=42")},
		},
		{
			name: "multiple fields changed",
			newInputs: inputsWithCPU(
				cpuWith("kvm64", map[string]interface{}{"cores": 8, "sockets": 4, "limit": 3.0, "units": 2048}),
			),
			oldInputs: inputsWithCPU(
				cpuWith("host", map[string]interface{}{"cores": 4, "sockets": 2, "limit": 2.0, "units": 1024}),
			),
			expected: []api.VirtualMachineOption{
				opt("cpu", "kvm64"),
				opt("cores", testutils.Ptr(8)),
				opt("sockets", testutils.Ptr(4)),
				opt("cpulimit", testutils.Ptr(3.0)),
				opt("cpuunits", testutils.Ptr(2048)),
			},
		},
		{
			name:      "nil to populated",
			newInputs: inputsWithCPU(host),
			oldInputs: inputsWithCPU(nil),
			expected: []api.VirtualMachineOption{
				opt("cpu", "host"),
				opt("cores", testutils.Ptr(4)),
				opt("sockets", testutils.Ptr(2)),
			},
		},
		{
			name:      "populated to nil",
			newInputs: inputsWithCPU(nil),
			oldInputs: inputsWithCPU(host),
			expected:  []api.VirtualMachineOption{},
		},
		{
			name: "numa nodes added",
			newInputs: inputsWithCPU(
				cpuWith(
					"host",
					map[string]interface{}{
						"numa-nodes": []proxmox.NumaNode{
							{Cpus: "0-1", Memory: testutils.Ptr(1024)},
							{Cpus: "2-3", Memory: testutils.Ptr(1024)},
						},
					},
				),
			),
			oldInputs: inputsWithCPU(cpuBase("host")),
			expected:  []api.VirtualMachineOption{opt("numa0", "cpus=0-1,memory=1024"), opt("numa1", "cpus=2-3,memory=1024")},
		},
		{
			name: "numa nodes changed",
			newInputs: inputsWithCPU(
				cpuWith(
					"host",
					map[string]interface{}{"numa-nodes": []proxmox.NumaNode{{Cpus: "0-3", Memory: testutils.Ptr(2048)}}},
				),
			),
			oldInputs: inputsWithCPU(
				cpuWith(
					"host",
					map[string]interface{}{"numa-nodes": []proxmox.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(1024)}}},
				),
			),
			expected: []api.VirtualMachineOption{opt("numa0", "cpus=0-3,memory=2048")},
		},
		{
			name:      "numa nodes removed",
			newInputs: inputsWithCPU(cpuBase("host")),
			oldInputs: inputsWithCPU(
				cpuWith(
					"host",
					map[string]interface{}{"numa-nodes": []proxmox.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(1024)}}},
				),
			),
			expected: []api.VirtualMachineOption{},
		},
		{
			name:      "field added from nil",
			newInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"cores": 4})),
			oldInputs: inputsWithCPU(cpuBase("host")),
			expected:  []api.VirtualMachineOption{opt("cores", testutils.Ptr(4))},
		},
		{
			name:      "field removed to nil",
			newInputs: inputsWithCPU(cpuBase("host")),
			oldInputs: inputsWithCPU(cpuWith("host", map[string]interface{}{"cores": 4})),
			expected:  []api.VirtualMachineOption{},
		},
		{
			name: "comprehensive change - all fields",
			newInputs: inputsWithCPU(&proxmox.CPU{
				Type: testutils.Ptr(
					"kvm64",
				), FlagsEnabled: []string{"aes", "avx2"}, FlagsDisabled: []string{"pcid"}, Hidden: testutils.Ptr(true),
				HVVendorID: testutils.Ptr(
					"GenuineIntel",
				), PhysBits: testutils.Ptr("42"), Cores: testutils.Ptr(16), Sockets: testutils.Ptr(4),
				Limit: testutils.Ptr(4.0), Units: testutils.Ptr(4096), Vcpus: testutils.Ptr(32), Numa: testutils.Ptr(true),
				NumaNodes: []proxmox.NumaNode{
					{Cpus: "0-7", Memory: testutils.Ptr(4096)},
					{Cpus: "8-15", Memory: testutils.Ptr(4096)},
				},
			}),
			oldInputs: inputsWithCPU(host),
			expected: []api.VirtualMachineOption{
				opt("cpu", "kvm64,flags=+aes;+avx2;-pcid,hidden=1,hv-vendor-id=GenuineIntel,phys-bits=42"),
				opt("cores", testutils.Ptr(16)), opt("sockets", testutils.Ptr(4)), opt("cpulimit", testutils.Ptr(4.0)),
				opt("cpuunits", testutils.Ptr(4096)), opt("vcpus", testutils.Ptr(32)), opt("numa", 1),
				opt("numa0", "cpus=0-7,memory=4096"), opt("numa1", "cpus=8-15,memory=4096"),
			},
		},
		{
			name:      "both nil",
			newInputs: inputsWithCPU(nil),
			oldInputs: inputsWithCPU(nil),
			expected:  []api.VirtualMachineOption{},
		},
		{
			name:      "empty CPU structs",
			newInputs: inputsWithCPU(&proxmox.CPU{}),
			oldInputs: inputsWithCPU(&proxmox.CPU{}),
			expected:  []api.VirtualMachineOption{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			options := adapters.BuildVMOptionsDiff(tt.newInputs, 100, &tt.oldInputs)
			cpuOptions := filterCPUOptions(options)

			require.Len(t, cpuOptions, len(tt.expected), "Number of CPU diff options mismatch")
			for _, exp := range tt.expected {
				found := false
				for _, act := range cpuOptions {
					if act.Name == exp.Name {
						assert.Equal(t, exp.Value, act.Value, "Value mismatch for option %s", exp.Name)
						found = true
						break
					}
				}
				assert.True(t, found, "Expected option %s not found", exp.Name)
			}
		})
	}
}

func TestNumaNodeRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		description string
	}{
		{
			name:  "minimal - only cpus",
			input: "cpus=0-1",
		},
		{
			name:  "complete NUMA config",
			input: "cpus=0-3,hostnodes=0,memory=2048,policy=bind",
		},
		{
			name:  "cpus with hostnodes",
			input: "cpus=0-3,hostnodes=0-1",
		},
		{
			name:  "cpus with memory",
			input: "cpus=0-3,memory=4096",
		},
		{
			name:  "cpus with policy",
			input: "cpus=0-3,policy=preferred",
		},
		{
			name:  "single CPU core",
			input: "cpus=0",
		},
		{
			name:  "complex CPU range",
			input: "cpus=0-3;5-7",
		},
		{
			name:  "policy interleave",
			input: "cpus=0-3,policy=interleave",
		},
		{
			name:  "large memory value",
			input: "cpus=0-15,memory=65536",
		},
		{
			name:  "all optional fields",
			input: "cpus=0-7,hostnodes=0-1,memory=4096,policy=bind",
		},
		{
			name:  "hostnodes and memory without policy",
			input: "cpus=0-3,hostnodes=0,memory=2048",
		},
		{
			name:  "hostnodes and policy without memory",
			input: "cpus=0-3,hostnodes=0-1,policy=bind",
		},
		{
			name:  "memory and policy without hostnodes",
			input: "cpus=0-3,memory=4096,policy=interleave",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed1, err := adapters.ParseNumaNode(tt.input)
			require.NoError(t, err, "First parse should not error")
			require.NotNil(t, parsed1, "First parse should return non-nil NumaNode")

			serialized := adapters.ToProxmoxNumaString(*parsed1)
			require.NotEmpty(t, serialized, "Serialization should produce non-empty string")

			parsed2, err := adapters.ParseNumaNode(serialized)
			require.NoError(t, err, "Second parse should not error")
			require.NotNil(t, parsed2, "Second parse should return non-nil NumaNode")

			assert.Equal(t, parsed1.Cpus, parsed2.Cpus, "Cpus should match after round-trip")
			assert.Equal(t, parsed1.HostNodes, parsed2.HostNodes, "HostNodes should match after round-trip")
			assert.Equal(t, parsed1.Memory, parsed2.Memory, "Memory should match after round-trip")
			assert.Equal(t, parsed1.Policy, parsed2.Policy, "Policy should match after round-trip")

			serialized2 := adapters.ToProxmoxNumaString(*parsed2)
			assert.Equal(t, serialized, serialized2, "Second serialization should match first serialization")
		})
	}
}

// TestParseCPUFromVMConfig verifies that CPU parsing from VirtualMachineConfig works correctly
func TestParseCPUFromVMConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		vmConfig    *api.VirtualMachineConfig
		expected    *proxmox.CPU
		expectError bool
	}{
		{
			name: "empty config returns empty CPU",
			vmConfig: &api.VirtualMachineConfig{
				CPU:      "",
				Cores:    0,
				Sockets:  0,
				CPULimit: 0,
				CPUUnits: 0,
				Vcpus:    0,
				Numa:     0,
			},
			expected: &proxmox.CPU{},
		},
		{
			name: "CPU string only - simple type",
			vmConfig: &api.VirtualMachineConfig{
				CPU: "host",
			},
			expected: cpuBase("host"),
		},
		{
			name: "CPU string with flags",
			vmConfig: &api.VirtualMachineConfig{
				CPU: "host,flags=+aes;-pcid",
			},
			expected: cpuWith("host", map[string]interface{}{
				"flags+": []string{"aes"},
				"flags-": []string{"pcid"},
			}),
		},
		{
			name: "cores field only",
			vmConfig: &api.VirtualMachineConfig{
				Cores: 4,
			},
			expected: &proxmox.CPU{
				Cores: testutils.Ptr(4),
			},
		},
		{
			name: "sockets field only",
			vmConfig: &api.VirtualMachineConfig{
				Sockets: 2,
			},
			expected: &proxmox.CPU{
				Sockets: testutils.Ptr(2),
			},
		},
		{
			name: "CPULimit field",
			vmConfig: &api.VirtualMachineConfig{
				CPULimit: 2,
			},
			expected: &proxmox.CPU{
				Limit: testutils.Ptr(2.0),
			},
		},
		{
			name: "CPUUnits field",
			vmConfig: &api.VirtualMachineConfig{
				CPUUnits: 1024,
			},
			expected: &proxmox.CPU{
				Units: testutils.Ptr(1024),
			},
		},
		{
			name: "Vcpus field",
			vmConfig: &api.VirtualMachineConfig{
				Vcpus: 8,
			},
			expected: &proxmox.CPU{
				Vcpus: testutils.Ptr(8),
			},
		},
		{
			name: "NUMA enabled",
			vmConfig: &api.VirtualMachineConfig{
				Numa: 1,
			},
			expected: &proxmox.CPU{
				Numa: testutils.Ptr(true),
			},
		},
		{
			name: "single NUMA node",
			vmConfig: &api.VirtualMachineConfig{
				Numa:  1,
				Numa0: "cpus=0-3,memory=2048,policy=bind",
			},
			expected: cpuWith("", map[string]interface{}{
				"numa": true,
				"numa-nodes": []proxmox.NumaNode{
					{
						Cpus:   "0-3",
						Memory: testutils.Ptr(2048),
						Policy: testutils.Ptr("bind"),
					},
				},
			}),
		},
		{
			name: "multiple NUMA nodes",
			vmConfig: &api.VirtualMachineConfig{
				Numa:  1,
				Numa0: "cpus=0-3,memory=2048",
				Numa1: "cpus=4-7,memory=4096",
				Numa2: "cpus=8-11,hostnodes=2",
			},
			expected: cpuWith("", map[string]interface{}{
				"numa": true,
				"numa-nodes": []proxmox.NumaNode{
					{
						Cpus:   "0-3",
						Memory: testutils.Ptr(2048),
					},
					{
						Cpus:   "4-7",
						Memory: testutils.Ptr(4096),
					},
					{
						Cpus:      "8-11",
						HostNodes: testutils.Ptr("2"),
					},
				},
			}),
		},
		{
			name: "comprehensive config",
			vmConfig: &api.VirtualMachineConfig{
				CPU:      "host,flags=+aes;-pcid,hidden=1,hv-vendor-id=GenuineIntel,phys-bits=42",
				Cores:    8,
				Sockets:  2,
				CPULimit: 4,
				CPUUnits: 2048,
				Vcpus:    16,
				Numa:     1,
				Numa0:    "cpus=0-7,hostnodes=0,memory=4096,policy=bind",
				Numa1:    "cpus=8-15,hostnodes=1,memory=4096,policy=bind",
			},
			expected: cpuWith("host", map[string]interface{}{
				"cores":        8,
				"sockets":      2,
				"limit":        4.0,
				"units":        2048,
				"vcpus":        16,
				"numa":         true,
				"flags+":       []string{"aes"},
				"flags-":       []string{"pcid"},
				"hidden":       true,
				"hv-vendor-id": "GenuineIntel",
				"phys-bits":    "42",
				"numa-nodes": []proxmox.NumaNode{
					{
						Cpus:      "0-7",
						HostNodes: testutils.Ptr("0"),
						Memory:    testutils.Ptr(4096),
						Policy:    testutils.Ptr("bind"),
					},
					{
						Cpus:      "8-15",
						HostNodes: testutils.Ptr("1"),
						Memory:    testutils.Ptr(4096),
						Policy:    testutils.Ptr("bind"),
					},
				},
			}),
		},
		{
			name: "zero values should not set fields",
			vmConfig: &api.VirtualMachineConfig{
				CPU:      "",
				Cores:    0,
				Sockets:  0,
				CPULimit: 0,
				CPUUnits: 0,
				Vcpus:    0,
				Numa:     0,
			},
			expected: &proxmox.CPU{},
		},
		{
			name: "NUMA disabled (0) should not set numa field",
			vmConfig: &api.VirtualMachineConfig{
				Numa: 0,
			},
			expected: &proxmox.CPU{},
		},
		{
			name: "empty NUMA node strings are skipped",
			vmConfig: &api.VirtualMachineConfig{
				Numa:  1,
				Numa0: "",
				Numa1: "cpus=0-3",
				Numa2: "",
			},
			expected: cpuWith("", map[string]interface{}{
				"numa": true,
				"numa-nodes": []proxmox.NumaNode{
					{Cpus: "0-3"},
				},
			}),
		},
		{
			name: "all NUMA node slots (0-9)",
			vmConfig: &api.VirtualMachineConfig{
				Numa:  1,
				Numa0: "cpus=0",
				Numa1: "cpus=1",
				Numa2: "cpus=2",
				Numa3: "cpus=3",
				Numa4: "cpus=4",
				Numa5: "cpus=5",
				Numa6: "cpus=6",
				Numa7: "cpus=7",
				Numa8: "cpus=8",
				Numa9: "cpus=9",
			},
			expected: cpuWith("", map[string]interface{}{
				"numa": true,
				"numa-nodes": []proxmox.NumaNode{
					{Cpus: "0"},
					{Cpus: "1"},
					{Cpus: "2"},
					{Cpus: "3"},
					{Cpus: "4"},
					{Cpus: "5"},
					{Cpus: "6"},
					{Cpus: "7"},
					{Cpus: "8"},
					{Cpus: "9"},
				},
			}),
		},
		{
			name: "unusual CPU string - ParseCPU is lenient",
			vmConfig: &api.VirtualMachineConfig{
				CPU: "host,flags=invalid[syntax",
			},
			expected: cpuWith("host", map[string]interface{}{
				"flags+": []string{"invalid[syntax"},
			}),
		},
		{
			name: "invalid NUMA node string",
			vmConfig: &api.VirtualMachineConfig{
				Numa:  1,
				Numa0: "memory=invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a minimal VirtualMachine to test CPU parsing
			testVM := &api.VirtualMachine{
				VirtualMachineConfig: tt.vmConfig,
			}

			// ConvertVMConfigToInputs calls parseCPUFromVMConfig internally
			inputs, err := adapters.ConvertVMConfigToInputs(testVM, nil)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Compare CPU fields
			if tt.expected.Type != nil {
				require.NotNil(t, inputs.CPU, "CPU should not be nil when Type is expected")
				assert.Equal(t, *tt.expected.Type, *inputs.CPU.Type)
			}

			if inputs.CPU != nil {
				assert.Equal(t, tt.expected.Cores, inputs.CPU.Cores)
				assert.Equal(t, tt.expected.Sockets, inputs.CPU.Sockets)
				assert.Equal(t, tt.expected.Limit, inputs.CPU.Limit)
				assert.Equal(t, tt.expected.Units, inputs.CPU.Units)
				assert.Equal(t, tt.expected.Vcpus, inputs.CPU.Vcpus)
				assert.Equal(t, tt.expected.Numa, inputs.CPU.Numa)
				assert.Equal(t, tt.expected.FlagsEnabled, inputs.CPU.FlagsEnabled)
				assert.Equal(t, tt.expected.FlagsDisabled, inputs.CPU.FlagsDisabled)
				assert.Equal(t, tt.expected.Hidden, inputs.CPU.Hidden)
				assert.Equal(t, tt.expected.HVVendorID, inputs.CPU.HVVendorID)
				assert.Equal(t, tt.expected.PhysBits, inputs.CPU.PhysBits)
				assert.Equal(t, tt.expected.NumaNodes, inputs.CPU.NumaNodes)
			}
		})
	}
}

// TestCPUAnnotate verifies that CPU.Annotate method exists and has the correct signature
func TestBuildOptionsDiskOrdering(t *testing.T) {
	t.Parallel()

	// Test case with multiple disks in specific order
	testCases := []struct {
		name          string
		disks         []*proxmox.Disk
		expectedOrder []string
		description   string
	}{
		{
			name: "SATA and IDE disks mixed order",
			disks: []*proxmox.Disk{
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
			disks: []*proxmox.Disk{
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
			disks: []*proxmox.Disk{
				createTestDisk("virtio0", "local-lvm", 100),
				createTestDisk("virtio1", "local-lvm", 50),
				createTestDisk("virtio2", "local-lvm", 200),
			},
			expectedOrder: []string{"virtio0", "virtio1", "virtio2"},
			description:   "VirtIO disks should maintain input order regardless of size",
		},
		{
			name: "Single disk",
			disks: []*proxmox.Disk{
				createTestDisk("sata0", "local-lvm", 20),
			},
			expectedOrder: []string{"sata0"},
			description:   "Single disk should be handled correctly",
		},
		{
			name: "All interface types mixed",
			disks: []*proxmox.Disk{
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
			disks: []*proxmox.Disk{
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
			inputs := proxmox.VMInputs{
				Disks: tc.disks,
			}
			vmID := 100

			// Build options
			options := adapters.BuildVMOptions(inputs, vmID)

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
				expectedKey, expectedValue := adapters.ToProxmoxDiskKeyConfig(*inputDisk)
				assert.Equal(t, expectedInterface, expectedKey, "Disk interface should match")
				assert.Equal(t, expectedValue, actualValue, "Disk configuration should match")
			}
		})
	}
}

// TestBuildOptionsConsistentOrdering verifies that multiple calls to BuildOptions
func TestBuildOptionsConsistentOrdering(t *testing.T) {
	t.Parallel()

	// Create a complex disk configuration
	disks := []*proxmox.Disk{
		createTestDisk("virtio2", "local-lvm", 40),
		createTestDisk("scsi0", "local-lvm", 30),
		createTestDisk("ide1", "local-lvm", 20),
		createTestDisk("sata5", "local-lvm", 50),
		createTestCDROMDisk("ide2", "none"),
		createTestDisk("virtio0", "local-lvm", 60),
	}

	inputs := proxmox.VMInputs{
		Disks: disks,
	}
	vmID := 200

	// Build options multiple times
	const numIterations = 10
	var allOrders [][]string

	for i := 0; i < numIterations; i++ {
		options := adapters.BuildVMOptions(inputs, vmID)

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
		expectedInterfaces[i], _ = adapters.ToProxmoxDiskKeyConfig(*disk)
	}
	assert.Equal(t, expectedInterfaces, expectedOrder,
		"Consistent order should match input disk order")
}

// TestBuildOptionsEmptyDisks verifies behavior with no disks
func TestBuildOptionsEmptyDisks(t *testing.T) {
	t.Parallel()

	inputs := proxmox.VMInputs{
		Disks: nil, // No disks
	}
	vmID := 300

	options := adapters.BuildVMOptions(inputs, vmID)

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
		disk          *proxmox.Disk
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
			disk: &proxmox.Disk{
				Interface: "virtio1",
				DiskBase: proxmox.DiskBase{
					Storage: "local-lvm",
					FileID:  testutils.Ptr("vm-100-disk-1"),
				},
				Size: 64,
			},
			expectedKey:   "virtio1",
			expectedValue: "file=local-lvm:vm-100-disk-1,size=64",
		},
		{
			name: "CD-ROM disk",
			disk: &proxmox.Disk{
				Interface: "ide2",
				DiskBase: proxmox.DiskBase{
					Storage: "none",
				},
				Size: 0,
			},
			expectedKey:   "ide2",
			expectedValue: "file=none:0,size=0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualKey, actualValue := adapters.ToProxmoxDiskKeyConfig(*tc.disk)
			assert.Equal(t, tc.expectedKey, actualKey, "Disk key should match expected")
			assert.Equal(t, tc.expectedValue, actualValue, "Disk configuration should match expected")
		})
	}
}

// Helper functions
func TestVMCreateDiskOrderingIntegration(t *testing.T) {
	t.Parallel()

	// Test that simulates how Create function processes disk ordering
	testCases := []struct {
		name          string
		disks         []*proxmox.Disk
		expectedOrder []string
		description   string
	}{
		{
			name: "Mixed interface types during create",
			disks: []*proxmox.Disk{
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
			disks: []*proxmox.Disk{
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
			inputs := proxmox.VMInputs{
				Name:  name,
				VMID:  &vmid,
				Disks: tc.disks,
			}

			// Call BuildOptions just like the Create function does (line 90 in vm.go)
			options := adapters.BuildVMOptions(inputs, *inputs.VMID)

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
				options2 := adapters.BuildVMOptions(inputs, *inputs.VMID)
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
func TestVMCreateDiskOptionsConsistency(t *testing.T) {
	t.Parallel()

	// Complex disk configuration similar to real-world scenarios
	disks := []*proxmox.Disk{
		createTestDisk("virtio0", "local-lvm", 32),     // Primary disk
		createTestDisk("virtio1", "local-ssd", 64),     // Secondary SSD
		createTestDisk("scsi0", "local-hdd", 500),      // Large storage
		createTestCDROMDisk("ide2", "none"),            // CD-ROM
		createTestDisk("sata0", "backup-storage", 100), // Backup disk
	}

	name := "test-vm-consistency"
	vmid := 200
	inputs := proxmox.VMInputs{
		Name:  name,
		VMID:  &vmid,
		Disks: disks,
	}

	// Multiple calls should produce identical ordering
	const iterations = 50
	var allOrders [][]string

	for i := 0; i < iterations; i++ {
		options := adapters.BuildVMOptions(inputs, *inputs.VMID)
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
			disks := make([]*proxmox.Disk, bm.diskCount)
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
			inputs := proxmox.VMInputs{
				Name:  name,
				VMID:  &vmid,
				Disks: disks,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = adapters.BuildVMOptions(inputs, *inputs.VMID)
			}
		})
	}
}

// BenchmarkBuildOptionsConsistency benchmarks multiple BuildOptions calls for consistency
func BenchmarkBuildOptionsConsistency(b *testing.B) {
	// Create a realistic disk configuration
	disks := []*proxmox.Disk{
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
	inputs := proxmox.VMInputs{
		Name:  name,
		VMID:  &vmid,
		Disks: disks,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		options := adapters.BuildVMOptions(inputs, *inputs.VMID)

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
func TestVMCreateDiskOrderingEndToEnd(t *testing.T) {
	t.Parallel()

	// This test simulates a real-world VM configuration that would be used
	// in production environments to ensure disk ordering is preserved correctly
	testCase := struct {
		name        string
		description string
		disks       []*proxmox.Disk
		expected    []string
	}{
		name:        "Production VM with complex disk setup",
		description: "Boot disk, data disks, CD-ROM, and backup storage in specific order",
		disks: []*proxmox.Disk{
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

		// Create VM inputs as they would be in real usage
		name := "production-vm"
		vmid := 500
		inputs := proxmox.VMInputs{
			Name:  name,
			VMID:  &vmid,
			Disks: testCase.disks,
		}

		t.Logf("Test case: %s", testCase.description)
		t.Logf("Input disk order: %v", testCase.expected)

		// Call BuildOptions multiple times to verify deterministic ordering.
		// This simulates what would happen during VM creation, updates, etc.
		const numCalls = 10
		var allOrders [][]string

		for i := 0; i < numCalls; i++ {
			options := adapters.BuildVMOptions(inputs, *inputs.VMID)

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

		// Verify all calls produce identical ordering.
		firstOrder := allOrders[0]
		for i := 1; i < numCalls; i++ {
			assert.Equal(t, firstOrder, allOrders[i],
				"BuildOptions call %d should produce consistent ordering", i)
		}

		// Verify the order matches the expected input order.
		assert.Equal(t, testCase.expected, firstOrder,
			"Disk ordering should exactly match input order")

		// Verify each disk configuration string matches the input disk.
		options := adapters.BuildVMOptions(inputs, *inputs.VMID)
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
			expectedKey, expectedValue := adapters.ToProxmoxDiskKeyConfig(*inputDisk)
			assert.Equal(t, expectedInterface, expectedKey,
				"Disk %d interface should match", i)
			assert.Equal(t, expectedValue, actualValue,
				"Disk %d configuration should match", i)
		}

		// Ensure ordering logic doesn't introduce measurable overhead at scale.
		start := time.Now()
		for i := 0; i < 1000; i++ {
			_ = adapters.BuildVMOptions(inputs, *inputs.VMID)
		}
		duration := time.Since(start)

		t.Logf("1000 BuildOptions calls took %v (avg: %v per call)",
			duration, duration/1000)

		// Should complete 1000 calls in under 100ms (very generous threshold)
		assert.Less(t, duration.Milliseconds(), int64(100),
			"Disk ordering should not significantly impact performance")
	})
}

func TestVMCreateDiskOrderPreservation(t *testing.T) {
	t.Parallel()

	// Test cases with various disk ordering scenarios
	testCases := []struct {
		name                string
		inputDisks          []*proxmox.Disk
		expectedDiskOrder   []string
		description         string
		shouldPreserveOrder bool
	}{
		{
			name: "Mixed interface types preserve order",
			inputDisks: []*proxmox.Disk{
				{
					Interface: "virtio0",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 32,
				},
				{
					Interface: "scsi1",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 64,
				},
				{
					Interface: "ide0",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 8,
				},
				{
					Interface: "sata2",
					DiskBase: proxmox.DiskBase{
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
			inputDisks: []*proxmox.Disk{
				{
					Interface: "scsi0",
					DiskBase: proxmox.DiskBase{
						Storage: ssdStorage,
					},
					Size: 32,
				}, // Boot disk
				{
					Interface: "scsi1",
					DiskBase: proxmox.DiskBase{
						Storage: ssdStorage,
					},
					Size: 100,
				}, // Data disk
				{
					Interface: "scsi2",
					DiskBase: proxmox.DiskBase{
						Storage: hddStorage,
					},
					Size: 500,
				}, // Backup storage
				{
					Interface: "ide2",
					DiskBase: proxmox.DiskBase{
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
			inputDisks: []*proxmox.Disk{
				{
					Interface: "scsi0",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 20,
				},
				{
					Interface: "scsi3",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 40,
				}, // Gap at scsi1, scsi2
				{
					Interface: "virtio1",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 30,
				},
				{
					Interface: "scsi1",
					DiskBase: proxmox.DiskBase{
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
			inputDisks: []*proxmox.Disk{
				{
					Interface: "virtio0",
					DiskBase: proxmox.DiskBase{
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
			inputs := proxmox.VMInputs{
				Name:  name,
				VMID:  &vmid,
				Disks: tc.inputDisks,
			}

			// Test BuildVMOptions which is called during VM creation
			options := adapters.BuildVMOptions(inputs, *inputs.VMID)

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
					expectedKey, expectedConfig := adapters.ToProxmoxDiskKeyConfig(*inputDisk)

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
					options2 := adapters.BuildVMOptions(inputs, *inputs.VMID)
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
		orderedDisks := []*proxmox.Disk{
			{
				Interface: "virtio0",
				DiskBase: proxmox.DiskBase{
					Storage: ssdStorage,
				},
				Size: 32,
			},
			{
				Interface: "scsi1",
				DiskBase: proxmox.DiskBase{
					Storage: lvmStorage,
				},
				Size: 64,
			},
			{
				Interface: "ide2",
				DiskBase: proxmox.DiskBase{
					Storage: "none",
				},
				Size: 0,
			},
			{
				Interface: "sata0",
				DiskBase: proxmox.DiskBase{
					Storage: hddStorage,
				},
				Size: 128,
			},
		}

		name := "build-options-test-vm"
		vmid := 200
		inputs := proxmox.VMInputs{
			Name:  name,
			VMID:  &vmid,
			Disks: orderedDisks,
		}

		// Call BuildVMOptions multiple times to ensure consistency
		const iterations = 10
		var allOrders [][]string

		for i := 0; i < iterations; i++ {
			options := adapters.BuildVMOptions(inputs, *inputs.VMID)

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
			disks    []*proxmox.Disk
			expected []string
		}{
			{
				name: "reverse_numerical_order",
				disks: []*proxmox.Disk{
					{
						Interface: "scsi3",
						DiskBase: proxmox.DiskBase{
							Storage: lvmStorage,
						},
						Size: 30,
					},
					{
						Interface: "scsi1",
						DiskBase: proxmox.DiskBase{
							Storage: lvmStorage,
						},
						Size: 20,
					},
					{
						Interface: "scsi0",
						DiskBase: proxmox.DiskBase{
							Storage: lvmStorage,
						},
						Size: 10,
					},
				},
				expected: []string{"scsi3", "scsi1", "scsi0"},
			},
			{
				name: "mixed_types_non_alphabetical",
				disks: []*proxmox.Disk{
					{
						Interface: "virtio5",
						DiskBase: proxmox.DiskBase{
							Storage: lvmStorage,
						},
						Size: 50,
					},
					{
						Interface: "ide0",
						DiskBase: proxmox.DiskBase{
							Storage: lvmStorage,
						},
						Size: 5,
					},
					{
						Interface: "scsi2",
						DiskBase: proxmox.DiskBase{
							Storage: lvmStorage,
						},
						Size: 25,
					},
					{
						Interface: "sata10",
						DiskBase: proxmox.DiskBase{
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
				inputs := proxmox.VMInputs{
					VMID:  &vmid,
					Disks: scenario.disks,
				}

				options := adapters.BuildVMOptions(inputs, *inputs.VMID)

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
		currentInputDisks []*proxmox.Disk
		vmConfigDisks     map[string]string // Interface -> Config
		expectedDiskOrder []string
		description       string
	}{
		{
			name: "Preserve existing disk order during read",
			currentInputDisks: []*proxmox.Disk{
				{
					Interface: "virtio0",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 32,
				},
				{
					Interface: "scsi1",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 64,
				},
				{
					Interface: "ide2",
					DiskBase: proxmox.DiskBase{
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
			currentInputDisks: []*proxmox.Disk{
				{
					Interface: "scsi0",
					DiskBase: proxmox.DiskBase{
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
			currentInputDisks: []*proxmox.Disk{
				{
					Interface: "scsi0",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 32,
				},
				{
					Interface: "scsi1",
					DiskBase: proxmox.DiskBase{
						Storage: lvmStorage,
					},
					Size: 64,
				},
				{
					Interface: "scsi2",
					DiskBase: proxmox.DiskBase{
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
			currentInput := proxmox.VMInputs{
				Disks: tc.currentInputDisks,
			}

			// Call ConvertVMConfigToInputs with current input to preserve order
			result, err := adapters.ConvertVMConfigToInputs(mockVM, currentInput.Disks)
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

func TestCPUToProxmoxString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       *proxmox.CPU
		expected    string
		description string
	}{
		{
			name:     "nil CPU returns empty string",
			input:    nil,
			expected: "",
		},
		{
			name: "minimal config - only type",
			input: &proxmox.CPU{
				Type: testutils.Ptr("host"),
			},
			expected: "host",
		},
		{
			name: "CPU type kvm64",
			input: &proxmox.CPU{
				Type: testutils.Ptr("kvm64"),
			},
			expected: "kvm64",
		},
		{
			name: "CPU with single enabled flag",
			input: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expected: "host,flags=+aes",
		},
		{
			name: "CPU with single disabled flag",
			input: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsDisabled: []string{"pcid"},
			},
			expected: "host,flags=-pcid",
		},
		{
			name: "CPU with mixed enabled and disabled flags",
			input: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes", "avx2"},
				FlagsDisabled: []string{"pcid"},
			},
			expected: "host,flags=+aes;+avx2;-pcid",
		},
		{
			name: "CPU with hidden true",
			input: &proxmox.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(true),
			},
			expected: "host,hidden=1",
		},
		{
			name: "CPU with hidden false",
			input: &proxmox.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(false),
			},
			expected: "host,hidden=0",
		},
		{
			name: "CPU with hv-vendor-id",
			input: &proxmox.CPU{
				Type:       testutils.Ptr("host"),
				HVVendorID: testutils.Ptr("AuthenticAMD"),
			},
			expected: "host,hv-vendor-id=AuthenticAMD",
		},
		{
			name: "CPU with phys-bits numeric",
			input: &proxmox.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("40"),
			},
			expected: "host,phys-bits=40",
		},
		{
			name: "CPU with phys-bits host",
			input: &proxmox.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("host"),
			},
			expected: "host,phys-bits=host",
		},
		{
			name: "comprehensive CPU config",
			input: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes", "avx2"},
				FlagsDisabled: []string{"pcid"},
				Hidden:        testutils.Ptr(true),
				HVVendorID:    testutils.Ptr("GenuineIntel"),
				PhysBits:      testutils.Ptr("42"),
			},
			expected: "host,flags=+aes;+avx2;-pcid,hidden=1,hv-vendor-id=GenuineIntel,phys-bits=42",
		},
		{
			name: "empty type pointer",
			input: &proxmox.CPU{
				Type: testutils.Ptr(""),
			},
			expected: "",
		},
		{
			name: "CPU without type but with flags",
			input: &proxmox.CPU{
				FlagsEnabled: []string{"aes"},
			},
			expected: "flags=+aes",
		},
		{
			name: "empty flags arrays",
			input: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{},
				FlagsDisabled: []string{},
			},
			expected: "host",
		},
		{
			name: "flags with empty strings filtered",
			input: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes", "", "avx2"},
			},
			expected: "host,flags=+aes;+avx2",
		},
		{
			name: "flags with only empty strings - all filtered out",
			input: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"", ""},
				FlagsDisabled: []string{""},
			},
			expected: "host",
		},
		{
			name: "only disabled flags",
			input: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsDisabled: []string{"pcid", "spec-ctrl"},
			},
			expected: "host,flags=-pcid;-spec-ctrl",
		},
		{
			name: "all optional fields nil",
			input: &proxmox.CPU{
				Type:          testutils.Ptr("kvm64"),
				FlagsEnabled:  nil,
				FlagsDisabled: nil,
				Hidden:        nil,
				HVVendorID:    nil,
				PhysBits:      nil,
			},
			expected: "kvm64",
		},
		{
			name: "complex type with dashes",
			input: &proxmox.CPU{
				Type: testutils.Ptr("x86-64-v2-AES"),
			},
			expected: "x86-64-v2-AES",
		},
		{
			name: "multiple flags with special characters",
			input: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes", "spec-ctrl", "ibpb"},
			},
			expected: "host,flags=+aes;+spec-ctrl;+ibpb",
		},
		{
			name: "hidden false with other fields",
			input: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
				Hidden:       testutils.Ptr(false),
			},
			expected: "host,flags=+aes,hidden=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := adapters.CPUToProxmoxString(tt.input)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestNumaNodeToProxmoxString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       *proxmox.NumaNode
		expected    string
		description string
	}{
		{
			name: "minimal config - only cpus",
			input: &proxmox.NumaNode{
				Cpus: "0-1",
			},
			expected: "cpus=0-1",
		},
		{
			name: "complete NUMA config",
			input: &proxmox.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0"),
				Memory:    testutils.Ptr(2048),
				Policy:    testutils.Ptr("bind"),
			},
			expected: "cpus=0-3,hostnodes=0,memory=2048,policy=bind",
		},
		{
			name: "cpus with hostnodes",
			input: &proxmox.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0-1"),
			},
			expected: "cpus=0-3,hostnodes=0-1",
		},
		{
			name: "cpus with memory",
			input: &proxmox.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(4096),
			},
			expected: "cpus=0-3,memory=4096",
		},
		{
			name: "cpus with policy",
			input: &proxmox.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("preferred"),
			},
			expected: "cpus=0-3,policy=preferred",
		},
		{
			name: "single CPU core",
			input: &proxmox.NumaNode{
				Cpus: "0",
			},
			expected: "cpus=0",
		},
		{
			name: "complex CPU range",
			input: &proxmox.NumaNode{
				Cpus: "0-3;5-7",
			},
			expected: "cpus=0-3;5-7",
		},
		{
			name: "multiple host nodes range",
			input: &proxmox.NumaNode{
				Cpus:      "0-7",
				HostNodes: testutils.Ptr("0-2"),
			},
			expected: "cpus=0-7,hostnodes=0-2",
		},
		{
			name: "policy interleave",
			input: &proxmox.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("interleave"),
			},
			expected: "cpus=0-3,policy=interleave",
		},
		{
			name: "policy preferred",
			input: &proxmox.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("preferred"),
			},
			expected: "cpus=0-3,policy=preferred",
		},
		{
			name: "large memory value",
			input: &proxmox.NumaNode{
				Cpus:   "0-15",
				Memory: testutils.Ptr(65536),
			},
			expected: "cpus=0-15,memory=65536",
		},
		{
			name: "small memory value",
			input: &proxmox.NumaNode{
				Cpus:   "0-1",
				Memory: testutils.Ptr(512),
			},
			expected: "cpus=0-1,memory=512",
		},
		{
			name: "zero memory value",
			input: &proxmox.NumaNode{
				Cpus:   "0-1",
				Memory: testutils.Ptr(0),
			},
			expected: "cpus=0-1,memory=0",
		},
		{
			name: "hostnodes and memory",
			input: &proxmox.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0"),
				Memory:    testutils.Ptr(2048),
			},
			expected: "cpus=0-3,hostnodes=0,memory=2048",
		},
		{
			name: "hostnodes and policy",
			input: &proxmox.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0-1"),
				Policy:    testutils.Ptr("bind"),
			},
			expected: "cpus=0-3,hostnodes=0-1,policy=bind",
		},
		{
			name: "memory and policy",
			input: &proxmox.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(4096),
				Policy: testutils.Ptr("interleave"),
			},
			expected: "cpus=0-3,memory=4096,policy=interleave",
		},
		{
			name: "all optional fields nil",
			input: &proxmox.NumaNode{
				Cpus:      "0-7",
				HostNodes: nil,
				Memory:    nil,
				Policy:    nil,
			},
			expected: "cpus=0-7",
		},
		{
			name: "empty cpus string",
			input: &proxmox.NumaNode{
				Cpus: "",
			},
			expected: "cpus=",
		},
		{
			name: "cpus with special formats",
			input: &proxmox.NumaNode{
				Cpus: "0,2,4,6",
			},
			expected: "cpus=0,2,4,6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := adapters.ToProxmoxNumaString(*tt.input)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestVMAdapterCreateValidation(t *testing.T) {
	t.Parallel()

	vmID := 100
	nodeName := "pve-node"

	tests := []struct {
		name           string
		inputs         proxmox.VMInputs
		wantErrContain string
	}{
		{
			name:           "returns error when inputs.Node is nil",
			inputs:         proxmox.VMInputs{VMID: &vmID},
			wantErrContain: "inputs.Node",
		},
		{
			name:           "returns error when inputs.VMID is nil",
			inputs:         proxmox.VMInputs{Node: &nodeName},
			wantErrContain: "inputs.VMID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := adapters.NewVMAdapter(adapters.NewProxmoxAdapter(nil)).CreateVM(context.Background(), tt.inputs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrContain)
		})
	}
}

func TestBuildVMOptionsTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tags        []string
		wantTagsVal *string
	}{
		{
			name:        "nil tags produces empty tags option",
			tags:        nil,
			wantTagsVal: nil,
		},
		{
			name:        "empty slice produces empty tags option",
			tags:        []string{},
			wantTagsVal: nil,
		},
		{
			name:        "single tag",
			tags:        []string{"prod"},
			wantTagsVal: testutils.Ptr("prod"),
		},
		{
			name:        "multiple tags joined by semicolon",
			tags:        []string{"prod", "web", "frontend"},
			wantTagsVal: testutils.Ptr("prod;web;frontend"),
		},
		{
			name:        "tag order is preserved",
			tags:        []string{"z-last", "a-first", "m-middle"},
			wantTagsVal: testutils.Ptr("z-last;a-first;m-middle"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputs := proxmox.VMInputs{Tags: tt.tags}
			options := adapters.BuildVMOptions(inputs, 100)

			var tagsOpt *api.VirtualMachineOption
			for i := range options {
				if options[i].Name == "tags" {
					tagsOpt = &options[i]
					break
				}
			}

			if tt.wantTagsVal == nil {
				assert.Nil(t, tagsOpt, "expected no 'tags' option when tags is nil")
				return
			}

			require.NotNil(t, tagsOpt, "expected a 'tags' option to be present")
			gotVal, ok := tagsOpt.Value.(*string)
			require.True(t, ok, "tags value should be a *string")
			assert.Equal(t, tt.wantTagsVal, gotVal)
		})
	}
}

func TestBuildVMOptionsDiffTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		newTags     []string
		currentTags []string
		wantChanged bool
		wantTagsVal string
	}{
		{
			name:        "no change - both nil",
			newTags:     nil,
			currentTags: nil,
			wantChanged: false,
		},
		{
			name:        "no change - both empty",
			newTags:     []string{},
			currentTags: []string{},
			wantChanged: false,
		},
		{
			name:        "no change - same tags same order",
			newTags:     []string{"prod", "web"},
			currentTags: []string{"prod", "web"},
			wantChanged: false,
		},
		{
			name:        "changed - tag added",
			newTags:     []string{"prod", "web"},
			currentTags: []string{"prod"},
			wantChanged: true,
			wantTagsVal: "prod;web",
		},
		{
			name:        "changed - tag removed",
			newTags:     []string{"prod"},
			currentTags: []string{"prod", "web"},
			wantChanged: true,
			wantTagsVal: "prod",
		},
		{
			name:        "changed - tag replaced",
			newTags:     []string{"staging"},
			currentTags: []string{"prod"},
			wantChanged: true,
			wantTagsVal: "staging",
		},
		{
			name:        "no change - order differs",
			newTags:     []string{"web", "prod"},
			currentTags: []string{"prod", "web"},
			wantChanged: false,
		},
		{
			name:        "changed - from nil to tags",
			newTags:     []string{"prod"},
			currentTags: nil,
			wantChanged: true,
			wantTagsVal: "prod",
		},
		{
			name:        "changed - tags cleared",
			newTags:     nil,
			currentTags: []string{"prod"},
			wantChanged: true,
			wantTagsVal: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			newInputs := proxmox.VMInputs{Tags: tt.newTags}
			currentInputs := proxmox.VMInputs{Tags: tt.currentTags}
			options := adapters.BuildVMOptionsDiff(newInputs, 100, &currentInputs)

			var tagsOpt *api.VirtualMachineOption
			for i := range options {
				if options[i].Name == "tags" {
					tagsOpt = &options[i]
					break
				}
			}

			if !tt.wantChanged {
				assert.Nil(t, tagsOpt, "expected no 'tags' option when unchanged")
				return
			}

			require.NotNil(t, tagsOpt, "expected a 'tags' option when changed")
			gotVal, ok := tagsOpt.Value.(*string)
			require.True(t, ok, "tags value should be a *string")
			assert.Equal(t, tt.wantTagsVal, *gotVal)
		})
	}
}

func TestVMReadTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		tagsSlice []string
		wantTags  []string
	}{
		{
			name:      "nil tags slice returns nil tags",
			tagsSlice: nil,
			wantTags:  nil,
		},
		{
			name:      "empty slice normalizes to nil",
			tagsSlice: []string{},
			wantTags:  nil,
		},
		{
			name:      "single tag preserved",
			tagsSlice: []string{"prod"},
			wantTags:  []string{"prod"},
		},
		{
			name:      "multiple tags preserved in order",
			tagsSlice: []string{"prod", "web", "frontend"},
			wantTags:  []string{"prod", "web", "frontend"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := createMockVM(nil)
			vm.VirtualMachineConfig.TagsSlice = tt.tagsSlice

			result, err := adapters.ConvertVMConfigToInputs(vm, nil)
			require.NoError(t, err)

			assert.Equal(t, tt.wantTags, result.Tags)
		})
	}
}

// TestVMReadTagsWhitespaceFromAPI verifies that when Proxmox returns a whitespace-only
// Tags string (which happens for VMs created without tags), the resulting state has nil
// tags rather than a slice containing a whitespace element.
// Proxmox returns " " (a single space) for VMs with no tags; this must be normalised to nil.
func TestVMReadTagsWhitespaceFromAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tagsStr  string
		wantTags []string
	}{
		{
			name:     "single space from API returns nil tags",
			tagsStr:  " ",
			wantTags: nil,
		},
		{
			name:     "multiple spaces from API returns nil tags",
			tagsStr:  "   ",
			wantTags: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := createMockVM(nil)
			vm.VirtualMachineConfig.Tags = tt.tagsStr
			vm.VirtualMachineConfig.TagsSlice = nil

			result, err := adapters.ConvertVMConfigToInputs(vm, nil)
			require.NoError(t, err)

			assert.Equal(t, tt.wantTags, result.Tags,
				"whitespace-only Tags string from API should produce nil tags, not %v", result.Tags)
		})
	}
}

// TestBuildVMOptionsDiffDisks verifies that BuildVMOptionsDiff emits config options
// for disks that are new (absent from current state) and omits options for disks
// that already exist in current state (unchanged, resized, or removed disks are
// handled by direct API calls before UpdateConfig is invoked).
func TestBuildVMOptionsDiffDisks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputDisks     []*proxmox.Disk
		currentDisks   []*proxmox.Disk
		wantDiskKeys   []string // interfaces that should appear as options
		wantNoDiskKeys []string // interfaces that must NOT appear as options
	}{
		{
			name: "new disk emitted with storage:size format",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "local-lvm"}, Size: 30, Interface: "sata1"},
			},
			currentDisks:   nil,
			wantDiskKeys:   []string{"sata1"},
			wantNoDiskKeys: nil,
		},
		{
			name: "existing disk not re-emitted",
			inputDisks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("local-lvm:vm-100-disk-0")},
					Size:      20,
					Interface: "scsi0",
				},
			},
			currentDisks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("local-lvm:vm-100-disk-0")},
					Size:      20,
					Interface: "scsi0",
				},
			},
			wantDiskKeys:   nil,
			wantNoDiskKeys: []string{"scsi0"},
		},
		{
			name: "mixed: new disk emitted, existing disk omitted",
			inputDisks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: testutils.Ptr("ceph-ha:vm-116-disk-0")},
					Size:      20,
					Interface: "scsi0",
				},
				{DiskBase: proxmox.DiskBase{Storage: "ceph-ha"}, Size: 30, Interface: "sata1"},
			},
			currentDisks: []*proxmox.Disk{
				{
					DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: testutils.Ptr("ceph-ha:vm-116-disk-0")},
					Size:      20,
					Interface: "scsi0",
				},
			},
			wantDiskKeys:   []string{"sata1"},
			wantNoDiskKeys: []string{"scsi0"},
		},
		{
			name:           "no disks in either inputs or current",
			inputDisks:     nil,
			currentDisks:   nil,
			wantDiskKeys:   nil,
			wantNoDiskKeys: nil,
		},
		{
			name: "new disk config uses storage:size format when FileID is nil",
			inputDisks: []*proxmox.Disk{
				{DiskBase: proxmox.DiskBase{Storage: "ceph-ha"}, Size: 50, Interface: "virtio0"},
			},
			currentDisks: nil,
			wantDiskKeys: []string{"virtio0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputs := proxmox.VMInputs{Disks: tt.inputDisks}
			current := proxmox.VMInputs{Disks: tt.currentDisks}
			options := adapters.BuildVMOptionsDiff(inputs, 100, &current)

			// Index options by name for easy lookup.
			optByName := make(map[string]api.VirtualMachineOption, len(options))
			for _, opt := range options {
				optByName[opt.Name] = opt
			}

			for _, iface := range tt.wantDiskKeys {
				opt, ok := optByName[iface]
				require.Truef(t, ok, "expected option for disk interface %q to be present", iface)
				// Value must be a non-empty string in Proxmox disk config format.
				strVal, isStr := opt.Value.(string)
				require.Truef(t, isStr, "disk option value for %q should be a string", iface)
				assert.NotEmpty(t, strVal, "disk option value for %q should not be empty", iface)
			}

			for _, iface := range tt.wantNoDiskKeys {
				_, ok := optByName[iface]
				assert.Falsef(t, ok, "unexpected option for existing disk interface %q", iface)
			}
		})
	}
}

// TestToProxmoxDiskKeyConfigFlags verifies that boolean disk flag fields
// (cache, aio, discard, iothread, ssd, backup, replicate, ro) are correctly
// serialized into the Proxmox disk config string.
func TestToProxmoxDiskKeyConfigFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		disk    proxmox.Disk
		wantKey string
		wantCfg string
	}{
		{
			name: "cache=writeback",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      20,
				Cache:     testutils.Ptr("writeback"),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:20,size=20,cache=writeback",
		},
		{
			name: "aio=io_uring",
			disk: proxmox.Disk{
				Interface: "virtio0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      32,
				Aio:       testutils.Ptr("io_uring"),
			},
			wantKey: "virtio0",
			wantCfg: "file=local-lvm:32,size=32,aio=io_uring",
		},
		{
			name: "discard=on",
			disk: proxmox.Disk{
				Interface: "scsi1",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      50,
				Discard:   testutils.Ptr("on"),
			},
			wantKey: "scsi1",
			wantCfg: "file=local-lvm:50,size=50,discard=on",
		},
		{
			name: "iothread=true",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				IOThread:  testutils.Ptr(true),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,iothread=1",
		},
		{
			name: "ssd=true",
			disk: proxmox.Disk{
				Interface: "sata0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				SSD:       testutils.Ptr(true),
			},
			wantKey: "sata0",
			wantCfg: "file=local-lvm:10,size=10,ssd=1",
		},
		{
			name: "backup=false",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				Backup:    testutils.Ptr(false),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,backup=0",
		},
		{
			name: "replicate=false",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				Replicate: testutils.Ptr(false),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,replicate=0",
		},
		{
			name: "ro=true",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				ReadOnly:  testutils.Ptr(true),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,ro=1",
		},
		{
			name: "multiple flags combined",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:      20,
				Cache:     testutils.Ptr("writeback"),
				Discard:   testutils.Ptr("on"),
				IOThread:  testutils.Ptr(true),
				Backup:    testutils.Ptr(false),
			},
			wantKey: "scsi0",
			wantCfg: "file=ceph-ha:vm-100-disk-0,size=20,cache=writeback,discard=on,iothread=1,backup=0",
		},
		{
			name: "nil flags produce no extra tokens",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotKey, gotCfg := adapters.ToProxmoxDiskKeyConfig(tt.disk)
			require.Equal(t, tt.wantKey, gotKey)
			require.Equal(t, tt.wantCfg, gotCfg)
		})
	}
}

// TestParseDiskConfigFlags verifies that boolean disk flag fields
// (cache, aio, discard, iothread, ssd, backup, replicate, ro) are correctly
// deserialized from Proxmox disk config strings.
func TestParseDiskConfigFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		diskConfig string
		want       proxmox.Disk
	}{
		{
			name:       "cache=writeback",
			diskConfig: "local-lvm:vm-100-disk-0,size=20G,cache=writeback",
			want: proxmox.Disk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:     20,
				Cache:    testutils.Ptr("writeback"),
			},
		},
		{
			name:       "aio=io_uring",
			diskConfig: "local-lvm:vm-100-disk-0,size=32G,aio=io_uring",
			want: proxmox.Disk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:     32,
				Aio:      testutils.Ptr("io_uring"),
			},
		},
		{
			name:       "discard=on",
			diskConfig: "local-lvm:vm-100-disk-0,size=50G,discard=on",
			want: proxmox.Disk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:     50,
				Discard:  testutils.Ptr("on"),
			},
		},
		{
			name:       "iothread=1",
			diskConfig: "local-lvm:vm-100-disk-0,size=10G,iothread=1",
			want: proxmox.Disk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:     10,
				IOThread: testutils.Ptr(true),
			},
		},
		{
			name:       "ssd=1",
			diskConfig: "local-lvm:vm-100-disk-0,size=10G,ssd=1",
			want: proxmox.Disk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:     10,
				SSD:      testutils.Ptr(true),
			},
		},
		{
			name:       "backup=0",
			diskConfig: "local-lvm:vm-100-disk-0,size=10G,backup=0",
			want: proxmox.Disk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:     10,
				Backup:   testutils.Ptr(false),
			},
		},
		{
			name:       "replicate=0",
			diskConfig: "local-lvm:vm-100-disk-0,size=10G,replicate=0",
			want: proxmox.Disk{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:      10,
				Replicate: testutils.Ptr(false),
			},
		},
		{
			name:       "ro=1",
			diskConfig: "local-lvm:vm-100-disk-0,size=10G,ro=1",
			want: proxmox.Disk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:     10,
				ReadOnly: testutils.Ptr(true),
			},
		},
		{
			name:       "multiple flags combined",
			diskConfig: "ceph-ha:vm-116-disk-0,size=20G,cache=writeback,discard=on,iothread=1,backup=0",
			want: proxmox.Disk{
				DiskBase: proxmox.DiskBase{Storage: "ceph-ha", FileID: testutils.Ptr("vm-116-disk-0")},
				Size:     20,
				Cache:    testutils.Ptr("writeback"),
				Discard:  testutils.Ptr("on"),
				IOThread: testutils.Ptr(true),
				Backup:   testutils.Ptr(false),
			},
		},
		{
			name:       "no flags yields nil fields",
			diskConfig: "local-lvm:vm-100-disk-0,size=20G",
			want: proxmox.Disk{
				DiskBase: proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:     20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got proxmox.Disk
			require.NoError(t, adapters.ParseDiskConfig(&got, tt.diskConfig))
			require.Equal(t, tt.want, got)
		})
	}
}

// TestBuildVMOptionsDiffDiskFlagsChanged verifies that BuildVMOptionsDiff re-emits
// the disk config when flag fields change on an existing disk, and does not emit
// anything when flags are unchanged.
func TestBuildVMOptionsDiffDiskFlagsChanged(t *testing.T) {
	t.Parallel()

	fileID := testutils.Ptr("ceph-ha:vm-100-disk-0")

	tests := []struct {
		name        string
		inputDisk   proxmox.Disk
		currentDisk proxmox.Disk
		wantEmit    bool   // whether a config option should be emitted for scsi0
		wantContain string // substring the emitted config must contain (when wantEmit=true)
	}{
		{
			name: "cache added to existing disk",
			inputDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
				Cache:     testutils.Ptr("writeback"),
			},
			currentDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
			},
			wantEmit:    true,
			wantContain: "cache=writeback",
		},
		{
			name: "discard enabled on existing disk",
			inputDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
				Discard:   testutils.Ptr("on"),
			},
			currentDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
			},
			wantEmit:    true,
			wantContain: "discard=on",
		},
		{
			name: "backup disabled on existing disk",
			inputDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
				Backup:    testutils.Ptr(false),
			},
			currentDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
			},
			wantEmit:    true,
			wantContain: "backup=0",
		},
		{
			name: "no flag change — not re-emitted",
			inputDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
				Cache:     testutils.Ptr("writeback"),
			},
			currentDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
				Cache:     testutils.Ptr("writeback"),
			},
			wantEmit: false,
		},
		{
			name: "iothread added to existing disk",
			inputDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
				IOThread:  testutils.Ptr(true),
			},
			currentDisk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha", FileID: fileID},
				Size:      20,
			},
			wantEmit:    true,
			wantContain: "iothread=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputs := proxmox.VMInputs{Disks: []*proxmox.Disk{&tt.inputDisk}}
			current := proxmox.VMInputs{Disks: []*proxmox.Disk{&tt.currentDisk}}
			options := adapters.BuildVMOptionsDiff(inputs, 100, &current)

			var diskOpt *api.VirtualMachineOption
			for i := range options {
				if options[i].Name == "scsi0" {
					diskOpt = &options[i]
					break
				}
			}

			if tt.wantEmit {
				require.NotNil(t, diskOpt, "expected a config option for scsi0 to be emitted")
				strVal, ok := diskOpt.Value.(string)
				require.True(t, ok)
				require.Contains(t, strVal, tt.wantContain)
			} else {
				require.Nil(t, diskOpt, "expected no config option for scsi0 (unchanged)")
			}
		})
	}
}

func TestToProxmoxDiskKeyConfigBandwidth(t *testing.T) {
	t.Parallel()

	f64 := func(v float64) *float64 { return &v }
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name     string
		bw       *proxmox.DiskBandwidth
		wantKeys []string
		wantNot  []string
	}{
		{
			name:    "nil bandwidth emits nothing",
			bw:      nil,
			wantNot: []string{"mbps_rd", "mbps_wr", "iops_rd", "iops_wr"},
		},
		{
			name: "all fields set",
			bw: &proxmox.DiskBandwidth{
				MBpsRd:    f64(100.5),
				MBpsRdMax: f64(200),
				MBpsWr:    f64(50),
				MBpsWrMax: f64(75.25),
				IOPSRd:    intPtr(1000),
				IOPSRdMax: intPtr(2000),
				IOPSWr:    intPtr(500),
				IOPSWrMax: intPtr(750),
			},
			wantKeys: []string{
				"mbps_rd=100.5", "mbps_rd_max=200", "mbps_wr=50", "mbps_wr_max=75.25",
				"iops_rd=1000", "iops_rd_max=2000", "iops_wr=500", "iops_wr_max=750",
			},
		},
		{
			name: "only mbps read fields",
			bw: &proxmox.DiskBandwidth{
				MBpsRd:    f64(100),
				MBpsRdMax: f64(150),
			},
			wantKeys: []string{"mbps_rd=100", "mbps_rd_max=150"},
			wantNot:  []string{"mbps_wr", "iops_rd", "iops_wr"},
		},
		{
			name: "only iops write fields",
			bw: &proxmox.DiskBandwidth{
				IOPSWr:    intPtr(200),
				IOPSWrMax: intPtr(400),
			},
			wantKeys: []string{"iops_wr=200", "iops_wr_max=400"},
			wantNot:  []string{"mbps_rd", "mbps_wr", "iops_rd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			disk := proxmox.Disk{
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-100-disk-0")},
				Size:      10,
				Interface: "scsi0",
				Bandwidth: tt.bw,
			}
			_, config := adapters.ToProxmoxDiskKeyConfig(disk)
			for _, want := range tt.wantKeys {
				require.Contains(t, config, want)
			}
			for _, notWant := range tt.wantNot {
				require.NotContains(t, config, notWant)
			}
		})
	}
}

func TestParseDiskConfigBandwidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
		wantBW *proxmox.DiskBandwidth
	}{
		{
			name:   "no bandwidth fields → nil Bandwidth",
			config: "local-lvm:vm-100-disk-0,size=10G,cache=none",
			wantBW: nil,
		},
		{
			name: "all bandwidth fields",
			config: "local-lvm:vm-100-disk-0,size=10G," +
				"mbps_rd=100.5,mbps_rd_max=200,mbps_wr=50,mbps_wr_max=75.25," +
				"iops_rd=1000,iops_rd_max=2000,iops_wr=500,iops_wr_max=750",
			wantBW: &proxmox.DiskBandwidth{
				MBpsRd:    proxmoxPtr(100.5),
				MBpsRdMax: proxmoxPtr(200.0),
				MBpsWr:    proxmoxPtr(50.0),
				MBpsWrMax: proxmoxPtr(75.25),
				IOPSRd:    testutils.Ptr(1000),
				IOPSRdMax: testutils.Ptr(2000),
				IOPSWr:    testutils.Ptr(500),
				IOPSWrMax: testutils.Ptr(750),
			},
		},
		{
			name:   "only mbps read",
			config: "local-lvm:vm-100-disk-0,size=10G,mbps_rd=50,mbps_rd_max=100",
			wantBW: &proxmox.DiskBandwidth{
				MBpsRd:    proxmoxPtr(50.0),
				MBpsRdMax: proxmoxPtr(100.0),
			},
		},
		{
			name:   "only iops write",
			config: "local-lvm:vm-100-disk-0,size=10G,iops_wr=300,iops_wr_max=600",
			wantBW: &proxmox.DiskBandwidth{
				IOPSWr:    testutils.Ptr(300),
				IOPSWrMax: testutils.Ptr(600),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var disk proxmox.Disk
			err := adapters.ParseDiskConfig(&disk, tt.config)
			require.NoError(t, err)
			if tt.wantBW == nil {
				require.Nil(t, disk.Bandwidth)
			} else {
				require.NotNil(t, disk.Bandwidth)
				require.Equal(t, tt.wantBW, disk.Bandwidth)
			}
		})
	}
}

// proxmoxPtr is a local helper to take a pointer to a float64 literal.
func proxmoxPtr(v float64) *float64 { return &v }

// TestToProxmoxDiskKeyConfigMiscFields verifies that miscellaneous disk fields
// (format, serial, wwn, media, queues, snapshot, shared, rerror, werror, scsiblock)
// are correctly serialized into the Proxmox disk config string.
func TestToProxmoxDiskKeyConfigMiscFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		disk    proxmox.Disk
		wantKey string
		wantCfg string
	}{
		{
			name: "format=qcow2",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local"},
				Size:      20,
				Format:    testutils.Ptr("qcow2"),
			},
			wantKey: "scsi0",
			wantCfg: "file=local:20,size=20,format=qcow2",
		},
		{
			name: "serial",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				Serial:    testutils.Ptr("SN12345678"),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,serial=SN12345678",
		},
		{
			name: "wwn",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				WWN:       testutils.Ptr("0x500a0000deadbeef"),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,wwn=0x500a0000deadbeef",
		},
		{
			name: "media=cdrom",
			disk: proxmox.Disk{
				Interface: "ide2",
				DiskBase:  proxmox.DiskBase{Storage: "local"},
				Size:      0,
				Media:     testutils.Ptr("cdrom"),
			},
			wantKey: "ide2",
			wantCfg: "file=local:0,size=0,media=cdrom",
		},
		{
			name: "queues=4",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				Queues:    testutils.Ptr(4),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,queues=4",
		},
		{
			name: "snapshot=true",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				Snapshot:  testutils.Ptr(true),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,snapshot=1",
		},
		{
			name: "snapshot=false",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				Snapshot:  testutils.Ptr(false),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,snapshot=0",
		},
		{
			name: "shared=true",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "ceph-ha"},
				Size:      50,
				Shared:    testutils.Ptr(true),
			},
			wantKey: "scsi0",
			wantCfg: "file=ceph-ha:50,size=50,shared=1",
		},
		{
			name: "rerror=ignore",
			disk: proxmox.Disk{
				Interface: "ide0",
				DiskBase:  proxmox.DiskBase{Storage: "local"},
				Size:      10,
				RError:    testutils.Ptr("ignore"),
			},
			wantKey: "ide0",
			wantCfg: "file=local:10,size=10,rerror=ignore",
		},
		{
			name: "werror=enospc",
			disk: proxmox.Disk{
				Interface: "virtio0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      20,
				WError:    testutils.Ptr("enospc"),
			},
			wantKey: "virtio0",
			wantCfg: "file=local-lvm:20,size=20,werror=enospc",
		},
		{
			name: "scsiblock=true",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				ScsiBlock: testutils.Ptr(true),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,scsiblock=1",
		},
		{
			name: "scsiblock=false",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
				ScsiBlock: testutils.Ptr(false),
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10,scsiblock=0",
		},
		{
			name: "all miscellaneous fields combined",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local", FileID: testutils.Ptr("vm-100-disk-0.qcow2")},
				Size:      50,
				Format:    testutils.Ptr("qcow2"),
				Serial:    testutils.Ptr("DISK001"),
				WWN:       testutils.Ptr("0x5000000000000001"),
				Queues:    testutils.Ptr(8),
				Shared:    testutils.Ptr(false),
				ScsiBlock: testutils.Ptr(false),
			},
			wantKey: "scsi0",
			wantCfg: "file=local:vm-100-disk-0.qcow2,size=50," +
				"format=qcow2,serial=DISK001,wwn=0x5000000000000001,queues=8,shared=0,scsiblock=0",
		},
		{
			name: "nil miscellaneous fields produce no extra tokens",
			disk: proxmox.Disk{
				Interface: "scsi0",
				DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
				Size:      10,
			},
			wantKey: "scsi0",
			wantCfg: "file=local-lvm:10,size=10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotKey, gotCfg := adapters.ToProxmoxDiskKeyConfig(tt.disk)
			require.Equal(t, tt.wantKey, gotKey)
			require.Equal(t, tt.wantCfg, gotCfg)
		})
	}
}

// TestParseDiskConfigMiscFields verifies that miscellaneous disk fields
// (format, serial, wwn, media, queues, snapshot, shared, rerror, werror, scsiblock)
// are correctly deserialized from Proxmox disk config strings.
func TestParseDiskConfigMiscFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
		want   func(disk proxmox.Disk) // assertions on the parsed disk
	}{
		{
			name:   "format=raw",
			config: "local:vm-100-disk-0.raw,size=10G,format=raw",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Format)
				require.Equal(t, "raw", *disk.Format)
			},
		},
		{
			name:   "format=qcow2",
			config: "local:vm-100-disk-0.qcow2,size=10G,format=qcow2",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Format)
				require.Equal(t, "qcow2", *disk.Format)
			},
		},
		{
			name:   "serial",
			config: "local-lvm:vm-100-disk-0,size=20G,serial=SN12345678",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Serial)
				require.Equal(t, "SN12345678", *disk.Serial)
			},
		},
		{
			name:   "wwn",
			config: "local-lvm:vm-100-disk-0,size=20G,wwn=0x500a0000deadbeef",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.WWN)
				require.Equal(t, "0x500a0000deadbeef", *disk.WWN)
			},
		},
		{
			name:   "media=cdrom",
			config: "local:iso/debian.iso,size=1G,media=cdrom",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Media)
				require.Equal(t, "cdrom", *disk.Media)
			},
		},
		{
			name:   "media=disk",
			config: "local-lvm:vm-100-disk-0,size=10G,media=disk",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Media)
				require.Equal(t, "disk", *disk.Media)
			},
		},
		{
			name:   "queues=4",
			config: "local-lvm:vm-100-disk-0,size=10G,queues=4",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Queues)
				require.Equal(t, 4, *disk.Queues)
			},
		},
		{
			name:   "snapshot=1",
			config: "local-lvm:vm-100-disk-0,size=10G,snapshot=1",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Snapshot)
				require.True(t, *disk.Snapshot)
			},
		},
		{
			name:   "snapshot=0",
			config: "local-lvm:vm-100-disk-0,size=10G,snapshot=0",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Snapshot)
				require.False(t, *disk.Snapshot)
			},
		},
		{
			name:   "shared=1",
			config: "ceph-ha:vm-100-disk-0,size=50G,shared=1",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Shared)
				require.True(t, *disk.Shared)
			},
		},
		{
			name:   "shared=0",
			config: "local-lvm:vm-100-disk-0,size=10G,shared=0",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Shared)
				require.False(t, *disk.Shared)
			},
		},
		{
			name:   "rerror=ignore",
			config: "local:vm-100-disk-0,size=10G,rerror=ignore",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.RError)
				require.Equal(t, "ignore", *disk.RError)
			},
		},
		{
			name:   "rerror=stop",
			config: "local:vm-100-disk-0,size=10G,rerror=stop",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.RError)
				require.Equal(t, "stop", *disk.RError)
			},
		},
		{
			name:   "werror=enospc",
			config: "local-lvm:vm-100-disk-0,size=20G,werror=enospc",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.WError)
				require.Equal(t, "enospc", *disk.WError)
			},
		},
		{
			name:   "werror=report",
			config: "local-lvm:vm-100-disk-0,size=20G,werror=report",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.WError)
				require.Equal(t, "report", *disk.WError)
			},
		},
		{
			name:   "scsiblock=1",
			config: "local-lvm:vm-100-disk-0,size=10G,scsiblock=1",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.ScsiBlock)
				require.True(t, *disk.ScsiBlock)
			},
		},
		{
			name:   "scsiblock=0",
			config: "local-lvm:vm-100-disk-0,size=10G,scsiblock=0",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.ScsiBlock)
				require.False(t, *disk.ScsiBlock)
			},
		},
		{
			name: "all miscellaneous fields combined",
			config: "local:vm-100-disk-0.qcow2,size=50G," +
				"format=qcow2,serial=DISK001,wwn=0x5000000000000001," +
				"queues=8,snapshot=0,shared=1,rerror=report,werror=enospc,scsiblock=0",
			want: func(disk proxmox.Disk) {
				require.NotNil(t, disk.Format)
				require.Equal(t, "qcow2", *disk.Format)
				require.NotNil(t, disk.Serial)
				require.Equal(t, "DISK001", *disk.Serial)
				require.NotNil(t, disk.WWN)
				require.Equal(t, "0x5000000000000001", *disk.WWN)
				require.NotNil(t, disk.Queues)
				require.Equal(t, 8, *disk.Queues)
				require.NotNil(t, disk.Snapshot)
				require.False(t, *disk.Snapshot)
				require.NotNil(t, disk.Shared)
				require.True(t, *disk.Shared)
				require.NotNil(t, disk.RError)
				require.Equal(t, "report", *disk.RError)
				require.NotNil(t, disk.WError)
				require.Equal(t, "enospc", *disk.WError)
				require.NotNil(t, disk.ScsiBlock)
				require.False(t, *disk.ScsiBlock)
			},
		},
		{
			name:   "no miscellaneous fields → all nil",
			config: "local-lvm:vm-100-disk-0,size=10G,cache=none",
			want: func(disk proxmox.Disk) {
				require.Nil(t, disk.Format)
				require.Nil(t, disk.Serial)
				require.Nil(t, disk.WWN)
				require.Nil(t, disk.Media)
				require.Nil(t, disk.Queues)
				require.Nil(t, disk.Snapshot)
				require.Nil(t, disk.Shared)
				require.Nil(t, disk.RError)
				require.Nil(t, disk.WError)
				require.Nil(t, disk.ScsiBlock)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var disk proxmox.Disk
			err := adapters.ParseDiskConfig(&disk, tt.config)
			require.NoError(t, err)
			tt.want(disk)
		})
	}
}
