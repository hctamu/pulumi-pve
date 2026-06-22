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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

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

			// Step 1: Parse the input string
			parsed1, err := adapters.ParseCPU(tt.input)
			require.NoError(t, err, "First parse should not error")
			require.NotNil(t, parsed1, "First parse should return non-nil CPU")

			// Step 2: Convert back to string
			serialized := adapters.CPUToProxmoxString(parsed1)
			require.NotEmpty(t, serialized, "Serialization should produce non-empty string")

			// Step 3: Parse the serialized string again
			parsed2, err := adapters.ParseCPU(serialized)
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
			serialized2 := adapters.CPUToProxmoxString(parsed2)
			assert.Equal(t, serialized, serialized2, "Second serialization should match first serialization")
		})
	}
}

// TestBuildOptionsCPUFields verifies that CPU fields generate correct VirtualMachineOptions
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
			parsed1, err := adapters.ParseNumaNode(tt.input)
			require.NoError(t, err, "First parse should not error")
			require.NotNil(t, parsed1, "First parse should return non-nil NumaNode")

			// Step 2: Convert back to string
			serialized := adapters.ToProxmoxNumaString(*parsed1)
			require.NotEmpty(t, serialized, "Serialization should produce non-empty string")

			// Step 3: Parse the serialized string again
			parsed2, err := adapters.ParseNumaNode(serialized)
			require.NoError(t, err, "Second parse should not error")
			require.NotNil(t, parsed2, "Second parse should return non-nil NumaNode")

			// Step 4: Verify both parsed structs are identical
			assert.Equal(t, parsed1.Cpus, parsed2.Cpus, "Cpus should match after round-trip")
			assert.Equal(t, parsed1.HostNodes, parsed2.HostNodes, "HostNodes should match after round-trip")
			assert.Equal(t, parsed1.Memory, parsed2.Memory, "Memory should match after round-trip")
			assert.Equal(t, parsed1.Policy, parsed2.Policy, "Policy should match after round-trip")

			// Step 5: Convert second parsed struct to string and verify it matches
			serialized2 := adapters.ToProxmoxNumaString(*parsed2)
			assert.Equal(t, serialized, serialized2, "Second serialization should match first serialization")
		})
	}
}

// TestParseCPUFromVMConfig verifies that CPU parsing from VirtualMachineConfig works correctly
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
