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

package group_test

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

func TestGroupHealthyLifeCycle(t *testing.T) {
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
			// Create Group resource
			assert.Equal(t, "/access/groups", r.URL.Path)
			assert.Equal(t, groupID, bodyData["groupid"])
			assert.Equal(t, groupComment, bodyData["comment"])

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"groupid": groupID,
					"comment": groupComment,
				},
			})

		case http.MethodGet:
			// Read Group resource - return different comment after first call
			switch r.URL.Path {
			case "/access/groups/" + groupID:
				getCount++
				comment := groupComment
				if getCount >= 2 { // After update, return new comment
					comment = updatedGroupComment
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"groupid": groupID,
						"comment": comment,
					},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		case http.MethodPut:
			// Update Group resource
			assert.Equal(t, "/access/groups/"+groupID, r.URL.Path)
			assert.Equal(t, updatedGroupComment, bodyData["comment"])

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"groupid": groupID,
					"comment": updatedGroupComment,
				},
			})

		case http.MethodDelete:
			// Delete Group resource
			assert.Equal(t, "/access/groups/"+groupID, r.URL.Path)

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
		"name":    property.New(groupID),
		"comment": property.New(updatedGroupComment),
	})

	// Run lifecycle test
	integration.LifeCycleTest{
		Resource: "pve:group:Group",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New(groupID),
				"comment": property.New(groupComment),
			}),
			Hook: func(in, out property.Map) {
				assert.Equal(t, groupID, out.Get("name").AsString())
				assert.Equal(t, groupComment, out.Get("comment").AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New(groupID),
				"comment": property.New(updatedGroupComment),
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
			assert.Equal(t, groupID, req.body["groupid"])
			assert.Equal(t, groupComment, req.body["comment"])
		case http.MethodPut:
			hasPUT = true
			assert.Equal(t, updatedGroupComment, req.body["comment"])
		case http.MethodDelete:
			hasDELETE = true
		}
	}

	assert.True(t, hasPOST, "Should have made POST request (create)")
	assert.True(t, hasPUT, "Should have made PUT request (update)")
	assert.True(t, hasDELETE, "Should have made DELETE request (cleanup)")
}

func TestGroupCreateLifecycleError(t *testing.T) {
	t.Parallel()
	var capture requestCapture

	// Create mock HTTP server that simulates an error on create
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
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Simulated API error on create",
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

	integration.LifeCycleTest{
		Resource: "pve:group:Group",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New(groupID),
				"comment": property.New(groupComment),
			}),
			ExpectFailure: true,
		},
	}.Run(t, pulumiServer)

	// Verify API calls
	requests := capture.get()
	require.Greater(t, len(requests), 0)

	var postCount int
	for _, req := range requests {
		if req.method == http.MethodPost {
			postCount++
			assert.Equal(t, "/access/groups", req.path)
			assert.Equal(t, groupID, req.body["groupid"])
			assert.Equal(t, groupComment, req.body["comment"])
		}
	}

	assert.Equal(t, 1, postCount, "Should have made exactly one POST request (create)")
}

func TestGroupUpdateLifecycleError(t *testing.T) {
	t.Parallel()
	var capture requestCapture
	var getCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

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
			assert.Equal(t, "/access/groups", r.URL.Path)
			assert.Equal(t, groupID, bodyData["groupid"])
			assert.Equal(t, groupComment, bodyData["comment"])

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"groupid": groupID,
					"comment": groupComment,
				},
			})

		case http.MethodGet:
			switch r.URL.Path {
			case "/access/groups/" + groupID:
				getCount++
				comment := groupComment
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{
						"groupid": groupID,
						"comment": comment,
					},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		case http.MethodPut:
			assert.Equal(t, "/access/groups/"+groupID, r.URL.Path)
			assert.Equal(t, updatedGroupComment, bodyData["comment"])

			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data":  nil,
				"error": "update failed",
			})

		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": nil,
			})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
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

	integration.LifeCycleTest{
		Resource: "pve:group:Group",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New(groupID),
				"comment": property.New(groupComment),
			}),
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New(groupID),
				"comment": property.New(updatedGroupComment),
			}),
			ExpectFailure: true,
		}},
	}.Run(t, pulumiServer)

	requests := capture.get()
	require.GreaterOrEqual(t, len(requests), 2)

	var hasPOST, hasPUT bool
	for _, req := range requests {
		switch req.method {
		case http.MethodPost:
			hasPOST = true
			assert.Equal(t, groupID, req.body["groupid"])
			assert.Equal(t, groupComment, req.body["comment"])
		case http.MethodPut:
			hasPUT = true
			assert.Equal(t, updatedGroupComment, req.body["comment"])
		}
	}
	assert.True(t, hasPOST, "Update lifecycle error test should perform a POST (create)")
	assert.True(t, hasPUT, "Update lifecycle error test should perform a PUT (update)")
}
