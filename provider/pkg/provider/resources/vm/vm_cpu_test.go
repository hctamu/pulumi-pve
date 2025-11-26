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
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			description: "Empty CPU config should return nil without error",
		},
		{
			name:  "simple CPU type - host",
			input: "host",
			expected: &vm.CPU{
				Type: testutils.Ptr("host"),
			},
			expectError: false,
			description: "Simple host CPU type without additional flags",
		},
		{
			name:  "simple CPU type - kvm64",
			input: "kvm64",
			expected: &vm.CPU{
				Type: testutils.Ptr("kvm64"),
			},
			expectError: false,
			description: "Simple kvm64 CPU type",
		},
		{
			name:  "CPU type with cputype key",
			input: "cputype=x86-64-v2-AES",
			expected: &vm.CPU{
				Type: testutils.Ptr("x86-64-v2-AES"),
			},
			expectError: false,
			description: "CPU type specified with cputype= key",
		},
		{
			name:  "CPU with single enabled flag",
			input: "host,flags=+aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
			description: "CPU with one enabled flag using + prefix",
		},
		{
			name:  "CPU with single disabled flag",
			input: "host,flags=-pcid",
			expected: &vm.CPU{
				Type:          testutils.Ptr("host"),
				FlagsDisabled: []string{"pcid"},
			},
			expectError: false,
			description: "CPU with one disabled flag using - prefix",
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
			description: "CPU with multiple flags, both enabled and disabled",
		},
		{
			name:  "CPU with flag without prefix (treated as enabled)",
			input: "host,flags=aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
			description: "Flag without +/- prefix defaults to enabled",
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
			description: "Flags with and without prefixes",
		},
		{
			name:  "CPU with hidden=1",
			input: "host,hidden=1",
			expected: &vm.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(true),
			},
			expectError: false,
			description: "Hidden flag set to true (1)",
		},
		{
			name:  "CPU with hidden=0",
			input: "host,hidden=0",
			expected: &vm.CPU{
				Type:   testutils.Ptr("host"),
				Hidden: testutils.Ptr(false),
			},
			expectError: false,
			description: "Hidden flag set to false (0)",
		},
		{
			name:  "CPU with hv-vendor-id",
			input: "host,hv-vendor-id=AuthenticAMD",
			expected: &vm.CPU{
				Type:       testutils.Ptr("host"),
				HVVendorID: testutils.Ptr("AuthenticAMD"),
			},
			expectError: false,
			description: "HV-Vendor-ID for AMD CPU",
		},
		{
			name:  "CPU with phys-bits",
			input: "host,phys-bits=40",
			expected: &vm.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("40"),
			},
			expectError: false,
			description: "Physical address bits specified",
		},
		{
			name:  "CPU with phys-bits=host",
			input: "host,phys-bits=host",
			expected: &vm.CPU{
				Type:     testutils.Ptr("host"),
				PhysBits: testutils.Ptr("host"),
			},
			expectError: false,
			description: "Physical bits set to host value",
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
			description: "Complex CPU config with all supported fields",
		},
		{
			name:  "empty flags value",
			input: "host,flags=",
			expected: &vm.CPU{
				Type: testutils.Ptr("host"),
			},
			expectError: false,
			description: "Empty flags value should not add any flags",
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
			description: "Empty flag segments (;;) should be skipped",
		},
		{
			name:  "multiple commas in config",
			input: "host,,flags=+aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
			description: "Empty segments between commas should be ignored",
		},
		{
			name:  "unknown keys are ignored",
			input: "host,unknown=value,flags=+aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
			description: "Unknown configuration keys should be silently ignored",
		},
		{
			name:  "segment without equals sign after first",
			input: "host,someflag,flags=+aes",
			expected: &vm.CPU{
				Type:         testutils.Ptr("host"),
				FlagsEnabled: []string{"aes"},
			},
			expectError: false,
			description: "Non-key=value segments after first are ignored",
		},
		{
			name:  "hidden with invalid value",
			input: "host,hidden=invalid",
			expected: &vm.CPU{
				Type: testutils.Ptr("host"),
			},
			expectError: false,
			description: "Invalid hidden value (not 0 or 1) is ignored",
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
			description: "Empty NUMA node config should return nil without error",
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
			description: "Complete NUMA node with all fields",
		},
		{
			name:  "only required cpus field",
			input: "cpus=0-1",
			expected: &vm.NumaNode{
				Cpus: "0-1",
			},
			expectError: false,
			description: "NUMA node with only required cpus field",
		},
		{
			name:  "cpus with hostnodes",
			input: "cpus=0-3,hostnodes=0-1",
			expected: &vm.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0-1"),
			},
			expectError: false,
			description: "NUMA node with cpus and hostnodes",
		},
		{
			name:  "cpus with memory",
			input: "cpus=0-3,memory=4096",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(4096),
			},
			expectError: false,
			description: "NUMA node with cpus and memory",
		},
		{
			name:  "cpus with policy",
			input: "cpus=0-3,policy=preferred",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("preferred"),
			},
			expectError: false,
			description: "NUMA node with cpus and policy",
		},
		{
			name:  "single CPU core",
			input: "cpus=0",
			expected: &vm.NumaNode{
				Cpus: "0",
			},
			expectError: false,
			description: "NUMA node with single CPU core",
		},
		{
			name:  "complex CPU range format",
			input: "cpus=0-3;5-7",
			expected: &vm.NumaNode{
				Cpus: "0-3;5-7",
			},
			expectError: false,
			description: "NUMA node with multiple CPU ranges using semicolon",
		},
		{
			name:  "multiple host nodes",
			input: "cpus=0-7,hostnodes=0-2",
			expected: &vm.NumaNode{
				Cpus:      "0-7",
				HostNodes: testutils.Ptr("0-2"),
			},
			expectError: false,
			description: "NUMA node with multiple host nodes range",
		},
		{
			name:  "policy interleave",
			input: "cpus=0-3,policy=interleave",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Policy: testutils.Ptr("interleave"),
			},
			expectError: false,
			description: "NUMA node with interleave policy",
		},
		{
			name:  "empty segments ignored",
			input: "cpus=0-3,,memory=2048",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(2048),
			},
			expectError: false,
			description: "Empty segments between commas should be ignored",
		},
		{
			name:  "unknown keys ignored",
			input: "cpus=0-3,unknown=value,memory=2048",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(2048),
			},
			expectError: false,
			description: "Unknown configuration keys should be silently ignored",
		},
		{
			name:  "segments without equals sign ignored",
			input: "cpus=0-3,someflag,memory=2048",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(2048),
			},
			expectError: false,
			description: "Segments without = should be ignored",
		},
		{
			name:        "missing required cpus field",
			input:       "memory=2048,policy=bind",
			expected:    nil,
			expectError: true,
			description: "Missing required cpus field should return error",
		},
		{
			name:        "empty cpus value",
			input:       "cpus=,memory=2048",
			expected:    nil,
			expectError: true,
			description: "Empty cpus value should return error",
		},
		{
			name:        "invalid memory value - non-numeric",
			input:       "cpus=0-3,memory=invalid",
			expected:    nil,
			expectError: true,
			description: "Non-numeric memory value should return error",
		},
		{
			name:        "invalid memory value - float",
			input:       "cpus=0-3,memory=2048.5",
			expected:    nil,
			expectError: true,
			description: "Float memory value should return error",
		},
		{
			name:  "negative memory value",
			input: "cpus=0-3,memory=-1024",
			expected: &vm.NumaNode{
				Cpus:   "0-3",
				Memory: testutils.Ptr(-1024),
			},
			expectError: false,
			description: "Negative memory value is parsed (no validation in parser)",
		},
		{
			name:  "large memory value",
			input: "cpus=0-15,memory=65536",
			expected: &vm.NumaNode{
				Cpus:   "0-15",
				Memory: testutils.Ptr(65536),
			},
			expectError: false,
			description: "Large memory value should be parsed correctly",
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
			description: "Field order should not matter",
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
