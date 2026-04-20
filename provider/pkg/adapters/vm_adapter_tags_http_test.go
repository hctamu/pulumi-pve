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

// Package adapters_test contains HTTP-level tests that verify tags values
// are sent correctly in VM API requests.
package adapters_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

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
