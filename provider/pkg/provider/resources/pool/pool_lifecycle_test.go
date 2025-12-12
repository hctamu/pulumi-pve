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

// Package pool_test contains lifecycle tests for Pool resource operations.
//
// These tests verify full create/update/delete cycles using integration.LifeCycleTest
// with httptest mock servers to validate HTTP API interactions.
package pool_test

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

// Test constants
const (
	poolNameTest    = "testpool"
	commentInitial  = "test pool comment"
	commentUpdated  = "updated comment"
	poolName2Test   = "testpool2"
	commentInitial2 = "initial comment for pool2"
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

func TestPoolHealthyLifeCycle(t *testing.T) {
	t.Parallel()
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
			// Create Pool resource
			assert.Equal(t, "/pools", r.URL.Path)
			assert.Equal(t, poolNameTest, bodyData["poolid"])
			assert.Equal(t, commentInitial, bodyData["comment"])

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"poolid":  poolNameTest,
					"comment": commentInitial,
				},
			})

		case http.MethodGet:
			// Read Pool resource - return different comment after first call
			switch r.URL.Path {
			case "/pools/" + poolNameTest:
				getCount++
				comment := commentInitial
				if getCount >= 2 { // After update, return new comment
					comment = commentUpdated
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  poolNameTest,
						"comment": comment,
					},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		case http.MethodPut:
			// Update Pool resource
			assert.Equal(t, "/pools/"+poolNameTest, r.URL.Path)
			assert.Equal(t, commentUpdated, bodyData["comment"])

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"poolid":  poolNameTest,
					"comment": commentUpdated,
				},
			})

		case http.MethodDelete:
			// Delete Pool resource
			assert.Equal(t, "/pools/"+poolNameTest, r.URL.Path)

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
		"name":    property.New(poolNameTest),
		"comment": property.New(commentUpdated),
	})

	// Run lifecycle test
	integration.LifeCycleTest{
		Resource: "pve:pool:Pool",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New(poolNameTest),
				"comment": property.New(commentInitial),
			}),
			Hook: func(in, out property.Map) {
				assert.Equal(t, poolNameTest, out.Get("name").AsString())
				assert.Equal(t, commentInitial, out.Get("comment").AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New(poolNameTest),
				"comment": property.New(commentUpdated),
			}),
			ExpectedOutput: &expected,
		}},
	}.Run(t, pulumiServer)

	// Verify API calls
	requests := capture.get()
	require.Greater(t, len(requests), 0, "Should have captured API requests")

	// Verify we have the expected API calls: POST (create), PUT (update), DELETE (cleanup)
	var hasPOST, hasPUT, hasDELETE bool
	for _, req := range requests {
		switch req.method {
		case http.MethodPost:
			hasPOST = true
			assert.Equal(t, poolNameTest, req.body["poolid"])
			assert.Equal(t, commentInitial, req.body["comment"])
		case http.MethodPut:
			hasPUT = true
			assert.Equal(t, commentUpdated, req.body["comment"])
		case http.MethodDelete:
			hasDELETE = true
		}
	}

	assert.True(t, hasPOST, "Should have made POST request (create)")
	assert.True(t, hasPUT, "Should have made PUT request (update)")
	assert.True(t, hasDELETE, "Should have made DELETE request (cleanup)")
}

func TestPoolNameChangeTriggersReplace(t *testing.T) {
	t.Parallel()
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
			// Create Pool resource
			postCallCount++
			if postCallCount == 1 {
				// First create - pool1
				recordOp("create-pool1")
				assert.Equal(t, poolNameTest, bodyData["poolid"])

				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  poolNameTest,
						"comment": commentInitial,
					},
				})
			} else {
				// Second create - pool2 (replacement)
				recordOp("create-pool2")
				assert.Equal(t, poolName2Test, bodyData["poolid"])

				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  poolName2Test,
						"comment": commentInitial2,
					},
				})
			}

		case http.MethodGet:
			// Read Pool resource
			switch r.URL.Path {
			case "/pools/" + poolNameTest:
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  poolNameTest,
						"comment": commentInitial,
					},
				})
			case "/pools/" + poolName2Test:
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  poolName2Test,
						"comment": commentInitial2,
					},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		case http.MethodDelete:
			// Delete Pool resource
			switch r.URL.Path {
			case "/pools/" + poolNameTest:
				recordOp("delete-pool1")
			case "/pools/" + poolName2Test:
				recordOp("delete-pool2")
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
		"name":    property.New(poolName2Test),
		"comment": property.New(commentInitial2),
	})

	// Run lifecycle test
	integration.LifeCycleTest{
		Resource: "pve:pool:Pool",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New(poolNameTest),
				"comment": property.New(commentInitial),
			}),
			Hook: func(in, out property.Map) {
				assert.Equal(t, poolNameTest, out.Get("name").AsString())
			},
		},
		Updates: []integration.Operation{{
			// Change name - should trigger replace
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New(poolName2Test), // Changed from testpool to testpool2
				"comment": property.New(commentInitial2),
			}),
			ExpectedOutput: &expectedAfterReplace,
		}},
	}.Run(t, pulumiServer)

	// Verify operations happened and resource was replaced
	mu.Lock()
	defer mu.Unlock()

	// Should have at least: create-pool1, create-pool2 (replacement)
	require.GreaterOrEqual(t, len(operations), 2, "Should have at least 2 operations (both creates)")

	// Find indices of key operations
	createPool1Idx, deletePool1Idx, createPool2Idx := -1, -1, -1
	for i, op := range operations {
		switch op {
		case "create-pool1":
			if createPool1Idx == -1 {
				createPool1Idx = i
			}
		case "delete-pool1":
			deletePool1Idx = i
		case "create-pool2":
			createPool2Idx = i
		}
	}

	// Verify replacement happened (both creates occurred)
	assert.NotEqual(t, -1, createPool1Idx, "Initial create-pool1 should have happened")
	assert.NotEqual(t, -1, createPool2Idx, "Replacement create-pool2 should have happened")
	assert.NotEqual(t, -1, deletePool1Idx, "Old pool1 should have been deleted during replacement")
	// Verify that name change triggered a replace (new resource was created)
	assert.Less(t, createPool1Idx, createPool2Idx, "Original resource should be created before replacement")

	assert.Less(t, createPool2Idx, deletePool1Idx, "Old resource deleted before new one created")

	// Verify API request details
	requests := capture.get()
	var createPool1Req, createPool2Req *capturedRequest
	for i := range requests {
		if requests[i].method == http.MethodPost {
			switch requests[i].body["poolid"] {
			case poolNameTest:
				createPool1Req = &requests[i]
			case poolName2Test:
				createPool2Req = &requests[i]
			}
		}
	}

	require.NotNil(t, createPool1Req, "Should have captured create-pool1 request")
	require.NotNil(t, createPool2Req, "Should have captured create-pool2 request")

	// Verify both creates had correct body
	assert.Equal(t, poolNameTest, createPool1Req.body["poolid"])
	assert.Equal(t, poolName2Test, createPool2Req.body["poolid"])
	assert.Equal(t, commentInitial, createPool1Req.body["comment"])
	assert.Equal(t, commentInitial2, createPool2Req.body["comment"])
}
