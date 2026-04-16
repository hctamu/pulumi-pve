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

// Package vm_test contains lifecycle tests for VM resource operations.
//
// These tests verify full create/update/delete cycles using integration.LifeCycleTest
// with httptest mock servers to validate HTTP API interactions.
package vm_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
)

const (
	lifecycleNodeName = "pve-node"
	lifecycleVMID     = 100.0 // Pulumi represents integers as float64 in property maps
	lifecycleVMIDStr  = "100"
	lifecycleVMName   = "test-vm"

	// Proxmox uses ";" as the tag separator in config responses.
	// Our adapter calls vm.SplitTags() to split these into a []string before returning.
	createTags = "prod;web" // Proxmox format returned by GET /config (create phase)
	updateTags = "staging"  // Proxmox format returned by GET /config (update phase)
)

// vmLifecycleCapture tracks HTTP requests by phase for verification.
type vmLifecycleCapture struct {
	mu       sync.Mutex
	requests []vmCapturedRequest
}

type vmCapturedRequest struct {
	method string
	path   string
	body   map[string]interface{}
}

func (capture *vmLifecycleCapture) add(method, path string, body map[string]interface{}) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.requests = append(capture.requests, vmCapturedRequest{method: method, path: path, body: body})
}

func (capture *vmLifecycleCapture) all() []vmCapturedRequest {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]vmCapturedRequest(nil), capture.requests...)
}

// TestVMTagsLifeCycle verifies a full create/update/delete lifecycle for a VM with tags.
// It validates that:
//   - Create sends tags as comma-separated string in the POST body
//   - Read correctly populates tags from the semicolon-separated Proxmox config format
//   - Update sends the new tags in the config POST body
func TestVMTagsLifeCycle(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping lifecycle test in short mode")
	}

	var capture vmLifecycleCapture

	// configPostCount tracks how many config POST calls have occurred to determine
	// what tags to return in subsequent GET /config calls.
	var stateMu sync.Mutex
	configPostCount := 0

	currentTags := func() string {
		stateMu.Lock()
		defer stateMu.Unlock()
		if configPostCount > 0 {
			return updateTags // after update POST, return the staging tag
		}
		return createTags // before update, return the initial tags
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var bodyData map[string]interface{}
		if r.Body != nil {
			if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
				_ = json.Unmarshal(raw, &bodyData)
			}
		}
		capture.add(r.Method, r.URL.Path, bodyData)

		enc := func(v interface{}) { _ = json.NewEncoder(w).Encode(v) }
		data := func(v interface{}) { enc(map[string]interface{}{"data": v}) }

		vmConfigPath := "/nodes/" + lifecycleNodeName + "/qemu/" + lifecycleVMIDStr + "/config"

		switch {

		// Task status: echo UPID and node to prevent copier.Copy from zeroing them.
		// See explanation in newVMHTTPMockServer in vm_adapter_tags_http_test.go.
		case strings.Contains(r.URL.Path, "/tasks/") && strings.HasSuffix(r.URL.Path, "/status"):
			parts := strings.Split(r.URL.Path, "/")
			upid := ""
			if len(parts) >= 5 {
				upid = parts[4]
			}
			data(map[string]interface{}{
				"upid":       upid,
				"node":       lifecycleNodeName,
				"status":     "stopped",
				"exitstatus": "OK",
			})

		// Cluster status: needed by FindVirtualMachine
		case r.URL.Path == "/cluster/status":
			data([]interface{}{
				map[string]interface{}{
					"type": "cluster", "name": "test", "id": "cluster",
					"version": 1, "quorate": 1, "nodes": 1,
				},
				map[string]interface{}{
					"type": "node", "name": lifecycleNodeName, "nodeid": 1,
					"id": "node/" + lifecycleNodeName, "online": 1,
					"ip": "127.0.0.1", "local": 1,
				},
			})

		// Node status: needed by Node() and NewVirtualMachine()
		case r.URL.Path == "/nodes/"+lifecycleNodeName+"/status" && r.Method == http.MethodGet:
			data(map[string]interface{}{"node": lifecycleNodeName, "status": "online", "maxcpu": 4})

		// VM status/current: needed by VirtualMachine()
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			data(map[string]interface{}{
				"vmid": lifecycleVMIDStr, "name": lifecycleVMName, "status": "running",
			})

		// VM config GET: returns tags in Proxmox's semicolon-separated format.
		// Our adapter's ConvertVMConfigToInputs bug-fix (vm.SplitTags) will split
		// these into a []string before populating VMInputs.Tags.
		case r.URL.Path == vmConfigPath && r.Method == http.MethodGet:
			data(map[string]interface{}{
				"name": lifecycleVMName, "memory": 2048,
				"vmid": lifecycleVMIDStr, "tags": currentTags(),
			})

		// Create VM POST
		case r.URL.Path == "/nodes/"+lifecycleNodeName+"/qemu" && r.Method == http.MethodPost:
			data("UPID:" + lifecycleNodeName + ":00001111:00000000:60000000:qmcreate:" + lifecycleVMIDStr + ":root@pam:")

		// Update VM config POST
		case r.URL.Path == vmConfigPath && r.Method == http.MethodPost:
			stateMu.Lock()
			configPostCount++
			stateMu.Unlock()
			data("UPID:" + lifecycleNodeName + ":00002222:00000000:60000001:qmconfig:" + lifecycleVMIDStr + ":root@pam:")

		// Delete VM
		case strings.HasSuffix(r.URL.Path, "/qemu/"+lifecycleVMIDStr) && r.Method == http.MethodDelete:
			data("UPID:" + lifecycleNodeName + ":00003333:00000000:60000002:qmdestroy:" + lifecycleVMIDStr + ":root@pam:")

		default:
			w.WriteHeader(http.StatusNotFound)
			data(nil)
		}
	}))
	defer server.Close()

	testConfig := &config.Config{
		PveURL:   server.URL,
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}

	pulumiServer, err := integration.NewServer(
		t.Context(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProviderWithConfig(testConfig)),
	)
	require.NoError(t, err)

	emptyDisks := property.New(property.NewArray([]property.Value{}))
	tagsCreate := property.New(property.NewArray([]property.Value{
		property.New("prod"),
		property.New("web"),
	}))
	tagsUpdate := property.New(property.NewArray([]property.Value{
		property.New("staging"),
	}))

	vmID := property.New(lifecycleVMID)
	node := property.New(lifecycleNodeName)
	name := property.New(lifecycleVMName)

	integration.LifeCycleTest{
		Resource: "pve:vm:VM",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":  name,
				"vmId":  vmID,
				"node":  node,
				"tags":  tagsCreate,
				"disks": emptyDisks,
			}),
			Hook: func(in, out property.Map) {
				tagsOut := out.Get("tags")
				require.False(t, tagsOut.IsNull(), "expected output tags to be present after Create")
				tagsArr := tagsOut.AsArray().AsSlice()
				assert.Equal(t, 2, len(tagsArr), "expected 2 tags after Create")
				if len(tagsArr) == 2 {
					assert.Equal(t, "prod", tagsArr[0].AsString())
					assert.Equal(t, "web", tagsArr[1].AsString())
				}
			},
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"name":  name,
				"vmId":  vmID,
				"node":  node,
				"tags":  tagsUpdate,
				"disks": emptyDisks,
			}),
			Hook: func(in, out property.Map) {
				tagsOut := out.Get("tags")
				require.False(t, tagsOut.IsNull(), "expected output tags to be present after Update")
				tagsArr := tagsOut.AsArray().AsSlice()
				assert.Equal(t, 1, len(tagsArr), "expected 1 tag after Update")
				if len(tagsArr) == 1 {
					assert.Equal(t, "staging", tagsArr[0].AsString())
				}
			},
		}},
	}.Run(t, pulumiServer)

	// Verify expected HTTP API calls.
	requests := capture.all()
	require.Greater(t, len(requests), 0, "expected at least one HTTP request")

	var createBody, updateBody map[string]interface{}
	var hasCreate, hasUpdate, hasDelete bool
	for _, req := range requests {
		switch {
		case req.method == http.MethodPost && req.path == "/nodes/"+lifecycleNodeName+"/qemu":
			hasCreate = true
			createBody = req.body
		case req.method == http.MethodPost && req.path == "/nodes/"+lifecycleNodeName+"/qemu/"+lifecycleVMIDStr+"/config":
			hasUpdate = true
			updateBody = req.body
		case req.method == http.MethodDelete && strings.HasSuffix(req.path, "/qemu/"+lifecycleVMIDStr):
			hasDelete = true
		}
	}

	assert.True(t, hasCreate, "expected POST to /nodes/%s/qemu (create)", lifecycleNodeName)
	assert.True(t, hasUpdate, "expected POST to /nodes/%s/qemu/%s/config (update)", lifecycleNodeName, lifecycleVMIDStr)
	assert.True(t, hasDelete, "expected DELETE to /nodes/%s/qemu/%s (cleanup)", lifecycleNodeName, lifecycleVMIDStr)

	if hasCreate {
		assert.Equal(t, "prod,web", createBody["tags"],
			"expected tags to be comma-joined in create POST body")
	}
	if hasUpdate {
		assert.Equal(t, "staging", updateBody["tags"],
			"expected updated tag in config POST body")
	}
}

// ---------------------------------------------------------------------------
// Shared infrastructure
// ---------------------------------------------------------------------------

// newVMLifecycleServer starts a mock Proxmox HTTP server for VM lifecycle tests.
//
// configData is called on every GET /config request and returns the config fields
// to include in the response. This allows the test to change what the server vends
// after an update POST by mutating state behind a closure.
//
// onPost is called with (isCreate bool, body) for every POST that targets either
// /nodes/{node}/qemu (create, isCreate=true) or /nodes/{node}/qemu/{vmid}/config
// (config update, isCreate=false). The capture parameter receives all requests.
//
// nextID, when non-empty, handles GET /cluster/nextid and returns the string as the
// next available VMID (used for the computed-VMID test).
//
// IMPORTANT: task status responses echo back the upid and node fields so that
// go-proxmox's Task.UnmarshalJSON / copier.Copy does not zero them (which would
// make the subsequent Ping construct the path "/nodes//tasks//status" and panic).
func newVMLifecycleServer(
	t *testing.T,
	nodeName, vmIDStr string,
	configData func() map[string]interface{},
	onPost func(isCreate bool, body map[string]interface{}),
	capture *vmLifecycleCapture,
	nextID string,
) *httptest.Server {
	t.Helper()

	nodeStatusPath := "/nodes/" + nodeName + "/status"
	createVMPath := "/nodes/" + nodeName + "/qemu"
	vmConfigPath := "/nodes/" + nodeName + "/qemu/" + vmIDStr + "/config"

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var bodyData map[string]interface{}
		if r.Body != nil {
			if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
				_ = json.Unmarshal(raw, &bodyData)
			}
		}
		capture.add(r.Method, r.URL.Path, bodyData)

		enc := func(v interface{}) { _ = json.NewEncoder(w).Encode(v) }
		data := func(v interface{}) { enc(map[string]interface{}{"data": v}) }

		switch {

		// Task status — echo upid+node so copier.Copy doesn't zero them.
		case strings.Contains(r.URL.Path, "/tasks/") && strings.HasSuffix(r.URL.Path, "/status"):
			parts := strings.Split(r.URL.Path, "/")
			upid := ""
			if len(parts) >= 5 {
				upid = parts[4]
			}
			data(map[string]interface{}{
				"upid":       upid,
				"node":       nodeName,
				"status":     "stopped",
				"exitstatus": "OK",
			})

		// Cluster nextid — optional, used for computed-VMID tests.
		case r.URL.Path == "/cluster/nextid" && nextID != "":
			data(nextID)

		// Cluster status — required by FindVirtualMachine.
		case r.URL.Path == "/cluster/status":
			data([]interface{}{
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
			data(map[string]interface{}{"node": nodeName, "status": "online", "maxcpu": 4})

		// VM status/current — needed by VirtualMachine().
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			data(map[string]interface{}{
				"vmid": vmIDStr, "name": lifecycleVMName, "status": "running",
			})

		// VM config GET — state-dependent, provided by configData.
		case r.URL.Path == vmConfigPath && r.Method == http.MethodGet:
			fields := configData()
			fields["name"] = lifecycleVMName
			fields["vmid"] = vmIDStr
			data(fields)

		// Create VM POST.
		case r.URL.Path == createVMPath && r.Method == http.MethodPost:
			if onPost != nil {
				onPost(true, bodyData)
			}
			data("UPID:" + nodeName + ":00001111:00000000:60000000:qmcreate:" + vmIDStr + ":root@pam:")

		// Config update POST.
		case r.URL.Path == vmConfigPath && r.Method == http.MethodPost:
			if onPost != nil {
				onPost(false, bodyData)
			}
			data("UPID:" + nodeName + ":00002222:00000001:60000001:qmconfig:" + vmIDStr + ":root@pam:")

		// Delete VM.
		case strings.HasSuffix(r.URL.Path, "/qemu/"+vmIDStr) && r.Method == http.MethodDelete:
			data("UPID:" + nodeName + ":00003333:00000002:60000002:qmdestroy:" + vmIDStr + ":root@pam:")

		default:
			w.WriteHeader(http.StatusNotFound)
			data(nil)
		}
	}))
}

// newVMLifecyclePulumiServer creates an integration.Server wired to the given HTTP server URL.
func newVMLifecyclePulumiServer(t *testing.T, serverURL string) integration.Server {
	t.Helper()
	cfg := &config.Config{
		PveURL:   serverURL,
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}
	srv, err := integration.NewServer(
		t.Context(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProviderWithConfig(cfg)),
	)
	require.NoError(t, err)
	return srv
}

// ---------------------------------------------------------------------------
// Memory lifecycle
// ---------------------------------------------------------------------------

// TestVMMemoryLifeCycle verifies that memory (and balloon) values are correctly sent
// on create, read back from the API, and updated via config POST.
func TestVMMemoryLifeCycle(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping lifecycle test in short mode")
	}

	var capture vmLifecycleCapture

	const (
		memCreate = 2048
		memUpdate = 4096
		balloon   = 1024
	)

	var stateMu sync.Mutex
	configPostsSeen := 0

	currentConfig := func() map[string]interface{} {
		stateMu.Lock()
		defer stateMu.Unlock()
		mem := memCreate
		if configPostsSeen > 0 {
			mem = memUpdate
		}
		return map[string]interface{}{"memory": mem, "balloon": balloon}
	}

	var createBody, updateBody map[string]interface{}
	onPost := func(isCreate bool, body map[string]interface{}) {
		stateMu.Lock()
		defer stateMu.Unlock()
		if isCreate {
			createBody = body
		} else {
			updateBody = body
			configPostsSeen++
		}
	}

	server := newVMLifecycleServer(t, lifecycleNodeName, lifecycleVMIDStr,
		currentConfig, onPost, &capture, "")
	defer server.Close()

	pulumiServer := newVMLifecyclePulumiServer(t, server.URL)

	emptyDisks := property.New(property.NewArray([]property.Value{}))
	vmID := property.New(lifecycleVMID)
	node := property.New(lifecycleNodeName)
	name := property.New(lifecycleVMName)

	integration.LifeCycleTest{
		Resource: "pve:vm:VM",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    name,
				"vmId":    vmID,
				"node":    node,
				"memory":  property.New(float64(memCreate)),
				"balloon": property.New(float64(balloon)),
				"disks":   emptyDisks,
			}),
			Hook: func(_, out property.Map) {
				outMem := out.Get("memory")
				require.False(t, outMem.IsNull(), "expected memory in output after Create")
				assert.Equal(t, float64(memCreate), outMem.AsNumber())

				outBalloon := out.Get("balloon")
				require.False(t, outBalloon.IsNull(), "expected balloon in output after Create")
				assert.Equal(t, float64(balloon), outBalloon.AsNumber())
			},
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    name,
				"vmId":    vmID,
				"node":    node,
				"memory":  property.New(float64(memUpdate)),
				"balloon": property.New(float64(balloon)),
				"disks":   emptyDisks,
			}),
			Hook: func(_, out property.Map) {
				outMem := out.Get("memory")
				require.False(t, outMem.IsNull(), "expected memory in output after Update")
				assert.Equal(t, float64(memUpdate), outMem.AsNumber())
			},
		}},
	}.Run(t, pulumiServer)

	stateMu.Lock()
	cb, ub := createBody, updateBody
	stateMu.Unlock()

	require.NotNil(t, cb, "expected create POST to be captured")
	assert.Equal(t, float64(memCreate), cb["memory"], "create POST should include memory")
	assert.Equal(t, float64(balloon), cb["balloon"], "create POST should include balloon")

	require.NotNil(t, ub, "expected config update POST to be captured")
	assert.Equal(t, float64(memUpdate), ub["memory"], "update POST should include new memory")
	_, hasBalloon := ub["balloon"]
	assert.False(t, hasBalloon, "update POST should omit balloon when unchanged")
}

// ---------------------------------------------------------------------------
// CPU lifecycle
// ---------------------------------------------------------------------------

// TestVMCPULifeCycle verifies that CPU type and core count are sent on create,
// read back from the API config response, and that only changed fields are sent
// on update (cores changes; cpu type string stays the same so is omitted from diff).
func TestVMCPULifeCycle(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping lifecycle test in short mode")
	}

	var capture vmLifecycleCapture

	const (
		cpuType    = "host"
		coresStart = 2
		coresEnd   = 4
	)

	var stateMu sync.Mutex
	configPostsSeen := 0

	currentConfig := func() map[string]interface{} {
		stateMu.Lock()
		defer stateMu.Unlock()
		cores := coresStart
		if configPostsSeen > 0 {
			cores = coresEnd
		}
		return map[string]interface{}{"cpu": cpuType, "cores": cores}
	}

	var createBody, updateBody map[string]interface{}
	onPost := func(isCreate bool, body map[string]interface{}) {
		stateMu.Lock()
		defer stateMu.Unlock()
		if isCreate {
			createBody = body
		} else {
			updateBody = body
			configPostsSeen++
		}
	}

	server := newVMLifecycleServer(t, lifecycleNodeName, lifecycleVMIDStr,
		currentConfig, onPost, &capture, "")
	defer server.Close()

	pulumiServer := newVMLifecyclePulumiServer(t, server.URL)

	emptyDisks := property.New(property.NewArray([]property.Value{}))
	vmID := property.New(lifecycleVMID)
	node := property.New(lifecycleNodeName)
	name := property.New(lifecycleVMName)

	cpuCreate := property.New(property.NewMap(map[string]property.Value{
		"type":  property.New(cpuType),
		"cores": property.New(float64(coresStart)),
	}))
	cpuUpdate := property.New(property.NewMap(map[string]property.Value{
		"type":  property.New(cpuType),
		"cores": property.New(float64(coresEnd)),
	}))

	integration.LifeCycleTest{
		Resource: "pve:vm:VM",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":  name,
				"vmId":  vmID,
				"node":  node,
				"cpu":   cpuCreate,
				"disks": emptyDisks,
			}),
			Hook: func(_, out property.Map) {
				outCPU := out.Get("cpu")
				require.False(t, outCPU.IsNull(), "expected cpu in output after Create")
				cpuMap := outCPU.AsMap()
				assert.Equal(t, cpuType, cpuMap.Get("type").AsString())
				assert.Equal(t, float64(coresStart), cpuMap.Get("cores").AsNumber())
			},
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"name":  name,
				"vmId":  vmID,
				"node":  node,
				"cpu":   cpuUpdate,
				"disks": emptyDisks,
			}),
			Hook: func(_, out property.Map) {
				outCPU := out.Get("cpu")
				require.False(t, outCPU.IsNull(), "expected cpu in output after Update")
				cpuMap := outCPU.AsMap()
				assert.Equal(t, cpuType, cpuMap.Get("type").AsString())
				assert.Equal(t, float64(coresEnd), cpuMap.Get("cores").AsNumber())
			},
		}},
	}.Run(t, pulumiServer)

	stateMu.Lock()
	cb, ub := createBody, updateBody
	stateMu.Unlock()

	require.NotNil(t, cb, "expected create POST to be captured")
	assert.Equal(t, cpuType, cb["cpu"], "create POST should include cpu type string")
	assert.Equal(t, float64(coresStart), cb["cores"], "create POST should include cores")

	require.NotNil(t, ub, "expected config update POST to be captured")
	// Only cores changed; the cpu type string is the same so addCPUDiff omits it.
	assert.Equal(t, float64(coresEnd), ub["cores"], "update POST should include new cores")
	_, hasCPUType := ub["cpu"]
	assert.False(t, hasCPUType, "update POST should omit cpu type string when unchanged")
}

// ---------------------------------------------------------------------------
// Description lifecycle
// ---------------------------------------------------------------------------

// TestVMDescriptionLifeCycle verifies that the description field is sent correctly
// on create, read back from the API, and updated via config POST.
func TestVMDescriptionLifeCycle(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping lifecycle test in short mode")
	}

	var capture vmLifecycleCapture

	const (
		descCreate = "initial description"
		descUpdate = "updated description"
	)

	var stateMu sync.Mutex
	configPostsSeen := 0

	currentConfig := func() map[string]interface{} {
		stateMu.Lock()
		defer stateMu.Unlock()
		desc := descCreate
		if configPostsSeen > 0 {
			desc = descUpdate
		}
		return map[string]interface{}{"description": desc}
	}

	var createBody, updateBody map[string]interface{}
	onPost := func(isCreate bool, body map[string]interface{}) {
		stateMu.Lock()
		defer stateMu.Unlock()
		if isCreate {
			createBody = body
		} else {
			updateBody = body
			configPostsSeen++
		}
	}

	server := newVMLifecycleServer(t, lifecycleNodeName, lifecycleVMIDStr,
		currentConfig, onPost, &capture, "")
	defer server.Close()

	pulumiServer := newVMLifecyclePulumiServer(t, server.URL)

	emptyDisks := property.New(property.NewArray([]property.Value{}))
	vmID := property.New(lifecycleVMID)
	node := property.New(lifecycleNodeName)
	name := property.New(lifecycleVMName)

	integration.LifeCycleTest{
		Resource: "pve:vm:VM",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":        name,
				"vmId":        vmID,
				"node":        node,
				"description": property.New(descCreate),
				"disks":       emptyDisks,
			}),
			Hook: func(_, out property.Map) {
				outDesc := out.Get("description")
				require.False(t, outDesc.IsNull(), "expected description in output after Create")
				assert.Equal(t, descCreate, outDesc.AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"name":        name,
				"vmId":        vmID,
				"node":        node,
				"description": property.New(descUpdate),
				"disks":       emptyDisks,
			}),
			Hook: func(_, out property.Map) {
				outDesc := out.Get("description")
				require.False(t, outDesc.IsNull(), "expected description in output after Update")
				assert.Equal(t, descUpdate, outDesc.AsString())
			},
		}},
	}.Run(t, pulumiServer)

	stateMu.Lock()
	cb, ub := createBody, updateBody
	stateMu.Unlock()

	require.NotNil(t, cb, "expected create POST to be captured")
	assert.Equal(t, descCreate, cb["description"], "create POST should include description")

	require.NotNil(t, ub, "expected config update POST to be captured")
	assert.Equal(t, descUpdate, ub["description"], "update POST should include updated description")
}

// ---------------------------------------------------------------------------
// Computed VMID lifecycle
// ---------------------------------------------------------------------------

// TestVMComputedVMIDLifeCycle verifies that when no vmId is provided in the inputs
// the provider calls GET /cluster/nextid to obtain one, and the computed value
// appears in the output state after Create.
func TestVMComputedVMIDLifeCycle(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping lifecycle test in short mode")
	}

	const computedVMID = "100"

	var capture vmLifecycleCapture

	currentConfig := func() map[string]interface{} {
		return map[string]interface{}{"memory": 2048}
	}

	server := newVMLifecycleServer(t, lifecycleNodeName, computedVMID,
		currentConfig, nil, &capture, computedVMID)
	defer server.Close()

	pulumiServer := newVMLifecyclePulumiServer(t, server.URL)

	emptyDisks := property.New(property.NewArray([]property.Value{}))

	integration.LifeCycleTest{
		Resource: "pve:vm:VM",
		Create: integration.Operation{
			// vmId is intentionally omitted — the provider must call /cluster/nextid.
			Inputs: property.NewMap(map[string]property.Value{
				"name":  property.New(lifecycleVMName),
				"node":  property.New(lifecycleNodeName),
				"disks": emptyDisks,
			}),
			Hook: func(_, out property.Map) {
				outVMID := out.Get("vmId")
				require.False(t, outVMID.IsNull(), "expected vmId to be computed in output after Create")
				assert.Equal(t, lifecycleVMID /* 100.0 */, outVMID.AsNumber(),
					"computed vmId should match the value returned by /cluster/nextid")
			},
		},
	}.Run(t, pulumiServer)

	// Verify that /cluster/nextid was actually called to obtain the VMID.
	requests := capture.all()
	var calledNextID bool
	for _, req := range requests {
		if req.method == http.MethodGet && req.path == "/cluster/nextid" {
			calledNextID = true
			break
		}
	}
	assert.True(t, calledNextID, "expected GET /cluster/nextid to be called when vmId is omitted")
}
