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

// Package adapters_test contains HTTP-level tests that verify VMAdapter
// methods send the correct requests to the Proxmox API.
package adapters_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

const (
	// testUpidCreate/testUpidConfig are Proxmox UPID strings formatted as:
	// UPID:{node}:{pid}:{pstart}:{starttime}:{type}:{id}:{user}:
	testUpidCreate = "UPID:pve-node:00001234:00000000:60000000:qmcreate:100:root@pam:"
	testUpidConfig = "UPID:pve-node:00005678:00000000:60000001:qmconfig:100:root@pam:"
)

// newVMHTTPMockServer creates a httptest.Server that handles the full set of
// Proxmox API endpoints exercised by VMAdapter.CreateVM and UpdateConfig.
//
// Design requirement — task status responses MUST include "upid" and "node":
//
// go-proxmox's Task.UnmarshalJSON uses copier.Copy to update the task struct
// after each Ping. copier.Copy overwrites ALL exported fields from the JSON
// response. If "upid" or "node" are absent the fields are zeroed, causing the
// next Ping to construct the path "/nodes//tasks//status" and then panic when
// it tries to assign t.client = tmp.client with tmp == nil (NewTask("") ≡ nil).
func newVMHTTPMockServer(
	t *testing.T,
	nodeName string,
	vmID int,
	tagsFunc func() string,
	captureFunc func(method, path string, body map[string]interface{}),
) *httptest.Server {
	t.Helper()
	enc := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	nodeStatusPath := "/nodes/" + nodeName + "/status"
	createVMPath := "/nodes/" + nodeName + "/qemu"
	vmConfigPath := "/nodes/" + nodeName + "/qemu/100/config"

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bodyData map[string]interface{}
		if r.Body != nil {
			if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
				_ = json.Unmarshal(raw, &bodyData)
			}
		}
		captureFunc(r.Method, r.URL.Path, bodyData)

		switch {
		// Task status: /nodes/{node}/tasks/{upid}/status
		// Echo back upid and node so Task.UnmarshalJSON / copier.Copy preserve them.
		case strings.Contains(r.URL.Path, "/tasks/") && strings.HasSuffix(r.URL.Path, "/status"):
			parts := strings.Split(r.URL.Path, "/")
			upid, taskNode := "", nodeName
			if len(parts) >= 5 {
				upid = parts[4]
			}
			enc(w, map[string]interface{}{
				"data": map[string]interface{}{
					"upid":       upid,
					"node":       taskNode,
					"status":     "stopped",
					"exitstatus": "OK",
				},
			})

		// Cluster status for FindVirtualMachine
		case r.URL.Path == "/cluster/status":
			enc(w, map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"type": "cluster", "name": "test", "id": "cluster",
						"version": 1, "quorate": 1, "nodes": 1,
					},
					map[string]interface{}{
						"type": "node", "name": nodeName, "nodeid": 1,
						"id": "node/" + nodeName, "online": 1, "ip": "127.0.0.1", "local": 1,
					},
				},
			})

		// Node status
		case r.URL.Path == nodeStatusPath && r.Method == http.MethodGet:
			enc(w, map[string]interface{}{
				"data": map[string]interface{}{"node": nodeName, "status": "online", "maxcpu": 4},
			})

		// VM config GET
		case r.URL.Path == vmConfigPath && r.Method == http.MethodGet:
			enc(w, map[string]interface{}{
				"data": map[string]interface{}{
					"name": "test-vm", "memory": 2048, "vmid": "100", "tags": tagsFunc(),
				},
			})

		// VM status/current
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			enc(w, map[string]interface{}{
				"data": map[string]interface{}{"vmid": vmID, "name": "test-vm", "status": "running"},
			})

		// Create VM POST
		case r.URL.Path == createVMPath && r.Method == http.MethodPost:
			enc(w, map[string]interface{}{"data": testUpidCreate})

		// Update VM config POST
		case r.URL.Path == vmConfigPath && r.Method == http.MethodPost:
			enc(w, map[string]interface{}{"data": testUpidConfig})

		// Delete VM
		case strings.HasSuffix(r.URL.Path, "/qemu/100") && r.Method == http.MethodDelete:
			enc(w, map[string]interface{}{"data": testUpidConfig})

		default:
			w.WriteHeader(http.StatusNotFound)
			enc(w, map[string]interface{}{"data": nil})
		}
	}))
}

// TestVMAdapterCreateVMSendsTags verifies that VMAdapter.CreateVM sends the
// tags field in the POST body to /nodes/{node}/qemu, joining the tags slice
// with a semicolon delimiter as expected by the Proxmox API.
func TestVMAdapterCreateVMSendsTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tags         []string
		expectedBody string
	}{
		{
			name:         "single tag sent correctly",
			tags:         []string{"prod"},
			expectedBody: "prod",
		},
		{
			name:         "multiple tags joined with semicolon",
			tags:         []string{"prod", "web", "frontend"},
			expectedBody: "prod;web;frontend",
		},
		{
			name:         "nil tags sends empty string",
			tags:         nil,
			expectedBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const nodeName = "pve-node"
			const vmID = 100

			var mu sync.Mutex
			var capturedCreateBody map[string]interface{}

			server := newVMHTTPMockServer(t, nodeName, vmID,
				func() string { return "" },
				func(method, path string, body map[string]interface{}) {
					if path == "/nodes/"+nodeName+"/qemu" && method == http.MethodPost {
						mu.Lock()
						capturedCreateBody = body
						mu.Unlock()
					}
				},
			)
			defer server.Close()

			node := nodeName
			id := vmID
			inputs := proxmox.VMInputs{
				Name: testutils.Ptr("test-vm"),
				Node: &node,
				VMID: &id,
				Tags: tt.tags,
			}

			cfg := &config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			}
			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			vmAdapter := adapters.NewVMAdapter(proxmoxAdapter)
			err = vmAdapter.CreateVM(context.Background(), inputs)
			require.NoError(t, err)

			mu.Lock()
			defer mu.Unlock()

			require.NotNil(t, capturedCreateBody, "expected POST to /nodes/%s/qemu to be captured", nodeName)
			gotTags, ok := capturedCreateBody["tags"]
			require.True(t, ok, "expected 'tags' key in POST body")
			assert.Equal(t, tt.expectedBody, gotTags)
		})
	}
}

// TestVMAdapterUpdateConfigSendsDiffTags verifies that VMAdapter.UpdateConfig sends
// only the changed tags value in the POST body to /nodes/{node}/qemu/{vmid}/config.
func TestVMAdapterUpdateConfigSendsDiffTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		currentTags       []string
		newTags           []string
		expectConfigPost  bool
		expectedTagsValue string
	}{
		{
			name:              "tags changed - new value sent",
			currentTags:       []string{"prod"},
			newTags:           []string{"staging"},
			expectConfigPost:  true,
			expectedTagsValue: "staging",
		},
		{
			name:              "multiple tags replacing single tag",
			currentTags:       []string{"prod"},
			newTags:           []string{"prod", "web"},
			expectConfigPost:  true,
			expectedTagsValue: "prod;web",
		},
		{
			name:             "tags unchanged - no config POST",
			currentTags:      []string{"prod", "web"},
			newTags:          []string{"prod", "web"},
			expectConfigPost: false,
		},
		{
			name:              "tags cleared",
			currentTags:       []string{"prod"},
			newTags:           nil,
			expectConfigPost:  true,
			expectedTagsValue: "",
		},
		{
			name:             "same tags different order - no config POST",
			currentTags:      []string{"prod", "web"},
			newTags:          []string{"web", "prod"},
			expectConfigPost: false,
		},
		{
			name:             "three tags reordered - no config POST",
			currentTags:      []string{"aaa", "mmm", "zzz"},
			newTags:          []string{"zzz", "aaa", "mmm"},
			expectConfigPost: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const nodeName = "pve-node"
			const vmID = 100

			var mu sync.Mutex
			var capturedConfigBody map[string]interface{}
			var configPostCalled bool

			server := newVMHTTPMockServer(t, nodeName, vmID,
				func() string { return "prod" },
				func(method, path string, body map[string]interface{}) {
					if path == "/nodes/"+nodeName+"/qemu/100/config" && method == http.MethodPost {
						mu.Lock()
						capturedConfigBody = body
						configPostCalled = true
						mu.Unlock()
					}
				},
			)
			defer server.Close()

			cfg := &config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			}
			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			node := nodeName
			id := vmID
			newInputs := proxmox.VMInputs{
				Name: testutils.Ptr("test-vm"),
				Node: &node,
				VMID: &id,
				Tags: tt.newTags,
			}
			stateInputs := proxmox.VMInputs{
				Name: testutils.Ptr("test-vm"),
				Node: &node,
				VMID: &id,
				Tags: tt.currentTags,
			}

			vmAdapter := adapters.NewVMAdapter(proxmoxAdapter)
			err = vmAdapter.UpdateConfig(context.Background(), id, &node, newInputs, stateInputs)
			require.NoError(t, err)

			mu.Lock()
			defer mu.Unlock()

			if !tt.expectConfigPost {
				assert.False(t, configPostCalled, "expected no config POST when tags unchanged")
				return
			}

			require.True(t, configPostCalled, "expected POST to /nodes/%s/qemu/100/config", nodeName)
			require.NotNil(t, capturedConfigBody)
			gotTags, ok := capturedConfigBody["tags"]
			require.True(t, ok, "expected 'tags' key in config POST body")
			assert.Equal(t, tt.expectedTagsValue, gotTags)
		})
	}
}

// ---------------------------------------------------------------------------
// Extended mock server for broader VM adapter HTTP tests
// ---------------------------------------------------------------------------

// vmHTTPCapture collects HTTP requests for later assertions.
type vmHTTPCapture struct {
	mu       sync.Mutex
	requests []vmCapturedReq
}

type vmCapturedReq struct {
	method string
	path   string
	body   map[string]interface{}
}

func (capture *vmHTTPCapture) add(method, path string, body map[string]interface{}) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.requests = append(capture.requests, vmCapturedReq{method: method, path: path, body: body})
}

func (capture *vmHTTPCapture) find(method, pathSuffix string) *vmCapturedReq {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	for i := range capture.requests {
		if capture.requests[i].method == method && strings.HasSuffix(capture.requests[i].path, pathSuffix) {
			return &capture.requests[i]
		}
	}
	return nil
}

// newExtendedVMMockServer creates a httptest.Server that handles the full Proxmox
// VM API surface, including clone, resize, unlink, and configurable GET /config data.
// configData returns the fields to include in GET /config responses.
func newExtendedVMMockServer(
	t *testing.T,
	nodeName string,
	vmIDStr string,
	configData func() map[string]interface{},
	capture *vmHTTPCapture,
) *httptest.Server {
	t.Helper()

	enc := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	data := func(w http.ResponseWriter, v interface{}) { enc(w, map[string]interface{}{"data": v}) }
	upid := func(taskType string) string {
		return fmt.Sprintf("UPID:%s:00001234:00000000:60000000:%s:%s:root@pam:", nodeName, taskType, vmIDStr)
	}

	nodeStatusPath := "/nodes/" + nodeName + "/status"
	createVMPath := "/nodes/" + nodeName + "/qemu"
	vmBasePath := "/nodes/" + nodeName + "/qemu/" + vmIDStr
	vmConfigPath := vmBasePath + "/config"

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bodyData map[string]interface{}
		if r.Body != nil {
			if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
				_ = json.Unmarshal(raw, &bodyData)
			}
		}
		capture.add(r.Method, r.URL.Path, bodyData)

		switch {
		// Task status — echo upid+node so copier.Copy doesn't zero them.
		case strings.Contains(r.URL.Path, "/tasks/") && strings.HasSuffix(r.URL.Path, "/status"):
			parts := strings.Split(r.URL.Path, "/")
			taskUpid := ""
			if len(parts) >= 5 {
				taskUpid = parts[4]
			}
			data(w, map[string]interface{}{
				"upid": taskUpid, "node": nodeName, "status": "stopped", "exitstatus": "OK",
			})

		// Cluster status — required by FindVirtualMachine.
		case r.URL.Path == "/cluster/status":
			data(w, []interface{}{
				map[string]interface{}{
					"type": "cluster", "name": "test", "id": "cluster",
					"version": 1, "quorate": 1, "nodes": 1,
				},
				map[string]interface{}{
					"type": "node", "name": nodeName, "nodeid": 1,
					"id": "node/" + nodeName, "online": 1, "ip": "127.0.0.1", "local": 1,
				},
			})

		// Node status.
		case r.URL.Path == nodeStatusPath && r.Method == http.MethodGet:
			data(w, map[string]interface{}{"node": nodeName, "status": "online", "maxcpu": 4})

		// VM status/current — needed by VirtualMachine() / findVM.
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			data(w, map[string]interface{}{"vmid": vmIDStr, "name": "test-vm", "status": "running"})

		// VM config GET — configurable.
		case r.URL.Path == vmConfigPath && r.Method == http.MethodGet:
			fields := configData()
			fields["vmid"] = vmIDStr
			if _, ok := fields["name"]; !ok {
				fields["name"] = "test-vm"
			}
			data(w, fields)

		// Create VM POST.
		case r.URL.Path == createVMPath && r.Method == http.MethodPost:
			data(w, upid("qmcreate"))

		// Clone POST.
		case strings.HasSuffix(r.URL.Path, "/clone") && r.Method == http.MethodPost:
			data(w, upid("qmclone"))

		// Config POST (update or apply).
		case r.URL.Path == vmConfigPath && r.Method == http.MethodPost:
			data(w, upid("qmconfig"))

		// Resize PUT.
		case strings.HasSuffix(r.URL.Path, "/resize") && r.Method == http.MethodPut:
			data(w, upid("qmresize"))

		// Unlink PUT.
		case strings.HasSuffix(r.URL.Path, "/unlink") && r.Method == http.MethodPut:
			data(w, upid("qmunlink"))

		// Delete VM.
		case r.URL.Path == vmBasePath && r.Method == http.MethodDelete:
			data(w, upid("qmdestroy"))

		default:
			w.WriteHeader(http.StatusNotFound)
			data(w, nil)
		}
	}))
}

// newConnectedVMAdapter creates a connected VMAdapter for the given mock server URL.
func newConnectedVMAdapter(t *testing.T, serverURL string) *adapters.VMAdapter {
	t.Helper()
	cfg := &config.Config{PveURL: serverURL, PveUser: "test@pam", PveToken: "test-token"}
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
	require.NoError(t, proxmoxAdapter.Connect(context.Background()))
	return adapters.NewVMAdapter(proxmoxAdapter)
}

// ---------------------------------------------------------------------------
// Phase 2: CreateVM full field tests
// ---------------------------------------------------------------------------

// TestVMAdapterCreateVMSendsAllFields verifies that CreateVM sends all scalar
// fields in the POST body to /nodes/{node}/qemu.
func TestVMAdapterCreateVMSendsAllFields(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	id := vmID
	mem := 4096
	desc := "test description"
	autostart := 1
	balloon := 1024
	ostype := "l26"
	machine := "q35"
	hotplug := "network,disk"
	template := 1

	inputs := proxmox.VMInputs{
		Name:        testutils.Ptr("full-vm"),
		Node:        &node,
		VMID:        &id,
		Memory:      &mem,
		Description: &desc,
		Autostart:   &autostart,
		Balloon:     &balloon,
		OSType:      &ostype,
		Machine:     &machine,
		Hotplug:     &hotplug,
		Template:    &template,
		Tags:        []string{"prod", "web"},
		Disks:       []*proxmox.Disk{},
	}

	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.CreateVM(context.Background(), inputs)
	require.NoError(t, err)

	req := capture.find(http.MethodPost, "/nodes/"+nodeName+"/qemu")
	require.NotNil(t, req, "expected POST to /nodes/%s/qemu", nodeName)

	assert.Equal(t, "full-vm", req.body["name"])
	assert.Equal(t, float64(4096), req.body["memory"])
	assert.Equal(t, "test description", req.body["description"])
	assert.Equal(t, float64(1), req.body["autostart"])
	assert.Equal(t, float64(1024), req.body["balloon"])
	assert.Equal(t, "l26", req.body["ostype"])
	assert.Equal(t, "q35", req.body["machine"])
	assert.Equal(t, "network,disk", req.body["hotplug"])
	assert.Equal(t, float64(1), req.body["template"])
	assert.Equal(t, "prod;web", req.body["tags"])
}

// TestVMAdapterCreateVMSendsCPU verifies that CreateVM sends CPU fields
// (cpu type string, cores, sockets, cpulimit, cpuunits, vcpus, numa) in the POST body.
func TestVMAdapterCreateVMSendsCPU(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	id := vmID
	cores := 4
	sockets := 2
	limit := 3.0
	units := 2048
	vcpus := 3
	numa := true

	inputs := proxmox.VMInputs{
		Name: testutils.Ptr("cpu-vm"),
		Node: &node,
		VMID: &id,
		CPU: &proxmox.CPU{
			Type:          testutils.Ptr("EPYC-v3"),
			FlagsEnabled:  []string{"virt-ssbd"},
			FlagsDisabled: []string{"spec-ctrl"},
			Hidden:        testutils.Ptr(true),
			HVVendorID:    testutils.Ptr("AuthenticAMD"),
			PhysBits:      testutils.Ptr("48"),
			Cores:         &cores,
			Sockets:       &sockets,
			Limit:         &limit,
			Units:         &units,
			Vcpus:         &vcpus,
			Numa:          &numa,
			NumaNodes: []proxmox.NumaNode{
				{Cpus: "0-1", HostNodes: testutils.Ptr("0"), Memory: testutils.Ptr(2048), Policy: testutils.Ptr("preferred")},
			},
		},
		Disks: []*proxmox.Disk{},
	}

	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.CreateVM(context.Background(), inputs)
	require.NoError(t, err)

	req := capture.find(http.MethodPost, "/nodes/"+nodeName+"/qemu")
	require.NotNil(t, req, "expected POST to /nodes/%s/qemu", nodeName)

	// CPU type string includes flags, hidden, hv-vendor-id, phys-bits
	cpuStr, ok := req.body["cpu"].(string)
	require.True(t, ok, "expected cpu to be a string")
	assert.Contains(t, cpuStr, "EPYC-v3")
	assert.Contains(t, cpuStr, "+virt-ssbd")
	assert.Contains(t, cpuStr, "-spec-ctrl")
	assert.Contains(t, cpuStr, "hidden=1")
	assert.Contains(t, cpuStr, "hv-vendor-id=AuthenticAMD")
	assert.Contains(t, cpuStr, "phys-bits=48")

	assert.Equal(t, float64(4), req.body["cores"])
	assert.Equal(t, float64(2), req.body["sockets"])
	assert.Equal(t, float64(3.0), req.body["cpulimit"])
	assert.Equal(t, float64(2048), req.body["cpuunits"])
	assert.Equal(t, float64(3), req.body["vcpus"])
	assert.Equal(t, float64(1), req.body["numa"])

	numa0, ok := req.body["numa0"].(string)
	require.True(t, ok, "expected numa0 to be a string")
	assert.Contains(t, numa0, "cpus=0-1")
	assert.Contains(t, numa0, "hostnodes=0")
	assert.Contains(t, numa0, "memory=2048")
	assert.Contains(t, numa0, "policy=preferred")
}

// TestVMAdapterCreateVMSendsDisks verifies that CreateVM sends disk options
// with the correct Proxmox format in the POST body.
func TestVMAdapterCreateVMSendsDisks(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	id := vmID
	inputs := proxmox.VMInputs{
		Name: testutils.Ptr("disk-vm"),
		Node: &node,
		VMID: &id,
		Disks: []*proxmox.Disk{
			{DiskBase: proxmox.DiskBase{Storage: "ceph-ha"}, Size: 20, Interface: "scsi0"},
			{DiskBase: proxmox.DiskBase{Storage: "ceph-ha"}, Size: 10, Interface: "sata0"},
		},
	}

	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.CreateVM(context.Background(), inputs)
	require.NoError(t, err)

	req := capture.find(http.MethodPost, "/nodes/"+nodeName+"/qemu")
	require.NotNil(t, req, "expected POST to /nodes/%s/qemu", nodeName)

	scsi0, ok := req.body["scsi0"].(string)
	require.True(t, ok, "expected scsi0 to be a string")
	assert.Contains(t, scsi0, "ceph-ha")
	assert.Contains(t, scsi0, "20")

	sata0, ok := req.body["sata0"].(string)
	require.True(t, ok, "expected sata0 to be a string")
	assert.Contains(t, sata0, "ceph-ha")
	assert.Contains(t, sata0, "10")
}

// TestVMAdapterCreateVMSendsEfiDisk verifies that CreateVM sends the efidisk0
// option with the correct Proxmox format in the POST body.
func TestVMAdapterCreateVMSendsEfiDisk(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	id := vmID
	inputs := proxmox.VMInputs{
		Name: testutils.Ptr("efi-vm"),
		Node: &node,
		VMID: &id,
		EfiDisk: &proxmox.EfiDisk{
			DiskBase:        proxmox.DiskBase{Storage: "local-lvm"},
			EfiType:         proxmox.EfiType4M,
			PreEnrolledKeys: testutils.Ptr(false),
		},
		Disks: []*proxmox.Disk{},
	}

	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.CreateVM(context.Background(), inputs)
	require.NoError(t, err)

	req := capture.find(http.MethodPost, "/nodes/"+nodeName+"/qemu")
	require.NotNil(t, req, "expected POST to /nodes/%s/qemu", nodeName)

	efi, ok := req.body["efidisk0"].(string)
	require.True(t, ok, "expected efidisk0 to be a string")
	assert.Contains(t, efi, "local-lvm")
	assert.Contains(t, efi, "efitype=4m")
	assert.Contains(t, efi, "pre-enrolled-keys=0")
}

// ---------------------------------------------------------------------------
// Phase 3: Get/Read HTTP tests
// ---------------------------------------------------------------------------

// TestVMAdapterGetReadsConfig verifies that Get returns the correct VMInputs
// by parsing the config response from the Proxmox API.
func TestVMAdapterGetReadsConfig(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} {
			return map[string]interface{}{
				"name":        "my-vm",
				"memory":      4096,
				"description": "a test vm",
				"tags":        "prod;web",
				"cpu":         "host",
				"cores":       2,
				"autostart":   1,
				"ostype":      "l26",
				"machine":     "q35",
				"hotplug":     "network,disk",
			}
		},
		&capture,
	)
	defer server.Close()

	node := nodeName
	vmAdapter := newConnectedVMAdapter(t, server.URL)
	result, err := vmAdapter.Get(context.Background(), vmID, &node, nil)
	require.NoError(t, err)

	assert.Equal(t, "my-vm", *result.Name)
	assert.Equal(t, 4096, *result.Memory)
	assert.Equal(t, "a test vm", *result.Description)
	assert.Equal(t, []string{"prod", "web"}, result.Tags)
	assert.Equal(t, "host", *result.CPU.Type)
	assert.Equal(t, 2, *result.CPU.Cores)
	assert.Equal(t, 1, *result.Autostart)
	assert.Equal(t, "l26", *result.OSType)
	assert.Equal(t, "q35", *result.Machine)
	assert.Equal(t, "network,disk", *result.Hotplug)
}

// TestVMAdapterGetReadsDisks verifies that Get parses disk fields from
// the Proxmox config response into Disk structs.
func TestVMAdapterGetReadsDisks(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} {
			return map[string]interface{}{
				"scsi0": "ceph-ha:vm-100-disk-0,size=20G",
				"sata0": "ceph-ha:vm-100-disk-1,size=10G",
			}
		},
		&capture,
	)
	defer server.Close()

	node := nodeName
	userDisks := []*proxmox.Disk{
		{DiskBase: proxmox.DiskBase{Storage: "ceph-ha"}, Size: 20, Interface: "scsi0"},
		{DiskBase: proxmox.DiskBase{Storage: "ceph-ha"}, Size: 10, Interface: "sata0"},
	}

	vmAdapter := newConnectedVMAdapter(t, server.URL)
	result, err := vmAdapter.Get(context.Background(), vmID, &node, userDisks)
	require.NoError(t, err)

	require.Len(t, result.Disks, 2)
	assert.Equal(t, "scsi0", result.Disks[0].Interface)
	assert.Equal(t, "ceph-ha", result.Disks[0].Storage)
	assert.Equal(t, 20, result.Disks[0].Size)
	assert.Equal(t, "sata0", result.Disks[1].Interface)
	assert.Equal(t, 10, result.Disks[1].Size)
}

// TestVMAdapterGetReadsEfiDisk verifies that Get parses the efidisk0 field
// from the Proxmox config response into an EfiDisk struct.
func TestVMAdapterGetReadsEfiDisk(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} {
			return map[string]interface{}{
				"efidisk0": "local-lvm:vm-100-disk-0,efitype=4m,pre-enrolled-keys=1,size=1M",
			}
		},
		&capture,
	)
	defer server.Close()

	node := nodeName
	vmAdapter := newConnectedVMAdapter(t, server.URL)
	result, err := vmAdapter.Get(context.Background(), vmID, &node, nil)
	require.NoError(t, err)

	require.NotNil(t, result.EfiDisk)
	assert.Equal(t, "local-lvm", result.EfiDisk.Storage)
	assert.Equal(t, proxmox.EfiType4M, result.EfiDisk.EfiType)
	require.NotNil(t, result.EfiDisk.PreEnrolledKeys)
	assert.True(t, *result.EfiDisk.PreEnrolledKeys)
}

// TestVMAdapterGetReadsTags verifies that Get splits semicolon-separated tags
// from the API response into a string slice.
func TestVMAdapterGetReadsTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tagsResponse string
		expectedTags []string
	}{
		{
			name:         "multiple tags split on semicolon",
			tagsResponse: "prod;web;frontend",
			expectedTags: []string{"prod", "web", "frontend"},
		},
		{
			name:         "single tag",
			tagsResponse: "prod",
			expectedTags: []string{"prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const nodeName = "pve-node"
			const vmIDStr = "100"
			vmID := 100

			var capture vmHTTPCapture
			server := newExtendedVMMockServer(t, nodeName, vmIDStr,
				func() map[string]interface{} {
					return map[string]interface{}{"tags": tt.tagsResponse}
				},
				&capture,
			)
			defer server.Close()

			node := nodeName
			vmAdapter := newConnectedVMAdapter(t, server.URL)
			result, err := vmAdapter.Get(context.Background(), vmID, &node, nil)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedTags, result.Tags)
		})
	}
}

// ---------------------------------------------------------------------------
// Phase 4: Delete HTTP test
// ---------------------------------------------------------------------------

// TestVMAdapterDeleteSendsCorrectRequest verifies that Delete sends
// DELETE /nodes/{node}/qemu/{vmid} to the Proxmox API.
func TestVMAdapterDeleteSendsCorrectRequest(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.Delete(context.Background(), vmID, &node)
	require.NoError(t, err)

	req := capture.find(http.MethodDelete, "/qemu/"+vmIDStr)
	require.NotNil(t, req, "expected DELETE to /nodes/%s/qemu/%s", nodeName, vmIDStr)
	assert.Equal(t, "/nodes/"+nodeName+"/qemu/"+vmIDStr, req.path)
}

// ---------------------------------------------------------------------------
// Phase 5: ApplyConfig HTTP tests
// ---------------------------------------------------------------------------

// TestVMAdapterApplyConfigSendsOptions verifies that ApplyConfig sends the
// full set of BuildVMOptions to POST /nodes/{node}/qemu/{vmid}/config.
func TestVMAdapterApplyConfigSendsOptions(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	mem := 2048

	inputs := proxmox.VMInputs{
		Name:   testutils.Ptr("apply-vm"),
		Memory: &mem,
		Tags:   []string{"staging"},
		Disks:  []*proxmox.Disk{},
	}

	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.ApplyConfig(context.Background(), vmID, &node, inputs, 60*time.Second)
	require.NoError(t, err)

	req := capture.find(http.MethodPost, "/qemu/"+vmIDStr+"/config")
	require.NotNil(t, req, "expected POST to config path")
	assert.Equal(t, "apply-vm", req.body["name"])
	assert.Equal(t, float64(2048), req.body["memory"])
	assert.Equal(t, "staging", req.body["tags"])
}

// TestVMAdapterApplyConfigMinimalInputs verifies that ApplyConfig with only
// empty inputs still sends the tags option (tags is always serialized, even
// as an empty string) and does not include other fields.
func TestVMAdapterApplyConfigMinimalInputs(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	inputs := proxmox.VMInputs{Disks: []*proxmox.Disk{}}

	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.ApplyConfig(context.Background(), vmID, &node, inputs, 60*time.Second)
	require.NoError(t, err)

	req := capture.find(http.MethodPost, "/qemu/"+vmIDStr+"/config")
	require.NotNil(t, req, "expected POST to config (tags is always sent)")
	assert.Equal(t, "", req.body["tags"], "empty inputs should send empty tags")
	_, hasName := req.body["name"]
	assert.False(t, hasName, "empty inputs should not include name")
}

// ---------------------------------------------------------------------------
// Phase 6: CloneVM HTTP tests
// ---------------------------------------------------------------------------

// TestVMAdapterCloneVMSendsRequest verifies that CloneVM sends
// POST /nodes/{node}/qemu/{sourceVmid}/clone with the correct body fields.
func TestVMAdapterCloneVMSendsRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		fullClone        *bool
		expectedFullBool float64
	}{
		{
			name:             "full clone sends full=1",
			fullClone:        testutils.Ptr(true),
			expectedFullBool: 1,
		},
		{
			name:             "linked clone omits full field",
			fullClone:        testutils.Ptr(false),
			expectedFullBool: -1, // sentinel: field is omitted by json omitempty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const nodeName = "pve-node"
			// Source VM is 102, target VM is 200
			const sourceVMIDStr = "102"
			const targetVMID = 200

			var capture vmHTTPCapture
			// Server must handle the source VM (102) for findVM
			server := newExtendedVMMockServer(t, nodeName, sourceVMIDStr,
				func() map[string]interface{} { return map[string]interface{}{} },
				&capture,
			)
			defer server.Close()

			node := nodeName
			id := targetVMID
			inputs := proxmox.VMInputs{
				Name: testutils.Ptr("cloned-vm"),
				Node: &node,
				VMID: &id,
				Clone: &proxmox.Clone{
					VMID:      102,
					FullClone: tt.fullClone,
					Timeout:   120,
				},
				Disks: []*proxmox.Disk{},
			}

			vmAdapter := newConnectedVMAdapter(t, server.URL)
			err := vmAdapter.CloneVM(context.Background(), inputs)
			require.NoError(t, err)

			req := capture.find(http.MethodPost, "/qemu/"+sourceVMIDStr+"/clone")
			require.NotNil(t, req, "expected POST to /nodes/%s/qemu/%s/clone", nodeName, sourceVMIDStr)
			assert.Equal(t, float64(targetVMID), req.body["newid"])
			assert.Equal(t, nodeName, req.body["target"])
			if tt.expectedFullBool >= 0 {
				assert.Equal(t, tt.expectedFullBool, req.body["full"])
			} else {
				_, hasFull := req.body["full"]
				assert.False(t, hasFull, "expected full to be omitted for linked clone")
			}
		})
	}
}

// TestVMAdapterCloneVMValidation verifies that CloneVM returns errors
// when required fields are missing.
func TestVMAdapterCloneVMValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		inputs proxmox.VMInputs
		errMsg string
	}{
		{
			name:   "nil Clone returns error",
			inputs: proxmox.VMInputs{Node: testutils.Ptr("n"), VMID: testutils.Ptr(1)},
			errMsg: "inputs.Clone must be set",
		},
		{
			name:   "nil Node returns error",
			inputs: proxmox.VMInputs{Clone: &proxmox.Clone{VMID: 1}, VMID: testutils.Ptr(1)},
			errMsg: "inputs.Node must be set",
		},
		{
			name:   "nil VMID returns error",
			inputs: proxmox.VMInputs{Clone: &proxmox.Clone{VMID: 1}, Node: testutils.Ptr("n")},
			errMsg: "inputs.VMID must be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const nodeName = "pve-node"
			const vmIDStr = "100"

			var capture vmHTTPCapture
			server := newExtendedVMMockServer(t, nodeName, vmIDStr,
				func() map[string]interface{} { return map[string]interface{}{} },
				&capture,
			)
			defer server.Close()

			vmAdapter := newConnectedVMAdapter(t, server.URL)
			err := vmAdapter.CloneVM(context.Background(), tt.inputs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// ---------------------------------------------------------------------------
// Phase 7: Disk operation HTTP tests
// ---------------------------------------------------------------------------

// TestVMAdapterResizeDisk verifies that ResizeDisk sends
// PUT /nodes/{node}/qemu/{vmid}/resize with the correct body fields.
func TestVMAdapterResizeDisk(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.ResizeDisk(context.Background(), vmID, &node, "scsi0", 40)
	require.NoError(t, err)

	req := capture.find(http.MethodPut, "/resize")
	require.NotNil(t, req, "expected PUT to resize endpoint")
	assert.Equal(t, "/nodes/"+nodeName+"/qemu/"+vmIDStr+"/resize", req.path)
	assert.Equal(t, "scsi0", req.body["disk"])
	assert.Equal(t, "40G", req.body["size"])
}

// TestVMAdapterRemoveDisk verifies that RemoveDisk sends
// PUT /nodes/{node}/qemu/{vmid}/unlink with the correct body fields.
func TestVMAdapterRemoveDisk(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.RemoveDisk(context.Background(), vmID, &node, "sata0")
	require.NoError(t, err)

	req := capture.find(http.MethodPut, "/unlink")
	require.NotNil(t, req, "expected PUT to unlink endpoint")
	assert.Equal(t, "/nodes/"+nodeName+"/qemu/"+vmIDStr+"/unlink", req.path)
	assert.Equal(t, "sata0", req.body["idlist"])
	assert.Equal(t, "1", req.body["force"])
}

// TestVMAdapterRemoveEfiDisk verifies that RemoveEfiDisk sends
// PUT /nodes/{node}/qemu/{vmid}/unlink with idlist=efidisk0.
func TestVMAdapterRemoveEfiDisk(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} { return map[string]interface{}{} },
		&capture,
	)
	defer server.Close()

	node := nodeName
	vmAdapter := newConnectedVMAdapter(t, server.URL)
	err := vmAdapter.RemoveEfiDisk(context.Background(), vmID, &node)
	require.NoError(t, err)

	req := capture.find(http.MethodPut, "/unlink")
	require.NotNil(t, req, "expected PUT to unlink endpoint")
	assert.Equal(t, "efidisk0", req.body["idlist"])
	assert.Equal(t, "1", req.body["force"])
}

// ---------------------------------------------------------------------------
// Phase 8: GetCurrentDisks HTTP test
// ---------------------------------------------------------------------------

// TestVMAdapterGetCurrentDisks verifies that GetCurrentDisks parses the
// config response and returns a map of disks keyed by interface, plus the EFI disk.
func TestVMAdapterGetCurrentDisks(t *testing.T) {
	t.Parallel()

	const nodeName = "pve-node"
	const vmIDStr = "100"
	vmID := 100

	var capture vmHTTPCapture
	server := newExtendedVMMockServer(t, nodeName, vmIDStr,
		func() map[string]interface{} {
			return map[string]interface{}{
				"scsi0":    "ceph-ha:vm-100-disk-0,size=20G",
				"sata0":    "ceph-ha:vm-100-disk-1,size=10G",
				"efidisk0": "local-lvm:vm-100-disk-2,efitype=4m,pre-enrolled-keys=0,size=1M",
			}
		},
		&capture,
	)
	defer server.Close()

	node := nodeName
	vmAdapter := newConnectedVMAdapter(t, server.URL)
	disks, efiDisk, err := vmAdapter.GetCurrentDisks(context.Background(), vmID, &node)
	require.NoError(t, err)

	require.Len(t, disks, 2)

	scsi0, ok := disks["scsi0"]
	require.True(t, ok, "expected scsi0 in disk map")
	assert.Equal(t, "ceph-ha", scsi0.Storage)
	assert.Equal(t, 20, scsi0.Size)
	assert.Equal(t, "scsi0", scsi0.Interface)

	sata0, ok := disks["sata0"]
	require.True(t, ok, "expected sata0 in disk map")
	assert.Equal(t, "ceph-ha", sata0.Storage)
	assert.Equal(t, 10, sata0.Size)

	require.NotNil(t, efiDisk, "expected EFI disk to be parsed")
	assert.Equal(t, "local-lvm", efiDisk.Storage)
	assert.Equal(t, proxmox.EfiType4M, efiDisk.EfiType)
}
