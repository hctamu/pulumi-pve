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
	"net/http"
	"strings"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolAdapterCreate(t *testing.T) {
	t.Parallel()

	t.Run("successful create with comment", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.PoolInputs{
			Name:    "test-pool",
			Comment: "Test pool comment",
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, capturedReq *mockRequest) {
			// Verify request method and path
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/pools", r.URL.Path)

			// Verify request body
			var receivedBody map[string]interface{}
			err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&receivedBody)
			require.NoError(t, err)
			assert.Equal(t, "test-pool", receivedBody["poolid"])
			assert.Equal(t, "Test pool comment", receivedBody["comment"])

			// Send response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Create(context.Background(), inputs)
		require.NoError(t, err)

		// Verify captured request
		assert.Equal(t, http.MethodPost, captured.Method)
		assert.Equal(t, "/pools", captured.Path)
		assert.Contains(t, captured.Body, "test-pool")
		assert.Contains(t, captured.Body, "Test pool comment")
	})

	t.Run("successful create without comment", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.PoolInputs{
			Name:    "simple-pool",
			Comment: "",
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, capturedReq *mockRequest) {
			// Verify request body
			var receivedBody map[string]interface{}
			err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&receivedBody)
			require.NoError(t, err)
			assert.Equal(t, "simple-pool", receivedBody["poolid"])
			assert.Equal(t, "", receivedBody["comment"])

			// Send response
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Create(context.Background(), inputs)
		require.NoError(t, err)

		// Verify captured request
		assert.Equal(t, http.MethodPost, captured.Method)
		assert.Contains(t, captured.Body, "simple-pool")
	})

	t.Run("create handles API error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.PoolInputs{
			Name:    "test-pool",
			Comment: "Test comment",
		}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors": "pool already exists"}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Create(context.Background(), inputs)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to create Pool resource: 500 Internal Server Error")
	})
}

func TestPoolAdapterGet(t *testing.T) {
	t.Parallel()

	t.Run("successful get with comment", func(t *testing.T) {
		t.Parallel()

		apiResponse := map[string]interface{}{
			"poolid":  "test-pool",
			"comment": "Pool comment",
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			// Verify request method and path
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/pools/test-pool", r.URL.Path)

			// Send response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			responseData := map[string]interface{}{
				"data": apiResponse,
			}
			err := json.NewEncoder(w).Encode(responseData)
			require.NoError(t, err)
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		outputs, err := poolAdapter.Get(context.Background(), "test-pool")
		require.NoError(t, err)
		require.NotNil(t, outputs)

		// Verify outputs
		assert.Equal(t, "test-pool", outputs.Name)
		assert.Equal(t, "Pool comment", outputs.Comment)

		// Verify captured request
		assert.Equal(t, http.MethodGet, captured.Method)
		assert.Equal(t, "/pools/test-pool", captured.Path)
	})

	t.Run("successful get without comment", func(t *testing.T) {
		t.Parallel()

		apiResponse := map[string]interface{}{
			"poolid":  "simple-pool",
			"comment": "",
		}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			assert.Equal(t, "/pools/simple-pool", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			responseData := map[string]interface{}{
				"data": apiResponse,
			}
			err := json.NewEncoder(w).Encode(responseData)
			require.NoError(t, err)
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		outputs, err := poolAdapter.Get(context.Background(), "simple-pool")
		require.NoError(t, err)
		require.NotNil(t, outputs)

		assert.Equal(t, "simple-pool", outputs.Name)
		assert.Empty(t, outputs.Comment)
	})

	t.Run("get handles API error", func(t *testing.T) {
		t.Parallel()

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		outputs, err := poolAdapter.Get(context.Background(), "nonexistent-pool")
		require.Error(t, err)
		assert.Nil(t, outputs)
		assert.EqualError(t, err, "failed to get Pool resource: 500 Internal Server Error")
	})

	t.Run("get with long comment", func(t *testing.T) {
		t.Parallel()

		longComment := "This is a very long comment that describes the pool in detail"
		apiResponse := map[string]interface{}{
			"poolid":  "detailed-pool",
			"comment": longComment,
		}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			w.WriteHeader(http.StatusOK)
			responseData := map[string]interface{}{
				"data": apiResponse,
			}
			err := json.NewEncoder(w).Encode(responseData)
			require.NoError(t, err)
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		outputs, err := poolAdapter.Get(context.Background(), "detailed-pool")
		require.NoError(t, err)
		assert.Equal(t, longComment, outputs.Comment)
	})
}

func TestPoolAdapterUpdate(t *testing.T) {
	t.Parallel()

	t.Run("update comment", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.PoolInputs{
			Name:    "test-pool",
			Comment: "Updated comment",
		}

		// First request is GET to fetch the pool
		// Second request is PUT to update it
		requestCount := 0
		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			requestCount++

			if requestCount == 1 {
				// First request: GET pool
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/pools/test-pool", r.URL.Path)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				responseData := map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  "test-pool",
						"comment": "Old comment",
					},
				}
				err := json.NewEncoder(w).Encode(responseData)
				require.NoError(t, err)
			} else {
				// Second request: PUT update
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "/pools/test-pool", r.URL.Path)

				// Verify request body
				var receivedBody map[string]interface{}
				err := json.NewDecoder(strings.NewReader(req.Body)).Decode(&receivedBody)
				require.NoError(t, err)
				assert.Equal(t, "Updated comment", receivedBody["comment"])

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			}
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Update(context.Background(), "test-pool", inputs)
		require.NoError(t, err)

		// Verify last captured request (the PUT)
		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Equal(t, "/pools/test-pool", captured.Path)
		assert.Contains(t, captured.Body, "Updated comment")
	})

	t.Run("update with empty comment", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.PoolInputs{
			Name:    "test-pool",
			Comment: "",
		}

		requestCount := 0
		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			requestCount++

			if requestCount == 1 {
				// First request: GET pool
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				responseData := map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  "test-pool",
						"comment": "Old comment",
					},
				}
				err := json.NewEncoder(w).Encode(responseData)
				require.NoError(t, err)
			} else {
				// Second request: PUT update
				var receivedBody map[string]interface{}
				err := json.NewDecoder(strings.NewReader(req.Body)).Decode(&receivedBody)
				require.NoError(t, err)
				// Comment field may not be present due to omitempty tag when empty
				comment, ok := receivedBody["comment"]
				if ok {
					// If present, should be empty string
					assert.Equal(t, "", comment)
				}
				// Field not being present is also acceptable for empty strings

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			}
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Update(context.Background(), "test-pool", inputs)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPut, captured.Method)
	})

	t.Run("update handles get error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.PoolInputs{
			Name:    "nonexistent-pool",
			Comment: "New comment",
		}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			// GET fails
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

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Update(context.Background(), "nonexistent-pool", inputs)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to get Pool resource for update: 500 Internal Server Error")
	})

	t.Run("update handles put error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.PoolInputs{
			Name:    "test-pool",
			Comment: "Updated comment",
		}

		requestCount := 0
		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			requestCount++

			if requestCount == 1 {
				// First request: GET succeeds
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				responseData := map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  "test-pool",
						"comment": "Old comment",
					},
				}
				err := json.NewEncoder(w).Encode(responseData)
				require.NoError(t, err)
			} else {
				// Second request: PUT fails
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors": "update failed"}`))
			}
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Update(context.Background(), "test-pool", inputs)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to update Pool resource: 500 Internal Server Error")
	})
}

func TestPoolAdapterDelete(t *testing.T) {
	t.Parallel()

	t.Run("successful delete", func(t *testing.T) {
		t.Parallel()

		// First request is GET to fetch the pool
		// Second request is DELETE
		requestCount := 0
		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			requestCount++

			if requestCount == 1 {
				// First request: GET pool
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/pools/test-pool", r.URL.Path)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				responseData := map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  "test-pool",
						"comment": "Test comment",
					},
				}
				err := json.NewEncoder(w).Encode(responseData)
				require.NoError(t, err)
			} else {
				// Second request: DELETE
				assert.Equal(t, http.MethodDelete, r.Method)
				assert.Equal(t, "/pools/test-pool", r.URL.Path)

				// Verify no body
				assert.Empty(t, req.Body)

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			}
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Delete(context.Background(), "test-pool")
		require.NoError(t, err)

		// Verify last captured request (the DELETE)
		assert.Equal(t, http.MethodDelete, captured.Method)
		assert.Equal(t, "/pools/test-pool", captured.Path)
		assert.Empty(t, captured.Body)
	})

	t.Run("delete handles get error", func(t *testing.T) {
		t.Parallel()

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			// GET fails
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

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Delete(context.Background(), "nonexistent-pool")
		require.Error(t, err)
		assert.EqualError(t, err, "failed to get Pool resource for deletion: 500 Internal Server Error")
	})

	t.Run("delete handles delete error", func(t *testing.T) {
		t.Parallel()

		requestCount := 0
		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			requestCount++

			if requestCount == 1 {
				// GET succeeds
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				responseData := map[string]interface{}{
					"data": map[string]interface{}{
						"poolid":  "test-pool",
						"comment": "Test",
					},
				}
				err := json.NewEncoder(w).Encode(responseData)
				require.NoError(t, err)
			} else {
				// DELETE fails
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors": "delete failed"}`))
			}
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Delete(context.Background(), "test-pool")
		require.Error(t, err)
		assert.EqualError(t, err, "failed to delete Pool resource: 500 Internal Server Error")
	})
}

func TestNewPoolAdapter(t *testing.T) {
	t.Parallel()

	t.Run("creates adapter with valid client", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "https://test.proxmox.com:8006",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := NewPoolAdapter(proxmoxAdapter)
		require.NotNil(t, poolAdapter)
		assert.NotNil(t, poolAdapter.proxmoxAdapter)
	})
}
