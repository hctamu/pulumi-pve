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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/params"
	"github.com/vitorsalgado/mocha/v3/reply"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// cpuBase creates a minimal CPU config with just a type
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

// TestVMDiffCPU verifies that VM Diff detects all CPU field changes correctly (simple and complex fields)
func TestVMDiffCPU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputCPU     *proxmox.CPU
		stateCPU     *proxmox.CPU
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
					"numa-nodes": []proxmox.NumaNode{
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
					"numa-nodes": []proxmox.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(2048)}},
				},
			),
		},
		{
			name: "numa_nodes_changed_-_different_cpus", expectChange: true,
			inputCPU: cpuWith(
				"host",
				map[string]interface{}{
					"cores":      4,
					"numa-nodes": []proxmox.NumaNode{{Cpus: "0-3", Memory: testutils.Ptr(2048)}},
				},
			),
			stateCPU: cpuWith(
				"host",
				map[string]interface{}{
					"cores":      4,
					"numa-nodes": []proxmox.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(2048)}},
				},
			),
		},
		{
			name: "numa_nodes_changed_-_different_memory", expectChange: true,
			inputCPU: cpuWith(
				"host",
				map[string]interface{}{"numa-nodes": []proxmox.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(4096)}}},
			),
			stateCPU: cpuWith(
				"host",
				map[string]interface{}{"numa-nodes": []proxmox.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(2048)}}},
			),
		},
		{
			name: "numa_nodes_changed_-_different_count", expectChange: true,
			inputCPU: cpuWith(
				"host",
				map[string]interface{}{
					"numa-nodes": []proxmox.NumaNode{
						{Cpus: "0-1", Memory: testutils.Ptr(2048)},
						{Cpus: "2-3", Memory: testutils.Ptr(2048)},
					},
				},
			),
			stateCPU: cpuWith(
				"host",
				map[string]interface{}{"numa-nodes": []proxmox.NumaNode{{Cpus: "0-1", Memory: testutils.Ptr(2048)}}},
			),
		},
		{
			name: "numa_nodes_unchanged", expectChange: false,
			inputCPU: cpuWith(
				"host",
				map[string]interface{}{
					"numa-nodes": []proxmox.NumaNode{
						{Cpus: "0-1", Memory: testutils.Ptr(2048)},
						{Cpus: "2-3", Memory: testutils.Ptr(2048)},
					},
				},
			),
			stateCPU: cpuWith(
				"host",
				map[string]interface{}{
					"numa-nodes": []proxmox.NumaNode{
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
			inputCPU: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				Cores:        testutils.Ptr(4),
				Numa:         testutils.Ptr(true),
				Hidden:       testutils.Ptr(true),
				FlagsEnabled: []string{"aes"},
				NumaNodes: []proxmox.NumaNode{
					{Cpus: "0-1", Memory: testutils.Ptr(2048)},
					{Cpus: "2-3", Memory: testutils.Ptr(2048)},
				},
			},
			stateCPU: &proxmox.CPU{
				Type:   testutils.Ptr("host"),
				Cores:  testutils.Ptr(2),
				Numa:   testutils.Ptr(false),
				Hidden: testutils.Ptr(false),
			},
		},
		{
			name: "all_complex_fields_unchanged", expectChange: false,
			inputCPU: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				Numa:          testutils.Ptr(true),
				Hidden:        testutils.Ptr(true),
				HVVendorID:    testutils.Ptr("GenuineIntel"),
				PhysBits:      testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes"},
				FlagsDisabled: []string{"pcid"},
				NumaNodes: []proxmox.NumaNode{
					{Cpus: "0-1", Memory: testutils.Ptr(2048), HostNodes: testutils.Ptr("0"), Policy: testutils.Ptr("preferred")},
				},
			},
			stateCPU: &proxmox.CPU{
				Type:          testutils.Ptr("host"),
				Numa:          testutils.Ptr(true),
				Hidden:        testutils.Ptr(true),
				HVVendorID:    testutils.Ptr("GenuineIntel"),
				PhysBits:      testutils.Ptr("host"),
				FlagsEnabled:  []string{"aes"},
				FlagsDisabled: []string{"pcid"},
				NumaNodes: []proxmox.NumaNode{
					{Cpus: "0-1", Memory: testutils.Ptr(2048), HostNodes: testutils.Ptr("0"), Policy: testutils.Ptr("preferred")},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vmResource := &vm.VM{
				Client: &testutils.MockProxmoxClient{DefaultNode: "pve-node", DefaultVMID: 100},
				VMOps:  adapters.NewVMAdapter(),
			}
			req := infer.DiffRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID: "100",
				Inputs: proxmox.VMInputs{
					CPU: tt.inputCPU,
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
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

func TestCPUAnnotate(t *testing.T) {
	t.Parallel()

	// Verify the CPU type has an Annotate method by checking interface compliance
	cpu := &proxmox.CPU{}
	var _ interface {
		Annotate(infer.Annotator)
	} = cpu
}

//nolint:paralleltest // uses global env + client seam
func TestVMReadWithCPU(t *testing.T) {
	mock, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	vmID := 100
	nodeName := "pve-node"

	// Mock GET /cluster/status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			Reply(reply.OK().BodyString(
				`{"data":[{"type":"cluster","nodes":[{"name":"` + nodeName + `","status":"online"}]}]}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			Reply(reply.OK().BodyString(
				`{"data":{"node":"` + nodeName + `","status":"online"}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/status/current
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":100}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/config - VM with CPU config
	vmConfigJSON := `{"data":{"vmid":100,"name":"test-vm","cores":4,"sockets":2,"memory":4096,` +
		`"cpu":"host,flags=+aes;-pcid,hidden=1","cpulimit":2,"cpuunits":2048,"vcpus":8,"numa":1,` +
		`"numa0":"cpus=0-3,hostnodes=0,memory=2048,policy=bind","numa1":"cpus=4-7,hostnodes=1,memory=2048,policy=bind"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(vmConfigJSON)),
	).Enable()

	vmRes := &vm.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: "pve-node", DefaultVMID: 100},
		VMOps:  adapters.NewVMAdapter(),
	}
	req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID: testutils.Ptr(vmID),
			Node: &nodeName,
		},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "100", resp.ID)

	// Verify CPU configuration was read correctly
	require.NotNil(t, resp.State.CPU)
	assert.Equal(t, "host", *resp.State.CPU.Type)
	assert.Equal(t, 4, *resp.State.CPU.Cores)
	assert.Equal(t, 2, *resp.State.CPU.Sockets)
	assert.Equal(t, 2.0, *resp.State.CPU.Limit)
	assert.Equal(t, 2048, *resp.State.CPU.Units)
	assert.Equal(t, 8, *resp.State.CPU.Vcpus)
	assert.True(t, *resp.State.CPU.Numa)
	assert.True(t, *resp.State.CPU.Hidden)
	assert.Equal(t, []string{"aes"}, resp.State.CPU.FlagsEnabled)
	assert.Equal(t, []string{"pcid"}, resp.State.CPU.FlagsDisabled)

	// Verify NUMA nodes
	require.Len(t, resp.State.CPU.NumaNodes, 2)
	assert.Equal(t, "0-3", resp.State.CPU.NumaNodes[0].Cpus)
	assert.Equal(t, "0", *resp.State.CPU.NumaNodes[0].HostNodes)
	assert.Equal(t, 2048, *resp.State.CPU.NumaNodes[0].Memory)
	assert.Equal(t, "bind", *resp.State.CPU.NumaNodes[0].Policy)

	assert.Equal(t, "4-7", resp.State.CPU.NumaNodes[1].Cpus)
	assert.Equal(t, "1", *resp.State.CPU.NumaNodes[1].HostNodes)
	assert.Equal(t, 2048, *resp.State.CPU.NumaNodes[1].Memory)
	assert.Equal(t, "bind", *resp.State.CPU.NumaNodes[1].Policy)
}

//nolint:paralleltest // uses global env + client seam
func TestVMUpdateCPUSuccess(t *testing.T) {
	mock, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	vmID := 100
	nodeName := "pve-node"

	// Mock GET /cluster/status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			Reply(reply.OK().BodyString(
				`{"data":[{"type":"cluster","nodes":[{"name":"` + nodeName + `","status":"online"}]}]}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			Reply(reply.OK().BodyString(
				`{"data":{"node":"` + nodeName + `","status":"online"}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/status/current
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":100}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/config - return updated config
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(
				`{"data":{"vmid":100,"name":"test-vm","cores":4,"cpu":"host"}}`,
			)),
	).Enable()

	// Mock POST /nodes/{node}/qemu/{vmid}/config for the update
	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(`{"data":"UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:"}`)),
	).Enable()

	// Mock task status endpoint
	taskStatusJSON := `{"data":{"upid":"UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:",` +
		`"node":"pve-node","pid":1234,"pstart":0,"starttime":1699999999,"type":"qmconfig",` +
		`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
	taskStatusURL := "/nodes/pve-node/tasks/UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(taskStatusURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(taskStatusJSON)}, nil
			}),
	).Enable()

	vmRes := &vm.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: "pve-node", DefaultVMID: 100},
		VMOps:  adapters.NewVMAdapter(),
	}
	req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID:    testutils.Ptr(vmID),
			Name:    testutils.Ptr("test-vm"),
			Disks:   []*proxmox.Disk{},  // Empty disks
			EfiDisk: &proxmox.EfiDisk{}, // Empty EfiDisk to prevent removal logic
			CPU: &proxmox.CPU{
				Type:  testutils.Ptr("host"),
				Cores: testutils.Ptr(4), // Changed from 2 to 4
			},
		},
		State: proxmox.VMOutputs{
			VMInputs: proxmox.VMInputs{
				VMID:    testutils.Ptr(vmID),
				Name:    testutils.Ptr("test-vm"),
				Node:    &nodeName,
				Disks:   []*proxmox.Disk{},  // Empty disks
				EfiDisk: &proxmox.EfiDisk{}, // Empty EfiDisk
				CPU: &proxmox.CPU{
					Type:  testutils.Ptr("host"),
					Cores: testutils.Ptr(2),
				},
			},
		},
	}

	resp, err := vmRes.Update(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 4, *resp.Output.CPU.Cores)
	mock.AssertCalled(t)
}

//nolint:paralleltest // uses global env + client seam
func TestVMUpdateCPUWithNUMA(t *testing.T) {
	mock, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	vmID := 100
	nodeName := "pve-node"

	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			Reply(reply.OK().BodyString(
				`{"data":[{"type":"cluster","nodes":[{"name":"` + nodeName + `","status":"online"}]}]}`,
			)),
	).Enable()

	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			Reply(reply.OK().BodyString(
				`{"data":{"node":"` + nodeName + `","status":"online"}}`,
			)),
	).Enable()

	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":100}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/config - return updated config with NUMA
	vmConfigJSON := `{"data":{"vmid":100,"name":"test-vm","cores":4,"cpu":"host","numa":1,` +
		`"numa0":"cpus=0-1,memory=2048","numa1":"cpus=2-3,memory=2048"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(vmConfigJSON)),
	).Enable()

	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(`{"data":"UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:"}`)),
	).Enable()

	taskStatusJSON := `{"data":{"upid":"UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:",` +
		`"node":"pve-node","pid":1234,"pstart":0,"starttime":1699999999,"type":"qmconfig",` +
		`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
	taskStatusURL := "/nodes/pve-node/tasks/UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(taskStatusURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(taskStatusJSON)}, nil
			}),
	).Enable()

	vmRes := &vm.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: "pve-node", DefaultVMID: 100},
		VMOps:  adapters.NewVMAdapter(),
	}
	req := infer.UpdateRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID:    testutils.Ptr(vmID),
			Name:    testutils.Ptr("test-vm"),
			Disks:   []*proxmox.Disk{},  // Empty disks
			EfiDisk: &proxmox.EfiDisk{}, // Empty EfiDisk to prevent removal logic
			CPU: &proxmox.CPU{
				Type:  testutils.Ptr("host"),
				Cores: testutils.Ptr(4),
				Numa:  testutils.Ptr(true), // Enable NUMA
				NumaNodes: []proxmox.NumaNode{
					{
						Cpus:   "0-1",
						Memory: testutils.Ptr(2048),
					},
					{
						Cpus:   "2-3",
						Memory: testutils.Ptr(2048),
					},
				},
			},
		},
		State: proxmox.VMOutputs{
			VMInputs: proxmox.VMInputs{
				VMID:    testutils.Ptr(vmID),
				Name:    testutils.Ptr("test-vm"),
				Node:    &nodeName,
				Disks:   []*proxmox.Disk{},  // Empty disks
				EfiDisk: &proxmox.EfiDisk{}, // Empty EfiDisk
				CPU: &proxmox.CPU{
					Type:  testutils.Ptr("host"),
					Cores: testutils.Ptr(4),
				},
			},
		},
	}

	resp, err := vmRes.Update(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, *resp.Output.CPU.Numa)
	require.Len(t, resp.Output.CPU.NumaNodes, 2)
	assert.Equal(t, "0-1", resp.Output.CPU.NumaNodes[0].Cpus)
	assert.Equal(t, "2-3", resp.Output.CPU.NumaNodes[1].Cpus)
	mock.AssertCalled(t)
}

//nolint:paralleltest // uses global env + client seam
func TestVMCreateWithCPU(t *testing.T) {
	mock, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	nodeName := "pve-node"

	// Mock cluster status - use Repeat for multiple calls
	clusterStatusJSON := `{"data":[{"type":"cluster","quorate":1,"nodes":1},` +
		`{"type":"node","name":"` + nodeName + `","online":1}]}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(clusterStatusJSON)}, nil
			}).
			Repeat(10),
	).Enable()

	// Mock node status
	nodeStatusJSON := `{"data":{"status":"online"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(nodeStatusJSON)}, nil
			}),
	).Enable()

	// Mock VM creation POST - verify it includes CPU settings
	var capturedBody string
	createResponseJSON := `{"data":"UPID:pve-node:00001234:00000000:00000000:qmcreate:100:root@pam:"}`
	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				body, _ := io.ReadAll(r.Body)
				capturedBody = string(body)
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(createResponseJSON)}, nil
			}),
	).Enable()

	// Mock create task status
	createTaskStatusJSON := `{"data":{"upid":"UPID:pve-node:00001234:00000000:00000000:qmcreate:100:root@pam:",` +
		`"node":"pve-node","pid":1234,"pstart":0,"starttime":1699999999,"type":"qmcreate",` +
		`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
	createTaskURL := "/nodes/pve-node/tasks/UPID:pve-node:00001234:00000000:00000000:qmcreate:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(createTaskURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(createTaskStatusJSON)}, nil
			}),
	).Enable()

	// Mock new VM status
	newVMStatusJSON := `{"data":{"status":"stopped","vmid":100}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(newVMStatusJSON)}, nil
			}),
	).Enable()

	// Mock new VM config - return with CPU config
	newVMConfigJSON := `{"data":{"vmid":100,"name":"test-vm","cores":8,"sockets":2,"memory":4096,` +
		`"cpu":"host,flags=+aes","cpulimit":4,"cpuunits":2048,"vcpus":16,"numa":1,` +
		`"numa0":"cpus=0-7,memory=2048","numa1":"cpus=8-15,memory=2048"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(newVMConfigJSON)}, nil
			}),
	).Enable()

	vmRes := &vm.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: "pve-node", DefaultVMID: 100},
		VMOps:  adapters.NewVMAdapter(),
	}
	req := infer.CreateRequest[proxmox.VMInputs]{
		Name: "test-vm",
		Inputs: proxmox.VMInputs{
			Name: testutils.Ptr("test-vm"),
			Node: &nodeName,
			CPU: &proxmox.CPU{
				Type:         testutils.Ptr("host"),
				Cores:        testutils.Ptr(8),
				Sockets:      testutils.Ptr(2),
				Limit:        testutils.Ptr(4.0),
				Units:        testutils.Ptr(2048),
				Vcpus:        testutils.Ptr(16),
				Numa:         testutils.Ptr(true),
				FlagsEnabled: []string{"aes"},
				NumaNodes: []proxmox.NumaNode{
					{Cpus: "0-7", Memory: testutils.Ptr(2048)},
					{Cpus: "8-15", Memory: testutils.Ptr(2048)},
				},
			},
			Disks: []*proxmox.Disk{},
		},
	}

	resp, err := vmRes.Create(context.Background(), req)
	require.NoError(t, err)

	// Verify CPU was created with correct settings
	require.NotNil(t, resp.Output.CPU)
	assert.Equal(t, "host", *resp.Output.CPU.Type)
	assert.Equal(t, 8, *resp.Output.CPU.Cores)
	assert.Equal(t, 2, *resp.Output.CPU.Sockets)
	assert.Equal(t, 4.0, *resp.Output.CPU.Limit)
	assert.Equal(t, 2048, *resp.Output.CPU.Units)
	assert.Equal(t, 16, *resp.Output.CPU.Vcpus)
	assert.True(t, *resp.Output.CPU.Numa)
	assert.Equal(t, []string{"aes"}, resp.Output.CPU.FlagsEnabled)
	require.Len(t, resp.Output.CPU.NumaNodes, 2)

	// Verify the request body contains CPU parameters (JSON format)
	assert.Contains(t, capturedBody, `"cpu":"host`)
	assert.Contains(t, capturedBody, `"cores":8`)
	assert.Contains(t, capturedBody, `"sockets":2`)
	assert.Contains(t, capturedBody, `"numa":1`)
	mock.AssertCalled(t)
}
