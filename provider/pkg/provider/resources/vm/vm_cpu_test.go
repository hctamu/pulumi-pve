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

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// cpuBase creates a minimal CPU config with just a type
func cpuBase(typ string) *vm.CPU {
	return &vm.CPU{Type: testutils.Ptr(typ)}
}

// cpuWith creates a CPU config with multiple fields set via a map for concise test case definitions
func cpuWith(typ string, fields map[string]interface{}) *vm.CPU {
	cpu := cpuBase(typ)
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
			cpu.NumaNodes = v.([]vm.NumaNode)
		}
	}
	return cpu
}

// inputsWithCPU wraps a CPU config in an Inputs struct for test convenience
func inputsWithCPU(cpu *vm.CPU) vm.Inputs {
	return vm.Inputs{CPU: cpu}
}

// opt creates a VirtualMachineOption concisely for expected test results
func opt(name string, value interface{}) api.VirtualMachineOption {
	return api.VirtualMachineOption{Name: name, Value: value}
}

// TestParseCPU verifies that ParseCPU correctly parses various CPU configuration strings
func TestParseCPU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    *vm.CPU
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
			expected: &vm.CPU{
				Type: testutils.Ptr("host"),
			},
			expectError: false,
		},
		{
			name:  "simple CPU type - kvm64",
			input: "kvm64",
			expected: &vm.CPU{
				Type: testutils.Ptr("kvm64"),
			},
			expectError: false,
		},
		{
			name:  "CPU type with cputype key",
			input: "cputype=x86-64-v2-AES",
			expected: &vm.CPU{
				Type: testutils.Ptr("x86-64-v2-AES"),
			},
			expectError: false,
		},
		{
			name:  "CPU with single enabled flag",
			input: "host,flags=+aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "CPU with single disabled flag",
			input: "host,flags=-pcid",
			expected: &vm.CPU{
				Type:          testutils.Ptr("host"),
				FlagsDisabled: []string{"pcid"},
			},
			expectError: false,
		},
		{
			name:  "CPU with mixed enabled and disabled flags",
			input: "host,flags=+aes;-pcid;+avx2",
			expected: &vm.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes", "avx2"},
				FlagsDisabled: []string{"pcid"},
			},
			expectError: false,
		},
		{
			name:  "CPU with flag without prefix (treated as enabled)",
			input: "host,flags=aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "CPU with mixed prefixed and unprefixed flags",
			input: "host,flags=aes;-pcid;+avx2;spec-ctrl",
			expected: &vm.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes", "avx2", "spec-ctrl"},
				FlagsDisabled: []string{"pcid"},
			},
			expectError: false,
		},
		{
			name:  "CPU with hidden=1",
			input: "host,hidden=1",
			expected: &vm.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(true),
			},
			expectError: false,
		},
		{
			name:  "CPU with hidden=0",
			input: "host,hidden=0",
			expected: &vm.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(false),
			},
			expectError: false,
		},
		{
			name:  "CPU with hv-vendor-id",
			input: "host,hv-vendor-id=AuthenticAMD",
			expected: &vm.CPU{
				Type:       testutils.Ptr("host"),
				HVVendorID: testutils.Ptr("AuthenticAMD"),
			},
			expectError: false,
		},
		{
			name:  "CPU with phys-bits",
			input: "host,phys-bits=40",
			expected: &vm.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("40"),
			},
			expectError: false,
		},
		{
			name:  "CPU with phys-bits=host",
			input: "host,phys-bits=host",
			expected: &vm.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("host"),
			},
			expectError: false,
		},
		{
			name:  "comprehensive CPU config",
			input: "host,flags=+aes;-pcid;+avx2,hidden=1,hv-vendor-id=GenuineIntel,phys-bits=42",
			expected: &vm.CPU{
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
			expected: &vm.CPU{
				Type: testutils.Ptr("host"),
			},
			expectError: false,
		},
		{
			name:  "flags with empty segments",
			input: "host,flags=+aes;;-pcid",
			expected: &vm.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes"},
				FlagsDisabled: []string{"pcid"},
			},
			expectError: false,
		},
		{
			name:  "multiple commas in config",
			input: "host,,flags=+aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "unknown keys are ignored",
			input: "host,unknown=value,flags=+aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "segment without equals sign after first",
			input: "host,someflag,flags=+aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
		},
		{
			name:  "hidden with invalid value",
			input: "host,hidden=invalid",
			expected: &vm.CPU{
				Type: testutils.Ptr("host"),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := vm.ParseCPU(tt.input)

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
		expected    *vm.NumaNode
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
			expected: &vm.NumaNode{
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
			expected: &vm.NumaNode{
				Cpus: "0-1",
			},
			expectError: false,
		},
		{
			name:  "cpus with hostnodes",
			input: "cpus=0-3,hostnodes=0-1",
			expected: &vm.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0-1"),
			},
			expectError: false,
		},
		{
			name:  "cpus with memory",
			input: "cpus=0-3,memory=4096",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(4096),
			},
			expectError: false,
		},
		{
			name:  "cpus with policy",
			input: "cpus=0-3,policy=preferred",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("preferred"),
			},
			expectError: false,
		},
		{
			name:  "single CPU core",
			input: "cpus=0",
			expected: &vm.NumaNode{
				Cpus: "0",
			},
			expectError: false,
		},
		{
			name:  "complex CPU range format",
			input: "cpus=0-3;5-7",
			expected: &vm.NumaNode{
				Cpus: "0-3;5-7",
			},
			expectError: false,
		},
		{
			name:  "multiple host nodes",
			input: "cpus=0-7,hostnodes=0-2",
			expected: &vm.NumaNode{
				Cpus:      "0-7",
				HostNodes: testutils.Ptr("0-2"),
			},
			expectError: false,
		},
		{
			name:  "policy interleave",
			input: "cpus=0-3,policy=interleave",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("interleave"),
			},
			expectError: false,
		},
		{
			name:  "empty segments ignored",
			input: "cpus=0-3,,memory=2048",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(2048),
			},
			expectError: false,
		},
		{
			name:  "unknown keys ignored",
			input: "cpus=0-3,unknown=value,memory=2048",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(2048),
			},
			expectError: false,
		},
		{
			name:  "segments without equals sign ignored",
			input: "cpus=0-3,someflag,memory=2048",
			expected: &vm.NumaNode{
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
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(-1024),
			},
			expectError: false,
		},
		{
			name:  "large memory value",
			input: "cpus=0-15,memory=65536",
			expected: &vm.NumaNode{
				Cpus:   "0-15",
				Memory: testutils.Ptr(65536),
			},
			expectError: false,
		},
		{
			name:  "all fields in different order",
			input: "policy=bind,memory=4096,hostnodes=0-1,cpus=0-7",
			expected: &vm.NumaNode{
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

			result, err := vm.ParseNumaNode(tt.input)

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

// TestCPUToProxmoxString verifies that ToProxmoxString correctly serializes CPU struct to Proxmox format
func TestCPUToProxmoxString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       *vm.CPU
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
			input: &vm.CPU{
				Type: testutils.Ptr("host"),
			},
			expected: "host",
		},
		{
			name: "CPU type kvm64",
			input: &vm.CPU{
				Type: testutils.Ptr("kvm64"),
			},
			expected: "kvm64",
		},
		{
			name: "CPU with single enabled flag",
			input: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expected: "host,flags=+aes",
		},
		{
			name: "CPU with single disabled flag",
			input: &vm.CPU{
				Type:          testutils.Ptr("host"),
				FlagsDisabled: []string{"pcid"},
			},
			expected: "host,flags=-pcid",
		},
		{
			name: "CPU with mixed enabled and disabled flags",
			input: &vm.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes", "avx2"},
				FlagsDisabled: []string{"pcid"},
			},
			expected: "host,flags=+aes;+avx2;-pcid",
		},
		{
			name: "CPU with hidden true",
			input: &vm.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(true),
			},
			expected: "host,hidden=1",
		},
		{
			name: "CPU with hidden false",
			input: &vm.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(false),
			},
			expected: "host,hidden=0",
		},
		{
			name: "CPU with hv-vendor-id",
			input: &vm.CPU{
				Type:       testutils.Ptr("host"),
				HVVendorID: testutils.Ptr("AuthenticAMD"),
			},
			expected: "host,hv-vendor-id=AuthenticAMD",
		},
		{
			name: "CPU with phys-bits numeric",
			input: &vm.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("40"),
			},
			expected: "host,phys-bits=40",
		},
		{
			name: "CPU with phys-bits host",
			input: &vm.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("host"),
			},
			expected: "host,phys-bits=host",
		},
		{
			name: "comprehensive CPU config",
			input: &vm.CPU{
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
			input: &vm.CPU{
				Type: testutils.Ptr(""),
			},
			expected: "",
		},
		{
			name: "CPU without type but with flags",
			input: &vm.CPU{
				FlagsEnabled: []string{"aes"},
			},
			expected: "flags=+aes",
		},
		{
			name: "empty flags arrays",
			input: &vm.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{},
				FlagsDisabled: []string{},
			},
			expected: "host",
		},
		{
			name: "flags with empty strings filtered",
			input: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes", "", "avx2"},
			},
			expected: "host,flags=+aes;+avx2",
		},
		{
			name: "only disabled flags",
			input: &vm.CPU{
				Type:          testutils.Ptr("host"),
				FlagsDisabled: []string{"pcid", "spec-ctrl"},
			},
			expected: "host,flags=-pcid;-spec-ctrl",
		},
		{
			name: "all optional fields nil",
			input: &vm.CPU{
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
			input: &vm.CPU{
				Type: testutils.Ptr("x86-64-v2-AES"),
			},
			expected: "x86-64-v2-AES",
		},
		{
			name: "multiple flags with special characters",
			input: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes", "spec-ctrl", "ibpb"},
			},
			expected: "host,flags=+aes;+spec-ctrl;+ibpb",
		},
		{
			name: "hidden false with other fields",
			input: &vm.CPU{
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

			result := tt.input.ToProxmoxString()
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestNumaNodeToProxmoxString verifies that ToProxmoxNumaString correctly serializes NumaNode struct to Proxmox format
func TestNumaNodeToProxmoxString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       *vm.NumaNode
		expected    string
		description string
	}{
		{
			name: "minimal config - only cpus",
			input: &vm.NumaNode{
				Cpus: "0-1",
			},
			expected: "cpus=0-1",
		},
		{
			name: "complete NUMA config",
			input: &vm.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0"),
				Memory:    testutils.Ptr(2048),
				Policy:    testutils.Ptr("bind"),
			},
			expected: "cpus=0-3,hostnodes=0,memory=2048,policy=bind",
		},
		{
			name: "cpus with hostnodes",
			input: &vm.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0-1"),
			},
			expected: "cpus=0-3,hostnodes=0-1",
		},
		{
			name: "cpus with memory",
			input: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(4096),
			},
			expected: "cpus=0-3,memory=4096",
		},
		{
			name: "cpus with policy",
			input: &vm.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("preferred"),
			},
			expected: "cpus=0-3,policy=preferred",
		},
		{
			name: "single CPU core",
			input: &vm.NumaNode{
				Cpus: "0",
			},
			expected: "cpus=0",
		},
		{
			name: "complex CPU range",
			input: &vm.NumaNode{
				Cpus: "0-3;5-7",
			},
			expected: "cpus=0-3;5-7",
		},
		{
			name: "multiple host nodes range",
			input: &vm.NumaNode{
				Cpus:      "0-7",
				HostNodes: testutils.Ptr("0-2"),
			},
			expected: "cpus=0-7,hostnodes=0-2",
		},
		{
			name: "policy interleave",
			input: &vm.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("interleave"),
			},
			expected: "cpus=0-3,policy=interleave",
		},
		{
			name: "policy preferred",
			input: &vm.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("preferred"),
			},
			expected: "cpus=0-3,policy=preferred",
		},
		{
			name: "large memory value",
			input: &vm.NumaNode{
				Cpus:   "0-15",
				Memory: testutils.Ptr(65536),
			},
			expected: "cpus=0-15,memory=65536",
		},
		{
			name: "small memory value",
			input: &vm.NumaNode{
				Cpus:   "0-1",
				Memory: testutils.Ptr(512),
			},
			expected: "cpus=0-1,memory=512",
		},
		{
			name: "zero memory value",
			input: &vm.NumaNode{
				Cpus:   "0-1",
				Memory: testutils.Ptr(0),
			},
			expected: "cpus=0-1,memory=0",
		},
		{
			name: "hostnodes and memory",
			input: &vm.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0"),
				Memory:    testutils.Ptr(2048),
			},
			expected: "cpus=0-3,hostnodes=0,memory=2048",
		},
		{
			name: "hostnodes and policy",
			input: &vm.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0-1"),
				Policy:    testutils.Ptr("bind"),
			},
			expected: "cpus=0-3,hostnodes=0-1,policy=bind",
		},
		{
			name: "memory and policy",
			input: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(4096),
				Policy: testutils.Ptr("interleave"),
			},
			expected: "cpus=0-3,memory=4096,policy=interleave",
		},
		{
			name: "all optional fields nil",
			input: &vm.NumaNode{
				Cpus:      "0-7",
				HostNodes: nil,
				Memory:    nil,
				Policy:    nil,
			},
			expected: "cpus=0-7",
		},
		{
			name: "empty cpus string",
			input: &vm.NumaNode{
				Cpus: "",
			},
			expected: "cpus=",
		},
		{
			name: "cpus with special formats",
			input: &vm.NumaNode{
				Cpus: "0,2,4,6",
			},
			expected: "cpus=0,2,4,6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.input.ToProxmoxNumaString()
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestCPURoundTrip verifies that CPU parsing and serialization are perfectly bidirectional
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

			// Step 1: Parse the input string
			parsed1, err := vm.ParseCPU(tt.input)
			require.NoError(t, err, "First parse should not error")
			require.NotNil(t, parsed1, "First parse should return non-nil CPU")

			// Step 2: Convert back to string
			serialized := parsed1.ToProxmoxString()
			require.NotEmpty(t, serialized, "Serialization should produce non-empty string")

			// Step 3: Parse the serialized string again
			parsed2, err := vm.ParseCPU(serialized)
			require.NoError(t, err, "Second parse should not error")
			require.NotNil(t, parsed2, "Second parse should return non-nil CPU")

			// Step 4: Verify both parsed structs are identical
			assert.Equal(t, parsed1.Type, parsed2.Type, "CPU Type should match after round-trip")
			assert.Equal(t, parsed1.FlagsEnabled, parsed2.FlagsEnabled, "FlagsEnabled should match after round-trip")
			assert.Equal(t, parsed1.FlagsDisabled, parsed2.FlagsDisabled, "FlagsDisabled should match after round-trip")
			assert.Equal(t, parsed1.Hidden, parsed2.Hidden, "Hidden should match after round-trip")
			assert.Equal(t, parsed1.HVVendorID, parsed2.HVVendorID, "HVVendorID should match after round-trip")
			assert.Equal(t, parsed1.PhysBits, parsed2.PhysBits, "PhysBits should match after round-trip")

			// Step 5: Convert second parsed struct to string and verify it matches
			serialized2 := parsed2.ToProxmoxString()
			assert.Equal(t, serialized, serialized2, "Second serialization should match first serialization")
		})
	}
}

// TestBuildOptionsCPUFields verifies that CPU fields generate correct VirtualMachineOptions
func TestBuildOptionsCPUFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputs      vm.Inputs
		expected    []api.VirtualMachineOption
		description string
	}{
		{
			name: "basic CPU with cores and sockets",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
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
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					Type: testutils.Ptr("host"),
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "cpu", Value: "host"},
			},
		},
		{
			name: "CPU with limit, units, and vcpus",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
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
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					Numa: testutils.Ptr(true),
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "numa", Value: 1},
			},
		},
		{
			name: "CPU with NUMA disabled",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					Numa: testutils.Ptr(false),
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "numa", Value: 0},
			},
		},
		{
			name: "CPU with single NUMA node",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					NumaNodes: []vm.NumaNode{
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
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					NumaNodes: []vm.NumaNode{
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
			inputs: vm.Inputs{
				CPU: &vm.CPU{
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
					NumaNodes: []vm.NumaNode{
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
			inputs: vm.Inputs{
				CPU: nil,
			},
			expected: []api.VirtualMachineOption{},
		},
		{
			name: "empty CPU struct",
			inputs: vm.Inputs{
				CPU: &vm.CPU{},
			},
			expected: []api.VirtualMachineOption{},
		},
		{
			name: "CPU with empty type string",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					Type: testutils.Ptr(""),
				},
			},
			expected: []api.VirtualMachineOption{},
		},
		{
			name: "CPU with flags only",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					FlagsEnabled: []string{"aes", "avx2"},
				},
			},
			expected: []api.VirtualMachineOption{
				{Name: "cpu", Value: "flags=+aes;+avx2"},
			},
		},
		{
			name: "CPU with type and cores",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
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
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					NumaNodes: []vm.NumaNode{},
				},
			},
			expected: []api.VirtualMachineOption{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Call BuildOptions with a dummy VMID
			options := tt.inputs.BuildOptions(100)

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
		inputs      vm.Inputs
		description string
	}{
		{
			name: "simple CPU config",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					Type:    testutils.Ptr("host"),
					Cores:   testutils.Ptr(4),
					Sockets: testutils.Ptr(2),
				},
			},
		},
		{
			name: "CPU with all fields",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
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
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					Type:  testutils.Ptr("kvm64"),
					Cores: testutils.Ptr(8),
					Numa:  testutils.Ptr(true),
					NumaNodes: []vm.NumaNode{
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
			inputs: vm.Inputs{
				CPU: &vm.CPU{
					Type: testutils.Ptr("host"),
				},
			},
		},
		{
			name: "CPU with only numeric fields",
			inputs: vm.Inputs{
				CPU: &vm.CPU{
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
			inputs: vm.Inputs{
				CPU: &vm.CPU{
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
			inputs: vm.Inputs{
				CPU: &vm.CPU{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Call BuildOptions multiple times
			const iterations = 10
			var results [][]api.VirtualMachineOption

			for i := 0; i < iterations; i++ {
				options := tt.inputs.BuildOptions(100 + i) // Use different VMIDs
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
		newInputs vm.Inputs
		oldInputs vm.Inputs
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
						"numa-nodes": []vm.NumaNode{
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
					map[string]interface{}{"numa-nodes": []vm.NumaNode{{Cpus: "0-3", Memory: testutils.Ptr(2048)}}},
				),
			),
			oldInputs: inputsWithCPU(
				cpuWith(
					"host",
					map[string]interface{}{"numa-nodes": []vm.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(1024)}}},
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
					map[string]interface{}{"numa-nodes": []vm.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(1024)}}},
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
			newInputs: inputsWithCPU(&vm.CPU{
				Type: testutils.Ptr(
					"kvm64",
				), FlagsEnabled: []string{"aes", "avx2"}, FlagsDisabled: []string{"pcid"}, Hidden: testutils.Ptr(true),
				HVVendorID: testutils.Ptr(
					"GenuineIntel",
				), PhysBits: testutils.Ptr("42"), Cores: testutils.Ptr(16), Sockets: testutils.Ptr(4),
				Limit: testutils.Ptr(4.0), Units: testutils.Ptr(4096), Vcpus: testutils.Ptr(32), Numa: testutils.Ptr(true),
				NumaNodes: []vm.NumaNode{
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
			newInputs: inputsWithCPU(&vm.CPU{}),
			oldInputs: inputsWithCPU(&vm.CPU{}),
			expected:  []api.VirtualMachineOption{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			options := tt.newInputs.BuildOptionsDiff(100, &tt.oldInputs)
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

// TestVMDiffCPU verifies that VM Diff detects all CPU field changes correctly (simple and complex fields)
func TestVMDiffCPU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputCPU     *vm.CPU
		stateCPU     *vm.CPU
		expectChange bool
	}{
		{
			name:         "cores_changed",
			inputCPU:     cpuWith("host", map[string]interface{}{"cores": 8}),
			stateCPU:     cpuWith("host", map[string]interface{}{"cores": 4}),
			expectChange: true,
		},
		{
			name:         "sockets_changed",
			inputCPU:     cpuWith("host", map[string]interface{}{"sockets": 4}),
			stateCPU:     cpuWith("host", map[string]interface{}{"sockets": 2}),
			expectChange: true,
		},
		{
			name:         "limit_changed",
			inputCPU:     cpuWith("host", map[string]interface{}{"limit": 3.0}),
			stateCPU:     cpuWith("host", map[string]interface{}{"limit": 2.0}),
			expectChange: true,
		},
		{
			name:         "units_changed",
			inputCPU:     cpuWith("host", map[string]interface{}{"units": 2048}),
			stateCPU:     cpuWith("host", map[string]interface{}{"units": 1024}),
			expectChange: true,
		},
		{
			name:         "vcpus_changed",
			inputCPU:     cpuWith("host", map[string]interface{}{"vcpus": 16}),
			stateCPU:     cpuWith("host", map[string]interface{}{"vcpus": 8}),
			expectChange: true,
		},
		{name: "type_changed", inputCPU: cpuBase("kvm64"), stateCPU: cpuBase("host"), expectChange: true},
		{
			name:         "cores_added_from_nil",
			inputCPU:     cpuWith("host", map[string]interface{}{"cores": 4}),
			stateCPU:     cpuBase("host"),
			expectChange: true,
		},
		{
			name:         "cores_removed_to_nil",
			inputCPU:     cpuBase("host"),
			stateCPU:     cpuWith("host", map[string]interface{}{"cores": 4}),
			expectChange: true,
		},
		{
			name:         "numa_enabled_changed",
			inputCPU:     cpuWith("host", map[string]interface{}{"cores": 2, "numa": true}),
			stateCPU:     cpuWith("host", map[string]interface{}{"cores": 2, "numa": false}),
			expectChange: true,
		},
		{
			name:         "numa_enabled_unchanged",
			inputCPU:     cpuWith("host", map[string]interface{}{"cores": 2, "numa": true}),
			stateCPU:     cpuWith("host", map[string]interface{}{"cores": 2, "numa": true}),
			expectChange: false,
		},
		{
			name: "numa_nodes_added", expectChange: true,
			inputCPU: cpuWith(
				"host",
				map[string]interface{}{
					"cores": 4,
					"numa":  true,
					"numa-nodes": []vm.NumaNode{
						{Cpus: "0-1", Memory: testutils.Ptr(2048)},
						{Cpus: "2-3", Memory: testutils.Ptr(2048)},
					},
				},
			),
			stateCPU: cpuWith("host", map[string]interface{}{"cores": 4}),
		},
		{
			name: "numa_nodes_removed", expectChange: true,
			inputCPU: cpuWith("host", map[string]interface{}{"cores": 4}),
			stateCPU: cpuWith(
				"host",
				map[string]interface{}{
					"cores":      4,
					"numa":       true,
					"numa-nodes": []vm.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(2048)}},
				},
			),
		},
		{
			name: "numa_nodes_changed_-_different_cpus", expectChange: true,
			inputCPU: cpuWith(
				"host",
				map[string]interface{}{"cores": 4, "numa-nodes": []vm.NumaNode{{Cpus: "0-3", Memory: testutils.Ptr(2048)}}},
			),
			stateCPU: cpuWith(
				"host",
				map[string]interface{}{"cores": 4, "numa-nodes": []vm.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(2048)}}},
			),
		},
		{
			name: "numa_nodes_changed_-_different_memory", expectChange: true,
			inputCPU: cpuWith(
				"host",
				map[string]interface{}{"numa-nodes": []vm.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(4096)}}},
			),
			stateCPU: cpuWith(
				"host",
				map[string]interface{}{"numa-nodes": []vm.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(2048)}}},
			),
		},
		{
			name: "numa_nodes_changed_-_different_count", expectChange: true,
			inputCPU: cpuWith(
				"host",
				map[string]interface{}{
					"numa-nodes": []vm.NumaNode{
						{Cpus: "0-1", Memory: testutils.Ptr(2048)},
						{Cpus: "2-3", Memory: testutils.Ptr(2048)},
					},
				},
			),
			stateCPU: cpuWith(
				"host",
				map[string]interface{}{"numa-nodes": []vm.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(2048)}}},
			),
		},
		{
			name: "numa_nodes_unchanged", expectChange: false,
			inputCPU: cpuWith(
				"host",
				map[string]interface{}{
					"numa-nodes": []vm.NumaNode{
						{Cpus: "0-1", Memory: testutils.Ptr(2048)},
						{Cpus: "2-3", Memory: testutils.Ptr(2048)},
					},
				},
			),
			stateCPU: cpuWith(
				"host",
				map[string]interface{}{
					"numa-nodes": []vm.NumaNode{
						{Cpus: "0-1", Memory: testutils.Ptr(2048)},
						{Cpus: "2-3", Memory: testutils.Ptr(2048)},
					},
				},
			),
		},
		{
			name:         "flags_changed_-_added_enabled_flag",
			inputCPU:     cpuWith("host", map[string]interface{}{"flags+": []string{"aes"}}),
			stateCPU:     cpuBase("host"),
			expectChange: true,
		},
		{
			name:         "flags_changed_-_removed_enabled_flag",
			inputCPU:     cpuBase("host"),
			stateCPU:     cpuWith("host", map[string]interface{}{"flags+": []string{"aes"}}),
			expectChange: true,
		},
		{
			name:         "flags_changed_-_different_enabled_flags",
			inputCPU:     cpuWith("host", map[string]interface{}{"flags+": []string{"md-clear"}}),
			stateCPU:     cpuWith("host", map[string]interface{}{"flags+": []string{"aes"}}),
			expectChange: true,
		},
		{
			name:         "flags_changed_-_disabled_flag_added",
			inputCPU:     cpuWith("host", map[string]interface{}{"flags-": []string{"pcid"}}),
			stateCPU:     cpuBase("host"),
			expectChange: true,
		},
		{
			name:         "flags_changed_-_both_enabled_and_disabled",
			inputCPU:     cpuWith("host", map[string]interface{}{"flags+": []string{"aes"}, "flags-": []string{"pcid"}}),
			stateCPU:     cpuWith("host", map[string]interface{}{"flags+": []string{"aes"}}),
			expectChange: true,
		},
		{
			name:         "flags_unchanged",
			inputCPU:     cpuWith("host", map[string]interface{}{"flags+": []string{"aes", "md-clear"}}),
			stateCPU:     cpuWith("host", map[string]interface{}{"flags+": []string{"aes", "md-clear"}}),
			expectChange: false,
		},
		{
			name:         "hidden_changed",
			inputCPU:     cpuWith("host", map[string]interface{}{"hidden": true}),
			stateCPU:     cpuWith("host", map[string]interface{}{"hidden": false}),
			expectChange: true,
		},
		{
			name:         "hidden_unchanged",
			inputCPU:     cpuWith("host", map[string]interface{}{"hidden": true}),
			stateCPU:     cpuWith("host", map[string]interface{}{"hidden": true}),
			expectChange: false,
		},
		{
			name:         "hv-vendor-id_changed",
			inputCPU:     cpuWith("host", map[string]interface{}{"hv-vendor-id": "GenuineIntel"}),
			stateCPU:     cpuWith("host", map[string]interface{}{"hv-vendor-id": "AuthenticAMD"}),
			expectChange: true,
		},
		{
			name:         "hv-vendor-id_unchanged",
			inputCPU:     cpuWith("host", map[string]interface{}{"hv-vendor-id": "GenuineIntel"}),
			stateCPU:     cpuWith("host", map[string]interface{}{"hv-vendor-id": "GenuineIntel"}),
			expectChange: false,
		},
		{
			name:         "phys-bits_changed",
			inputCPU:     cpuWith("host", map[string]interface{}{"phys-bits": "host"}),
			stateCPU:     cpuWith("host", map[string]interface{}{"phys-bits": "32"}),
			expectChange: true,
		},
		{
			name:         "phys-bits_unchanged",
			inputCPU:     cpuWith("host", map[string]interface{}{"phys-bits": "host"}),
			stateCPU:     cpuWith("host", map[string]interface{}{"phys-bits": "host"}),
			expectChange: false,
		},
		{
			name: "multiple_complex_fields_changed", expectChange: true,
			inputCPU: &vm.CPU{
				Type:         testutils.Ptr("host"),
				Cores:        testutils.Ptr(4),
				Numa:         testutils.Ptr(true),
				Hidden:       testutils.Ptr(true),
				FlagsEnabled: []string{"aes"},
				NumaNodes: []vm.NumaNode{
					{Cpus: "0-1", Memory: testutils.Ptr(2048)},
					{Cpus: "2-3", Memory: testutils.Ptr(2048)},
				},
			},
			stateCPU: &vm.CPU{
				Type:   testutils.Ptr("host"),
				Cores:  testutils.Ptr(2),
				Numa:   testutils.Ptr(false),
				Hidden: testutils.Ptr(false),
			},
		},
		{
			name: "all_complex_fields_unchanged", expectChange: false,
			inputCPU: &vm.CPU{
				Type:          testutils.Ptr("host"),
				Numa:          testutils.Ptr(true),
				Hidden:        testutils.Ptr(true),
				HVVendorID:    testutils.Ptr("GenuineIntel"),
				PhysBits:      testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes"},
				FlagsDisabled: []string{"pcid"},
				NumaNodes: []vm.NumaNode{
					{Cpus: "0-1", Memory: testutils.Ptr(2048), HostNodes: testutils.Ptr("0"), Policy: testutils.Ptr("preferred")},
				},
			},
			stateCPU: &vm.CPU{
				Type:          testutils.Ptr("host"),
				Numa:          testutils.Ptr(true),
				Hidden:        testutils.Ptr(true),
				HVVendorID:    testutils.Ptr("GenuineIntel"),
				PhysBits:      testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes"},
				FlagsDisabled: []string{"pcid"},
				NumaNodes: []vm.NumaNode{
					{Cpus: "0-1", Memory: testutils.Ptr(2048), HostNodes: testutils.Ptr("0"), Policy: testutils.Ptr("preferred")},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vmResource := &vm.VM{}
			req := infer.DiffRequest[vm.Inputs, vm.Outputs]{
				ID: "100",
				Inputs: vm.Inputs{
					CPU: tt.inputCPU,
				},
				State: vm.Outputs{
					Inputs: vm.Inputs{
						CPU: tt.stateCPU,
					},
				},
			}

			resp, err := vmResource.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, "Expected changes for test: %s", tt.name)
				assert.Contains(t, resp.DetailedDiff, "cpu", "Expected CPU in diff for test: %s", tt.name)
			} else {
				assert.False(t, resp.HasChanges, "Expected no changes for test: %s", tt.name)
			}
		})
	}
}

// TestNumaNodeRoundTrip verifies that NUMA node parsing and serialization are perfectly bidirectional
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

			// Step 1: Parse the input string
			parsed1, err := vm.ParseNumaNode(tt.input)
			require.NoError(t, err, "First parse should not error")
			require.NotNil(t, parsed1, "First parse should return non-nil NumaNode")

			// Step 2: Convert back to string
			serialized := parsed1.ToProxmoxNumaString()
			require.NotEmpty(t, serialized, "Serialization should produce non-empty string")

			// Step 3: Parse the serialized string again
			parsed2, err := vm.ParseNumaNode(serialized)
			require.NoError(t, err, "Second parse should not error")
			require.NotNil(t, parsed2, "Second parse should return non-nil NumaNode")

			// Step 4: Verify both parsed structs are identical
			assert.Equal(t, parsed1.Cpus, parsed2.Cpus, "Cpus should match after round-trip")
			assert.Equal(t, parsed1.HostNodes, parsed2.HostNodes, "HostNodes should match after round-trip")
			assert.Equal(t, parsed1.Memory, parsed2.Memory, "Memory should match after round-trip")
			assert.Equal(t, parsed1.Policy, parsed2.Policy, "Policy should match after round-trip")

			// Step 5: Convert second parsed struct to string and verify it matches
			serialized2 := parsed2.ToProxmoxNumaString()
			assert.Equal(t, serialized, serialized2, "Second serialization should match first serialization")
		})
	}
}
