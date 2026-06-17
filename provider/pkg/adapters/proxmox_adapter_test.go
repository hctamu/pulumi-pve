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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

func TestProxmoxAdapterConnect(t *testing.T) {
	t.Parallel()

	t.Run("successful connection", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "https://test.proxmox.com:8006",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)
	})

	t.Run("connect is idempotent", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "https://test.proxmox.com:8006",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)

		// Connect multiple times
		err1 := adapter.Connect(context.Background())
		require.NoError(t, err1)

		// Second connect call should also succeed (idempotent)
		err2 := adapter.Connect(context.Background())
		require.NoError(t, err2)
	})

	t.Run("connection with empty URL succeeds but requests will fail", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)

		// Connect succeeds
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		// But actual requests will fail
		var result map[string]interface{}
		err = adapter.Get(context.Background(), "/test", &result)
		require.Error(t, err)
	})
}

func TestProxmoxAdapterGet(t *testing.T) {
	t.Parallel()

	t.Run("successful GET request", func(t *testing.T) {
		t.Parallel()

		// luthermonson/go-proxmox library unwraps the "data" field automatically
		innerData := map[string]interface{}{
			"status": "active",
			"name":   "test-resource",
		}

		// But the API returns data wrapped in a "data" field
		apiResponse := map[string]interface{}{
			"data": innerData,
		}

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
				// Verify request method and path (go-proxmox adds /api2/json prefix)
				assert.Equal(t, http.MethodGet, r.Method)
				// Note: The path passed to adapter.Get() doesn't include /api2/json
				// but the actual HTTP request will have it (added by go-proxmox library)

				// Verify headers
				assert.Contains(t, r.Header.Get("Authorization"), "PVEAPIToken")
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				// Send response (go-proxmox expects data wrapped in "data" field)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(apiResponse)
				require.NoError(t, err)
			},
		)
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		var result map[string]interface{}
		err = adapter.Get(context.Background(), "/test/resource", &result)
		require.NoError(t, err)

		// Verify response was unmarshaled correctly (go-proxmox unwraps "data" field)
		assert.Equal(t, innerData, result)

		// Verify captured request path
		assert.Equal(t, http.MethodGet, captured.Method)
		assert.Contains(t, captured.Path, "/test/resource") // Path contains our endpoint
	})

	t.Run("GET request with query parameters", func(t *testing.T) {
		t.Parallel()

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
				// Verify query parameters are preserved
				assert.Equal(t, "value1", r.URL.Query().Get("param1"))
				assert.Equal(t, "value2", r.URL.Query().Get("param2"))

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": {}}`))
			},
		)
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		var result map[string]interface{}
		err = adapter.Get(context.Background(), "/test/resource?param1=value1&param2=value2", &result)
		require.NoError(t, err)

		// Verify query parameters were sent
		assert.Equal(t, []string{"value1"}, captured.QueryParams["param1"])
		assert.Equal(t, []string{"value2"}, captured.QueryParams["param2"])
	})

	t.Run("GET request handles 500 error", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			// go-proxmox expects proper JSON response structure even for errors
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		var result map[string]interface{}
		err = adapter.Get(context.Background(), "/test/error", &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500") // Error should mention 500 status
	})
}

func TestProxmoxAdapterPost(t *testing.T) {
	t.Parallel()

	t.Run("successful POST request", func(t *testing.T) {
		t.Parallel()

		requestBody := map[string]interface{}{
			"name":   "test-resource",
			"status": "active",
			"count":  42,
		}

		// go-proxmox unwraps "data" field
		innerData := "created"
		apiResponse := map[string]interface{}{
			"data": innerData,
		}

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, req *testutils.MockRequest) {
				// Verify request method
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Contains(t, r.URL.Path, "/test/resources")

				// Verify headers
				assert.Contains(t, r.Header.Get("Authorization"), "PVEAPIToken")

				// Verify request body
				var receivedBody map[string]interface{}
				err := json.NewDecoder(strings.NewReader(req.Body)).Decode(&receivedBody)
				require.NoError(t, err)
				assert.Equal(t, "test-resource", receivedBody["name"])
				assert.Equal(t, "active", receivedBody["status"])
				assert.Equal(t, float64(42), receivedBody["count"]) // JSON numbers are float64

				// Send response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err = json.NewEncoder(w).Encode(apiResponse)
				require.NoError(t, err)
			},
		)
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		var result string
		err = adapter.Post(context.Background(), "/test/resources", requestBody, &result)
		require.NoError(t, err)

		// Verify response (go-proxmox unwraps "data")
		assert.Equal(t, innerData, result)

		// Verify captured request details
		assert.Equal(t, http.MethodPost, captured.Method)
		assert.Equal(t, "/test/resources", captured.Path)
		assert.Contains(t, captured.Body, "test-resource")
	})

	t.Run("POST request with nil result", func(t *testing.T) {
		t.Parallel()

		requestBody := map[string]string{"key": "value"}

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			w.WriteHeader(http.StatusNoContent)
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		err = adapter.Post(context.Background(), "/test/resources", requestBody, nil)
		require.NoError(t, err)
	})
}

func TestProxmoxAdapterPut(t *testing.T) {
	t.Parallel()

	t.Run("successful PUT request", func(t *testing.T) {
		t.Parallel()

		requestBody := map[string]interface{}{
			"status": "updated",
			"value":  100,
		}

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, req *testutils.MockRequest) {
				// Verify request method
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "/test/resources/123", r.URL.Path)

				// Verify headers
				assert.Contains(t, r.Header.Get("Authorization"), "PVEAPIToken")

				// Verify request body
				var receivedBody map[string]interface{}
				err := json.NewDecoder(strings.NewReader(req.Body)).Decode(&receivedBody)
				require.NoError(t, err)
				assert.Equal(t, "updated", receivedBody["status"])
				assert.Equal(t, float64(100), receivedBody["value"])

				// Send response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": "updated"}`))
			},
		)
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		var result string
		err = adapter.Put(context.Background(), "/test/resources/123", requestBody, &result)
		require.NoError(t, err)

		// Verify response
		assert.Equal(t, "updated", result)

		// Verify captured request
		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Equal(t, "/test/resources/123", captured.Path)
		assert.Contains(t, captured.Body, "updated")
	})

	t.Run("PUT request handles error", func(t *testing.T) {
		t.Parallel()

		requestBody := map[string]string{"key": "value"}

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors": "internal error"}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		var result map[string]interface{}
		err = adapter.Put(context.Background(), "/test/resources/123", requestBody, &result)
		require.Error(t, err)
	})

	t.Run("PUT request surfaces field-level validation errors", func(t *testing.T) {
		t.Parallel()

		requestBody := map[string]string{"fabric": "fakeRealFabric"}

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *testutils.MockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(
				`{"data":null,"errors":{"fabric":"fabric 'fakeRealFabric' does not exist"},` +
					`"message":"Parameter verification failed."}`,
			))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		err = adapter.Put(context.Background(), "/test/resources/123", requestBody, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Parameter verification failed.")
		assert.Contains(t, err.Error(), "fabric: fabric 'fakeRealFabric' does not exist")
	})
}

func TestProxmoxAdapterDelete(t *testing.T) {
	t.Parallel()

	t.Run("successful DELETE request", func(t *testing.T) {
		t.Parallel()

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
				// Verify request method
				assert.Equal(t, http.MethodDelete, r.Method)
				assert.Equal(t, "/test/resources/456", r.URL.Path)

				// Verify headers
				assert.Contains(t, r.Header.Get("Authorization"), "PVEAPIToken")

				// Send response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": "deleted"}`))
			},
		)
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		var result string
		err = adapter.Delete(context.Background(), "/test/resources/456", &result)
		require.NoError(t, err)

		// Verify response
		assert.Equal(t, "deleted", result)

		// Verify captured request
		assert.Equal(t, http.MethodDelete, captured.Method)
		assert.Contains(t, captured.Path, "/test/resources/456")
		assert.Empty(t, captured.Body) // DELETE should have no body
	})

	t.Run("DELETE request with nil result", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			w.WriteHeader(http.StatusNoContent)
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		err = adapter.Delete(context.Background(), "/test/resources/456", nil)
		require.NoError(t, err)
	})
}

func TestProxmoxAdapterAuthenticationHeaders(t *testing.T) {
	t.Parallel()

	t.Run("requests include correct authentication token", func(t *testing.T) {
		t.Parallel()

		expectedUser := "admin@pam"
		expectedToken := "secret-token-123"

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
				// Verify Authorization header format
				authHeader := r.Header.Get("Authorization")
				assert.Contains(t, authHeader, "PVEAPIToken")
				assert.Contains(t, authHeader, expectedUser)
				assert.Contains(t, authHeader, expectedToken)

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": {}}`))
			},
		)
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  expectedUser,
			PveToken: expectedToken,
		}

		adapter := adapters.NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		var result map[string]interface{}
		err = adapter.Get(context.Background(), "/test", &result)
		require.NoError(t, err)

		// Verify the authorization header was sent
		assert.Contains(t, captured.Headers.Get("Authorization"), "PVEAPIToken")
	})
}

func TestProxmoxAdapterConnectionFailureHandling(t *testing.T) {
	t.Parallel()

	t.Run("requests fail with invalid server URL", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "http://invalid-url-that-does-not-exist:9999",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)

		// Connect succeeds (doesn't validate URL)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		// But GET request fails
		var result map[string]interface{}
		err = adapter.Get(context.Background(), "/test", &result)
		require.Error(t, err)
	})

	t.Run("requests fail with empty URL", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := adapters.NewProxmoxAdapter(cfg)

		// Connect succeeds
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		// But requests fail
		var result map[string]interface{}

		err = adapter.Get(context.Background(), "/test", &result)
		require.Error(t, err)

		err = adapter.Post(context.Background(), "/test", map[string]string{"key": "value"}, &result)
		require.Error(t, err)

		err = adapter.Put(context.Background(), "/test", map[string]string{"key": "value"}, &result)
		require.Error(t, err)

		err = adapter.Delete(context.Background(), "/test", &result)
		require.Error(t, err)
	})
}

func TestProxmoxAdapterConfigFromContext(t *testing.T) {
	t.Parallel()

	t.Run("panics when config is nil and context has no Pulumi config", func(t *testing.T) {
		t.Parallel()

		// Create adapter without explicit config
		adapter := adapters.NewProxmoxAdapter(nil)

		// Attempting to connect with a plain context (no Pulumi config) should panic
		// because infer.GetConfig will panic when called on a non-Pulumi context
		assert.Panics(t, func() {
			_ = adapter.Connect(context.Background())
		}, "Expected panic when trying to get config from non-Pulumi context")
	})

	t.Run("works with explicit config even without Pulumi context", func(t *testing.T) {
		t.Parallel()

		// Create adapter with explicit config
		cfg := &config.Config{
			PveURL:   "https://test.proxmox.com:8006",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}
		adapter := adapters.NewProxmoxAdapter(cfg)

		// Should work fine even with plain context because config is explicit
		err := adapter.Connect(context.Background())
		require.NoError(t, err)
	})
}

func TestProxmoxAdapterResolveNode(t *testing.T) {
	t.Parallel()

	const (
		clusterOK = `{"data":[{"type":"cluster","quorate":1,"nodes":1},{"type":"node","name":"pve-node","online":1}]}`
	)

	tests := []struct {
		name           string
		inputNode      *string // nil means "let cluster decide"
		serverResponse string  // empty means no server needed
		serverStatus   int
		wantNode       string
		wantErr        bool
		wantErrContain string
	}{
		{
			name:      "returns provided node name unchanged",
			inputNode: func() *string { s := "my-node"; return &s }(),
			wantNode:  "my-node",
		},
		{
			name:           "fetches first cluster node when node is nil",
			inputNode:      nil,
			serverResponse: clusterOK,
			serverStatus:   http.StatusOK,
			wantNode:       "pve-node",
		},
		{
			name:         "returns error when cluster status request fails",
			inputNode:    nil,
			serverStatus: http.StatusInternalServerError,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var pveURL string
			if tt.inputNode != nil {
				// No HTTP server needed — ResolveNode returns early when node is non-nil.
				pveURL = "https://test.proxmox.com:8006"
			} else {
				server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.serverStatus)
					if tt.serverResponse != "" {
						_, _ = w.Write([]byte(tt.serverResponse))
					}
				})
				defer server.Close()
				pveURL = server.URL
			}

			adapter := adapters.NewProxmoxAdapter(&config.Config{
				PveURL:   pveURL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			})

			result, err := adapter.ResolveNode(context.Background(), tt.inputNode)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantNode, result)
			}
		})
	}
}

func TestProxmoxAdapterNode(t *testing.T) {
	t.Parallel()

	const (
		nodeOK = `{"data":{"node":"pve-node","status":"online","cpu":0.05,"maxcpu":8}}`
	)

	tests := []struct {
		name           string
		nodeName       string
		serverStatus   int
		serverResponse string
		wantErr        bool
	}{
		{
			name:           "returns node for a valid node name",
			nodeName:       "pve-node",
			serverStatus:   http.StatusOK,
			serverResponse: nodeOK,
		},
		{
			name:         "returns error when server responds with 500",
			nodeName:     "pve-node",
			serverStatus: http.StatusInternalServerError,
			wantErr:      true,
		},
		{
			// go-proxmox treats 400 as an error when "errors" key is present
			name:           "returns error when server responds with 400",
			nodeName:       "missing-node",
			serverStatus:   http.StatusBadRequest,
			serverResponse: `{"errors":{"node":"not found"}}`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, captured := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.serverStatus)
					if tt.serverResponse != "" {
						_, _ = w.Write([]byte(tt.serverResponse))
					}
				},
			)
			defer server.Close()

			adapter := adapters.NewProxmoxAdapter(&config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			})

			node, err := adapter.Node(context.Background(), tt.nodeName)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, node)
				// go-proxmox always sets node.Name from the argument passed to Node()
				assert.Equal(t, tt.nodeName, node.Name)
				assert.Equal(t, http.MethodGet, captured.Method)
				assert.Contains(t, captured.Path, "/nodes/"+tt.nodeName+"/status")
			}
		})
	}
}

func TestProxmoxAdapterFindVirtualMachineOnNode(t *testing.T) {
	t.Parallel()

	const (
		nodeOK = `{"data":{"node":"pve-node","status":"online"}}`
		vmOK   = `{"data":{"vmid":100,"name":"test-vm","status":"running"}}`
		vmConf = `{"data":{"vmid":100,"name":"test-vm","cores":2}}`
	)

	tests := []struct {
		name           string
		vmID           int
		nodeName       string
		nodeStatus     int
		nodeResponse   string
		vmStatus       int
		vmResponse     string
		wantErr        bool
		wantErrContain string
	}{
		{
			name:         "finds VM on node",
			vmID:         100,
			nodeName:     "pve-node",
			nodeStatus:   http.StatusOK,
			nodeResponse: nodeOK,
			vmStatus:     http.StatusOK,
			vmResponse:   vmOK,
		},
		{
			// go-proxmox returns error on 500 from node status
			name:       "returns error when node request fails",
			vmID:       100,
			nodeName:   "pve-node",
			nodeStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			// go-proxmox returns error on 500 from VM status/current;
			// node.VirtualMachine() returns nil, err so adapter propagates it
			name:         "returns error when VM status request fails",
			vmID:         999,
			nodeName:     "pve-node",
			nodeStatus:   http.StatusOK,
			nodeResponse: nodeOK,
			vmStatus:     http.StatusInternalServerError,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.Contains(r.URL.Path, fmt.Sprintf("/qemu/%d/config", tt.vmID)):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(vmConf))
					case strings.Contains(r.URL.Path, fmt.Sprintf("/qemu/%d/status/current", tt.vmID)):
						w.WriteHeader(tt.vmStatus)
						if tt.vmResponse != "" {
							_, _ = w.Write([]byte(tt.vmResponse))
						}
					case strings.Contains(r.URL.Path, "/nodes/"+tt.nodeName+"/status"):
						w.WriteHeader(tt.nodeStatus)
						if tt.nodeResponse != "" {
							_, _ = w.Write([]byte(tt.nodeResponse))
						}
					default:
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"data":null}`))
					}
				},
			)
			defer server.Close()

			adapter := adapters.NewProxmoxAdapter(&config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			})

			vm, node, err := adapter.FindVirtualMachineOnNode(context.Background(), tt.vmID, tt.nodeName)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, vm)
				require.NotNil(t, node)
				assert.Equal(t, tt.nodeName, node.Name)
				assert.Equal(t, strconv.Itoa(tt.vmID), fmt.Sprint(vm.VMID))
			}
		})
	}
}

func TestProxmoxAdapterFindVirtualMachine(t *testing.T) {
	t.Parallel()

	const (
		// go-proxmox Cluster() parses entries where type=="node" into Nodes slice
		clusterOK       = `{"data":[{"type":"cluster","quorate":1,"nodes":1},{"type":"node","name":"pve-node","online":1}]}`
		clusterTwoNodes = `{"data":[{"type":"cluster","quorate":1,"nodes":2},` +
			`{"type":"node","name":"wrong-node","online":1},{"type":"node","name":"pve-node","online":1}]}`
		nodeOK   = `{"data":{"node":"pve-node","status":"online"}}`
		vmStatus = `{"data":{"vmid":100,"name":"test-vm","status":"running"}}`
		vmConf   = `{"data":{"vmid":100,"name":"test-vm","cores":2}}`
	)

	tests := []struct {
		name           string
		vmID           int
		lastKnownNode  *string
		clusterResp    string
		wantErr        bool
		wantErrContain string
		wantNodeName   string
	}{
		{
			name:          "finds VM on last known node without scanning cluster",
			vmID:          100,
			lastKnownNode: func() *string { s := "pve-node"; return &s }(),
			clusterResp:   clusterOK,
			wantNodeName:  "pve-node",
		},
		{
			// wrong-node is in the cluster; VM status/current returns 500 on wrong-node,
			// so FindVirtualMachineOnNode returns an error and FindVirtualMachine falls
			// back to the full cluster scan, finding the VM on pve-node.
			name:          "falls back to cluster scan when VM not on last known node",
			vmID:          100,
			lastKnownNode: func() *string { s := "wrong-node"; return &s }(),
			clusterResp:   clusterTwoNodes,
			wantNodeName:  "pve-node",
		},
		{
			name:         "scans cluster when no last known node provided",
			vmID:         100,
			clusterResp:  clusterOK,
			wantNodeName: "pve-node",
		},
		{
			name:           "returns error when VM not found on any cluster node",
			vmID:           999,
			clusterResp:    clusterOK,
			wantErr:        true,
			wantErrContain: "not found on any nodes",
		},
		{
			// 500 from /cluster/status makes Cluster() return an error
			name:        "returns error when cluster status request fails",
			vmID:        100,
			clusterResp: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")

					switch {
					case strings.Contains(r.URL.Path, "/cluster/status"):
						if tt.clusterResp == "" {
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(tt.clusterResp))

					case strings.Contains(r.URL.Path, "/nodes/pve-node/status"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(nodeOK))

					case strings.Contains(r.URL.Path, "/nodes/wrong-node/status"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"data":{"node":"wrong-node","status":"online"}}`))

					// VM 100 on pve-node: success
					case strings.Contains(r.URL.Path, "/nodes/pve-node/qemu/100/config"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(vmConf))

					case strings.Contains(r.URL.Path, "/nodes/pve-node/qemu/100/status/current"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(vmStatus))

					default:
						// VM 100 on wrong-node, VM 999 on any node → 500 so VirtualMachine() returns nil, err
						w.WriteHeader(http.StatusInternalServerError)
					}
				},
			)
			defer server.Close()

			adapter := adapters.NewProxmoxAdapter(&config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			})

			foundVM, node, cluster, err := adapter.FindVirtualMachine(context.Background(), tt.vmID, tt.lastKnownNode)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, foundVM)
				require.NotNil(t, node)
				require.NotNil(t, cluster)
				assert.Equal(t, tt.wantNodeName, node.Name)
				assert.Equal(t, strconv.Itoa(tt.vmID), fmt.Sprint(foundVM.VMID))
			}
		})
	}
}

func TestProxmoxAdapterNextVMID(t *testing.T) {
	t.Parallel()

	const (
		clusterOK = `{"data":[{"type":"cluster","quorate":1,"nodes":1},{"type":"node","name":"pve-node","online":1}]}`
		nextIDOK  = `{"data":"100"}`
	)

	tests := []struct {
		name            string
		clusterResponse string
		clusterStatus   int
		nextIDResponse  string
		nextIDStatus    int
		wantVMID        int
		wantErr         bool
		wantErrContain  string
	}{
		{
			name:            "returns next VM ID from cluster",
			clusterResponse: clusterOK,
			clusterStatus:   http.StatusOK,
			nextIDResponse:  nextIDOK,
			nextIDStatus:    http.StatusOK,
			wantVMID:        100,
		},
		{
			name:          "returns error when cluster status request fails",
			clusterStatus: http.StatusInternalServerError,
			nextIDStatus:  http.StatusOK,
			wantErr:       true,
		},
		{
			name:            "returns error when next VM ID request fails",
			clusterResponse: clusterOK,
			clusterStatus:   http.StatusOK,
			nextIDStatus:    http.StatusInternalServerError,
			wantErr:         true,
			wantErrContain:  "next VM ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")
					if strings.Contains(r.URL.Path, "/cluster/nextid") {
						w.WriteHeader(tt.nextIDStatus)
						if tt.nextIDResponse != "" {
							_, _ = w.Write([]byte(tt.nextIDResponse))
						}
					} else {
						w.WriteHeader(tt.clusterStatus)
						if tt.clusterResponse != "" {
							_, _ = w.Write([]byte(tt.clusterResponse))
						}
					}
				},
			)
			defer server.Close()

			adapter := adapters.NewProxmoxAdapter(&config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			})

			vmID, err := adapter.NextVMID(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantVMID, vmID)
			}
		})
	}
}

func TestProxmoxAdapterWaitForTask(t *testing.T) {
	t.Parallel()

	// A syntactically valid UPID whose node field parses to "pve-node".
	// Format: UPID:{node}:{pid}:{pstart}:{starttime}:{type}:{id}:{user}@{realm}:
	const testUPID = "UPID:pve-node:00000001:00000001:00000001:qmdel:100:root@pam:"

	tests := []struct {
		name           string
		upid           string // empty string means nil task
		serverStatus   int    // 0 means no server needed
		serverResponse string
		wantErr        bool
		wantErrContain string
	}{
		{
			name: "empty UPID returns nil immediately",
			upid: "",
		},
		{
			name:         "task stopped with OK exit status returns nil",
			upid:         testUPID,
			serverStatus: http.StatusOK,
			serverResponse: `{"data":{"upid":"UPID:pve-node:00000001:00000001:00000001:qmdel:100:root@pam:",` +
				`"node":"pve-node","status":"stopped","exitstatus":"OK"}}`,
		},
		{
			// go-proxmox sets IsFailed=true for any exit status other than "OK".
			name:         "task stopped with non-OK exit status returns error",
			upid:         testUPID,
			serverStatus: http.StatusOK,
			serverResponse: `{"data":{"upid":"UPID:pve-node:00000001:00000001:00000001:qmdel:100:root@pam:",` +
				`"node":"pve-node","status":"stopped","exitstatus":"FAILED - exit code 1"}}`,
			wantErr:        true,
			wantErrContain: "task failed",
		},
		{
			// go-proxmox propagates HTTP 500 as an error from Ping → Wait.
			name:         "server error is propagated as error",
			upid:         testUPID,
			serverStatus: http.StatusInternalServerError,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.upid == "" {
				// Empty UPID path: no server needed
				adapter := adapters.NewProxmoxAdapter(nil)
				err := adapter.WaitForTask(context.Background(), "", time.Second, 0)
				require.NoError(t, err)
				return
			}

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.serverStatus)
					if tt.serverResponse != "" {
						_, _ = w.Write([]byte(tt.serverResponse))
					}
				},
			)
			defer server.Close()

			// Create adapter with config pointing to mock server
			cfg := &config.Config{
				PveURL:   server.URL,
				PveUser:  "user@pve!token",
				PveToken: "TOKEN",
			}
			adapter := adapters.NewProxmoxAdapter(cfg)

			// Pass 0 so WaitForTask uses the default 5s production interval.
			err := adapter.WaitForTask(context.Background(), tt.upid, 5*time.Second, 0)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProxmoxAdapterWaitForTask_Timeout(t *testing.T) {
	t.Parallel()

	// A syntactically valid UPID whose node field parses to "pve-node".
	const testUPID = "UPID:pve-node:00000001:00000001:00000001:qmdel:100:root@pam:"

	// The server always responds with status "running" so the task never completes.
	runningResponse := `{"data":{"upid":"UPID:pve-node:00000001:00000001:00000001:qmdel:100:root@pam:",` +
		`"node":"pve-node","status":"running"}}`

	server, _ := testutils.CreateMockServer(
		t,
		func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(runningResponse))
		},
	)
	defer server.Close()

	// Create adapter with config pointing to mock server
	cfg := &config.Config{
		PveURL:   server.URL,
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}
	adapter := adapters.NewProxmoxAdapter(cfg)

	// Use a short poll interval so the test completes quickly:
	// go-proxmox's Wait loop is: Ping → sleep(interval) → check timeout.
	// With interval=50ms and timeout=150ms the timeout fires after the first sleep.
	err := adapter.WaitForTask(context.Background(), testUPID, 150*time.Millisecond, 50*time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}
