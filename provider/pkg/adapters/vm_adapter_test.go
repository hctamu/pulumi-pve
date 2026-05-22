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

	api "github.com/luthermonson/go-proxmox"
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

// TestToProxmoxDiskKeyConfigFlags verifies that Group A and B flag fields are
// correctly serialized into the Proxmox disk config string.
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

// TestParseDiskConfigFlags verifies that Group A and B flag fields are correctly
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
