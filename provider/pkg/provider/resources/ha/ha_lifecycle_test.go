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

// Package ha_test contains lifecycle tests for HA resource operations.
//
// These tests verify full create/update/delete cycles using integration.LifeCycleTest
// with httptest mock servers to validate HTTP API interactions.
package ha_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Test constants (constants shared with ha_test.go: resourceName, id100Float, group1, group2)
const (
	apiPathHAResources = "/api2/json/cluster/ha/resources/"

	stateStarted = "started"
	stateStopped = "stopped"

	id100      = "100"
	id200      = "200"
	id200Float = 200.0
)

// requestCapture captures API request details for verification
type requestCapture struct {
	mu       sync.Mutex
	requests []capturedRequest
}

type capturedRequest struct {
	method string
	path   string
	body   map[string]interface{}
}

func (rc *requestCapture) add(method, path string, body map[string]interface{}) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.requests = append(rc.requests, capturedRequest{
		method: method,
		path:   path,
		body:   body,
	})
}

func (rc *requestCapture) get() []capturedRequest {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return append([]capturedRequest(nil), rc.requests...)
}

//nolint:paralleltest // uses global env + client seam
func TestHaHealthyLifeCycle(t *testing.T) {
	var getCount int
	var capture requestCapture

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Read and parse body if present
		var bodyData map[string]interface{}
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			if len(bodyBytes) > 0 {
				_ = json.Unmarshal(bodyBytes, &bodyData)
			}
		}

		capture.add(r.Method, r.URL.Path, bodyData)

		switch r.Method {
		case http.MethodPost:
			// Create HA resource
			assert.Contains(t, r.URL.Path, "/cluster/ha/resources")
			assert.Equal(t, group1, bodyData["group"])
			assert.Equal(t, stateStarted, bodyData["state"])
			assert.Equal(t, id100, bodyData["sid"])

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"sid":   id100,
					"state": stateStarted,
					"group": group1,
				},
			})

		case http.MethodGet:
			// Read HA resource - return different state after first call
			switch r.URL.Path {
			case apiPathHAResources + id100:
				getCount++
				state := stateStarted
				group := group1
				if getCount >= 2 { // After update, return new state
					state = stateStopped
					group = group2
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"sid":   id100,
						"state": state,
						"group": group,
					},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		case http.MethodPut:
			// Update HA resource
			assert.Contains(t, r.URL.Path, "/cluster/ha/resources/"+id100)
			assert.Equal(t, group2, bodyData["group"])
			assert.Equal(t, stateStopped, bodyData["state"])

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"sid":   id100,
					"state": stateStopped,
					"group": group2,
				},
			})

		case http.MethodDelete:
			// Delete HA resource
			assert.Contains(t, r.URL.Path, "/cluster/ha/resources/"+id100)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": nil,
			})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	// Create provider with test config
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

	// Expected output after update
	expected := property.NewMap(map[string]property.Value{
		"group":      property.New(group2),
		"state":      property.New(stateStopped),
		"resourceId": property.New(id100Float),
	})

	// Run lifecycle test
	integration.LifeCycleTest{
		Resource: "pve:ha:HA",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New(group1),
				"state":      property.New(stateStarted),
				"resourceId": property.New(id100Float),
			}),
			Hook: func(in, out property.Map) {
				assert.Equal(t, id100Float, out.Get("resourceId").AsNumber())
				assert.Equal(t, group1, out.Get("group").AsString())
				assert.Equal(t, stateStarted, out.Get("state").AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New(group2),
				"state":      property.New(stateStopped),
				"resourceId": property.New(id100Float),
			}),
			ExpectedOutput: &expected,
		}},
	}.Run(t, pulumiServer)

	// Verify API calls
	requests := capture.get()
	require.Greater(t, len(requests), 0, "Should have captured API requests")

	// Verify we have the expected API calls: POST (create), PUT (update), DELETE (cleanup)
	// Note: GET may not always be called depending on the test framework behavior
	var hasPOST, hasPUT, hasDELETE bool
	for _, req := range requests {
		switch req.method {
		case http.MethodPost:
			hasPOST = true
			assert.Equal(t, group1, req.body["group"])
			assert.Equal(t, stateStarted, req.body["state"])
		case http.MethodPut:
			hasPUT = true
			assert.Equal(t, group2, req.body["group"])
			assert.Equal(t, stateStopped, req.body["state"])
		case http.MethodDelete:
			hasDELETE = true
		}
	}

	assert.True(t, hasPOST, "Should have made POST request (create)")
	assert.True(t, hasPUT, "Should have made PUT request (update)")
	assert.True(t, hasDELETE, "Should have made DELETE request (cleanup)")
}

//nolint:paralleltest // uses global env + client seam
func TestHaResourceIDChangeTriggersReplace(t *testing.T) {
	var operations []string
	var mu sync.Mutex
	var capture requestCapture

	recordOp := func(op string) {
		mu.Lock()
		defer mu.Unlock()
		operations = append(operations, op)
	}

	postCallCount := 0

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Read and parse body if present
		var bodyData map[string]interface{}
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			if len(bodyBytes) > 0 {
				_ = json.Unmarshal(bodyBytes, &bodyData)
			}
		}

		capture.add(r.Method, r.URL.Path, bodyData)

		switch r.Method {
		case http.MethodPost:
			// Create HA resource
			postCallCount++
			if postCallCount == 1 {
				// First create - resource 100
				recordOp("create-100")
				assert.Equal(t, id100, bodyData["sid"])

				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"sid":   id100,
						"state": stateStarted,
						"group": group1,
					},
				})
			} else {
				// Second create - resource 200 (replacement)
				recordOp("create-200")
				assert.Equal(t, id200, bodyData["sid"])

				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"sid":   id200,
						"state": stateStarted,
						"group": group1,
					},
				})
			}

		case http.MethodGet:
			// Read HA resource
			switch r.URL.Path {
			case apiPathHAResources + id100:
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"sid":   id100,
						"state": stateStarted,
						"group": group1,
					},
				})
			case apiPathHAResources + id200:
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"sid":   id200,
						"state": stateStarted,
						"group": group1,
					},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		case http.MethodDelete:
			// Delete HA resource
			switch r.URL.Path {
			case apiPathHAResources + id100:
				recordOp("delete-100")
			case apiPathHAResources + id200:
				recordOp("delete-200")
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": nil,
			})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	// Create provider with test config
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

	expectedAfterReplace := property.NewMap(map[string]property.Value{
		"group":      property.New(group1),
		"state":      property.New(stateStarted),
		"resourceId": property.New(id200Float),
	})

	// Run lifecycle test
	integration.LifeCycleTest{
		Resource: "pve:ha:HA",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New(group1),
				"state":      property.New(stateStarted),
				"resourceId": property.New(id100Float),
			}),
			Hook: func(in, out property.Map) {
				assert.Equal(t, id100Float, out.Get("resourceId").AsNumber())
			},
		},
		Updates: []integration.Operation{{
			// Change ResourceID - should trigger replace
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New(group1),
				"state":      property.New(stateStarted),
				"resourceId": property.New(id200Float), // Changed from 100 to 200
			}),
			ExpectedOutput: &expectedAfterReplace,
		}},
	}.Run(t, pulumiServer)

	// Verify operations happened and resource was replaced
	mu.Lock()
	defer mu.Unlock()

	// Should have at least: create-100, create-200 (replacement)
	require.GreaterOrEqual(t, len(operations), 2, "Should have at least 2 operations (both creates)")

	// Find indices of key operations
	create100Idx, delete100Idx, create200Idx := -1, -1, -1
	for i, op := range operations {
		switch op {
		case "create-100":
			if create100Idx == -1 {
				create100Idx = i
			}
		case "delete-100":
			delete100Idx = i
		case "create-200":
			create200Idx = i
		}
	}

	// Verify replacement happened (both creates occurred)
	assert.NotEqual(t, -1, create100Idx, "Initial create-100 should have happened")
	assert.NotEqual(t, -1, create200Idx, "Replacement create-200 should have happened")

	// Verify that ResourceID change triggered a replace (new resource was created)
	assert.Less(t, create100Idx, create200Idx, "Original resource should be created before replacement")

	// If delete-100 happened, verify it was after create-200 (create-before-delete)
	if delete100Idx != -1 {
		assert.Less(t, create200Idx, delete100Idx, "New resource created before old one deleted (create-before-delete)")
	}

	// Verify API request details
	requests := capture.get()
	var create100Req, create200Req *capturedRequest
	for i := range requests {
		if requests[i].method == http.MethodPost {
			switch requests[i].body["sid"] {
			case id100:
				create100Req = &requests[i]
			case id200:
				create200Req = &requests[i]
			}
		}
	}

	require.NotNil(t, create100Req, "Should have captured create-100 request")
	require.NotNil(t, create200Req, "Should have captured create-200 request")

	// Verify both creates had correct body
	assert.Equal(t, id100, create100Req.body["sid"])
	assert.Equal(t, id200, create200Req.body["sid"])
	assert.Equal(t, group1, create100Req.body["group"])
	assert.Equal(t, group1, create200Req.body["group"])
}
