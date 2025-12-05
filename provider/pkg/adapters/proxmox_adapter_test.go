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

package adapters

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRequest captures information about HTTP requests for verification
type mockRequest struct {
	Method      string
	Path        string
	Body        string
	Headers     http.Header
	QueryParams map[string][]string
}

// createMockServer creates a test HTTP server that captures requests and returns mock responses
func createMockServer(
	t *testing.T,
	handler func(w http.ResponseWriter, r *http.Request, captured *mockRequest),
) (*httptest.Server, *mockRequest) {
	t.Helper()
	var captured mockRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture request details
		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.Headers = r.Header.Clone()
		captured.QueryParams = r.URL.Query()

		// Read body
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			captured.Body = string(bodyBytes)
		}

		// Call handler
		handler(w, r, &captured)
	}))

	return server, &captured
}

func TestProxmoxAdapterConnect(t *testing.T) {
	t.Parallel()

	t.Run("successful connection", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "https://test.proxmox.com:8006",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, adapter.client)
	})

	t.Run("connect is idempotent", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "https://test.proxmox.com:8006",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)

		// Connect multiple times
		err1 := adapter.Connect(context.Background())
		require.NoError(t, err1)
		client1 := adapter.client

		err2 := adapter.Connect(context.Background())
		require.NoError(t, err2)
		client2 := adapter.client

		// Should be the same client instance
		assert.Same(t, client1, client2)
	})

	t.Run("connection with empty URL succeeds but requests will fail", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)

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

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
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
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)
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

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			// Verify query parameters are preserved
			assert.Equal(t, "value1", r.URL.Query().Get("param1"))
			assert.Equal(t, "value2", r.URL.Query().Get("param2"))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": {}}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)
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

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
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

		adapter := NewProxmoxAdapter(cfg)
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

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
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
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)
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

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			w.WriteHeader(http.StatusNoContent)
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)
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

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
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
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)
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

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors": "internal error"}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)
		err := adapter.Connect(context.Background())
		require.NoError(t, err)

		var result map[string]interface{}
		err = adapter.Put(context.Background(), "/test/resources/123", requestBody, &result)
		require.Error(t, err)
	})
}

func TestProxmoxAdapterDelete(t *testing.T) {
	t.Parallel()

	t.Run("successful DELETE request", func(t *testing.T) {
		t.Parallel()

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			// Verify request method
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "/test/resources/456", r.URL.Path)

			// Verify headers
			assert.Contains(t, r.Header.Get("Authorization"), "PVEAPIToken")

			// Send response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": "deleted"}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)
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

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			w.WriteHeader(http.StatusNoContent)
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		adapter := NewProxmoxAdapter(cfg)
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

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			// Verify Authorization header format
			authHeader := r.Header.Get("Authorization")
			assert.Contains(t, authHeader, "PVEAPIToken")
			assert.Contains(t, authHeader, expectedUser)
			assert.Contains(t, authHeader, expectedToken)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": {}}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  expectedUser,
			PveToken: expectedToken,
		}

		adapter := NewProxmoxAdapter(cfg)
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

		adapter := NewProxmoxAdapter(cfg)

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

		adapter := NewProxmoxAdapter(cfg)

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
		adapter := NewProxmoxAdapter(nil)

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
		adapter := NewProxmoxAdapter(cfg)

		// Should work fine even with plain context because config is explicit
		err := adapter.Connect(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, adapter.client)
	})
}
