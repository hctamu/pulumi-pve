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
		name     string
		input    string
		expected *proxmox.CPU
	}{
		{name: "empty", input: "", expected: nil},
		{name: "type", input: "host", expected: &proxmox.CPU{Type: testutils.Ptr("host")}},
		{
			name:  "all fields",
			input: "host,flags=+aes;-pcid,hidden=1,hv-vendor-id=GenuineIntel,phys-bits=42",
			expected: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes"},
				FlagsDisabled: []string{"pcid"},
				Hidden:        testutils.Ptr(true),
				HVVendorID:    testutils.Ptr("GenuineIntel"),
				PhysBits:      testutils.Ptr("42"),
			},
		},
		{name: "cputype", input: "cputype=x86-64-v2-AES", expected: &proxmox.CPU{Type: testutils.Ptr("x86-64-v2-AES")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual, err := adapters.ParseCPU(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestParseNumaNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		expected  *proxmox.NumaNode
		wantError bool
	}{
		{name: "empty", input: "", expected: nil},
		{
			name:  "all fields",
			input: "cpus=0-3,hostnodes=0,memory=2048,policy=bind",
			expected: &proxmox.NumaNode{
				Cpus:      "0-3",
				HostNodes: testutils.Ptr("0"),
				Memory:    testutils.Ptr(2048),
				Policy:    testutils.Ptr("bind"),
			},
		},
		{name: "missing cpus", input: "memory=2048", wantError: true},
		{name: "invalid memory", input: "cpus=0-3,memory=invalid", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual, err := adapters.ParseNumaNode(tt.input)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestPublicVMSerializers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		actual   string
		expected string
	}{
		{
			name: "CPU",
			actual: adapters.CPUToProxmoxString(&proxmox.CPU{
				Type:          testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes"},
				FlagsDisabled: []string{"pcid"},
				Hidden:        testutils.Ptr(false),
			}),
			expected: "host,flags=+aes;-pcid,hidden=0",
		},
		{
			name: "NUMA",
			actual: adapters.ToProxmoxNumaString(proxmox.NumaNode{
				Cpus: "0-3", HostNodes: testutils.Ptr("0"), Memory: testutils.Ptr(2048), Policy: testutils.Ptr("bind"),
			}),
			expected: "cpus=0-3,hostnodes=0,memory=2048,policy=bind",
		},
		{
			name: "disk flags bandwidth and misc",
			actual: func() string {
				_, config := adapters.ToProxmoxDiskKeyConfig(proxmox.Disk{
					Interface: "scsi0",
					DiskBase:  proxmox.DiskBase{Storage: "local-lvm"},
					Size:      20,
					Cache:     testutils.Ptr("writeback"),
					IOThread:  testutils.Ptr(true),
					Bandwidth: &proxmox.DiskBandwidth{MBpsRd: testutils.Ptr(100.5), IOPSWr: testutils.Ptr(500)},
					Serial:    testutils.Ptr("disk-serial"),
					ScsiBlock: testutils.Ptr(false),
				})
				return config
			}(),
			expected: "file=local-lvm:20,size=20,cache=writeback,iothread=1,mbps_rd=100.5,iops_wr=500,serial=disk-serial,scsiblock=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.actual)
		})
	}
}

func TestPublicDiskParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{
			name:   "flags bandwidth and misc",
			config: "local-lvm:vm-100-disk-0,size=20G,cache=writeback,iothread=1,mbps_rd=100.5,iops_wr=500,serial=disk-serial,scsiblock=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			disk := proxmox.Disk{Interface: "scsi0"}
			require.NoError(t, adapters.ParseDiskConfig(&disk, tt.config))
			assert.Equal(t, "local-lvm", disk.Storage)
			assert.Equal(t, testutils.Ptr("vm-100-disk-0"), disk.FileID)
			assert.Equal(t, 20, disk.Size)
			assert.Equal(t, testutils.Ptr("writeback"), disk.Cache)
			assert.Equal(t, testutils.Ptr(true), disk.IOThread)
			require.NotNil(t, disk.Bandwidth)
			assert.Equal(t, testutils.Ptr(100.5), disk.Bandwidth.MBpsRd)
			assert.Equal(t, testutils.Ptr(500), disk.Bandwidth.IOPSWr)
			assert.Equal(t, testutils.Ptr("disk-serial"), disk.Serial)
			assert.Equal(t, testutils.Ptr(false), disk.ScsiBlock)
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
		{name: "missing node", inputs: proxmox.VMInputs{VMID: &vmID}, wantErrContain: "inputs.Node"},
		{name: "missing VMID", inputs: proxmox.VMInputs{Node: &nodeName}, wantErrContain: "inputs.VMID"},
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
