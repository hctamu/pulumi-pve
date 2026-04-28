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
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

func TestPoolAdapterCreate(t *testing.T) {
	t.Parallel()

	t.Run("successful create with comment", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.PoolInputs{
			Name:    "test-pool",
			Comment: "Test pool comment",
		}

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, capturedReq *testutils.MockRequest) {
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

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
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

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, capturedReq *testutils.MockRequest) {
				// Verify request body
				var receivedBody map[string]interface{}
				err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&receivedBody)
				require.NoError(t, err)
				assert.Equal(t, "simple-pool", receivedBody["poolid"])
				assert.Equal(t, "", receivedBody["comment"])

				// Send response
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
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

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
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

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors": "pool already exists"}`))
		})
		defer server.Close()

		cfg := &config.Config{
			PveURL:   server.URL,
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
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

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
				// Verify request method and path
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/pools/", r.URL.Path)
				assert.Equal(t, "test-pool", r.URL.Query().Get("poolid"))

				// Send response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				responseData := map[string]interface{}{
					"data": []interface{}{apiResponse},
				}
				err := json.NewEncoder(w).Encode(responseData)
				require.NoError(t, err)
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

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
		outputs, err := poolAdapter.Get(context.Background(), "test-pool")
		require.NoError(t, err)
		require.NotNil(t, outputs)

		// Verify outputs
		assert.Equal(t, "test-pool", outputs.Name)
		assert.Equal(t, "Pool comment", outputs.Comment)

		// Verify captured request
		assert.Equal(t, http.MethodGet, captured.Method)
		assert.Equal(t, "/pools/", captured.Path)
	})

	t.Run("successful get without comment", func(t *testing.T) {
		t.Parallel()

		apiResponse := map[string]interface{}{
			"poolid":  "simple-pool",
			"comment": "",
		}

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			assert.Equal(t, "/pools/", r.URL.Path)
			assert.Equal(t, "simple-pool", r.URL.Query().Get("poolid"))

			w.WriteHeader(http.StatusOK)
			responseData := map[string]interface{}{
				"data": []interface{}{apiResponse},
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

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
		outputs, err := poolAdapter.Get(context.Background(), "simple-pool")
		require.NoError(t, err)
		require.NotNil(t, outputs)

		assert.Equal(t, "simple-pool", outputs.Name)
		assert.Empty(t, outputs.Comment)
	})

	t.Run("get handles API error", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
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

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
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

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			w.WriteHeader(http.StatusOK)
			responseData := map[string]interface{}{
				"data": []interface{}{apiResponse},
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

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
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
		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, req *testutils.MockRequest) {
				requestCount++

				if requestCount == 1 {
					// First request: GET pool
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, "/pools/", r.URL.Path)
					assert.Equal(t, "test-pool", r.URL.Query().Get("poolid"))

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					responseData := map[string]interface{}{
						"data": []interface{}{map[string]interface{}{
							"poolid":  "test-pool",
							"comment": "Old comment",
						}},
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

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Update(context.Background(), "test-pool", proxmox.PoolInputs{}, inputs)
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
		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, req *testutils.MockRequest) {
				requestCount++

				if requestCount == 1 {
					// First request: GET pool
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					responseData := map[string]interface{}{
						"data": []interface{}{map[string]interface{}{
							"poolid":  "test-pool",
							"comment": "Old comment",
						}},
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

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Update(context.Background(), "test-pool", proxmox.PoolInputs{Comment: "Old comment"}, inputs)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPut, captured.Method)
	})

	t.Run("update handles get error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.PoolInputs{
			Name:    "nonexistent-pool",
			Comment: "New comment",
		}

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
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

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Update(context.Background(), "nonexistent-pool", proxmox.PoolInputs{}, inputs)
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
		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			requestCount++

			if requestCount == 1 {
				// First request: GET succeeds
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				responseData := map[string]interface{}{
					"data": []interface{}{map[string]interface{}{
						"poolid":  "test-pool",
						"comment": "Old comment",
					}},
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

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Update(context.Background(), "test-pool", proxmox.PoolInputs{}, inputs)
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
		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, req *testutils.MockRequest) {
				requestCount++

				if requestCount == 1 {
					// First request: GET pool
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, "/pools/", r.URL.Path)
					assert.Equal(t, "test-pool", r.URL.Query().Get("poolid"))

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					responseData := map[string]interface{}{
						"data": []interface{}{map[string]interface{}{
							"poolid":  "test-pool",
							"comment": "Test comment",
						}},
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

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Delete(context.Background(), "test-pool")
		require.NoError(t, err)

		// Verify last captured request (the DELETE)
		assert.Equal(t, http.MethodDelete, captured.Method)
		assert.Equal(t, "/pools/test-pool", captured.Path)
		assert.Empty(t, captured.Body)
	})

	t.Run("delete handles get error", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
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

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
		err = poolAdapter.Delete(context.Background(), "nonexistent-pool")
		require.Error(t, err)
		assert.EqualError(t, err, "failed to get Pool resource for deletion: 500 Internal Server Error")
	})

	t.Run("delete handles delete error", func(t *testing.T) {
		t.Parallel()

		requestCount := 0
		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
			requestCount++

			if requestCount == 1 {
				// GET succeeds
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				responseData := map[string]interface{}{
					"data": []interface{}{map[string]interface{}{
						"poolid":  "test-pool",
						"comment": "Test",
					}},
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

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
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

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
		require.NotNil(t, poolAdapter)
	})
}

func TestPoolAdapterCreateWithMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputs      proxmox.PoolInputs
		wantVMs     string
		wantStorage string
	}{
		{
			name: "create with vms and storage",
			inputs: proxmox.PoolInputs{
				Name:    "test-pool",
				Comment: "comment",
				VMs:     []int{100, 101},
				Storage: []string{"local", "local-lvm"},
			},
			wantVMs:     "100,101",
			wantStorage: "local,local-lvm",
		},
		{
			name: "create with vms only",
			inputs: proxmox.PoolInputs{
				Name: "vms-pool",
				VMs:  []int{200},
			},
			wantVMs:     "200",
			wantStorage: "",
		},
		{
			name: "create with storage only",
			inputs: proxmox.PoolInputs{
				Name:    "storage-pool",
				Storage: []string{"ceph"},
			},
			wantVMs:     "",
			wantStorage: "ceph",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requestCount := 0
			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, req *testutils.MockRequest) {
					requestCount++
					switch requestCount {
					case 1:
						// POST /pools — NewPool
						assert.Equal(t, http.MethodPost, r.Method)
						assert.Equal(t, "/pools", r.URL.Path)
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"data": null}`))
					case 2:
						// GET /pools/?poolid={name} — fetch pool object
						assert.Equal(t, http.MethodGet, r.Method)
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						err := json.NewEncoder(w).Encode(map[string]interface{}{
							"data": []interface{}{map[string]interface{}{
								"poolid":  tt.inputs.Name,
								"comment": tt.inputs.Comment,
							}},
						})
						require.NoError(t, err)
					case 3:
						// PUT /pools/{name} — add members
						assert.Equal(t, http.MethodPut, r.Method)
						var body map[string]interface{}
						err := json.NewDecoder(strings.NewReader(req.Body)).Decode(&body)
						require.NoError(t, err)
						if tt.wantVMs != "" {
							assert.Equal(t, tt.wantVMs, body["vms"])
						}
						if tt.wantStorage != "" {
							assert.Equal(t, tt.wantStorage, body["storage"])
						}
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"data": null}`))
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

			poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
			err = poolAdapter.Create(context.Background(), tt.inputs)
			require.NoError(t, err)
			assert.Equal(t, 3, requestCount)
		})
	}
}

func TestPoolAdapterGetWithMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		apiMembers  []map[string]interface{}
		wantVMs     []int
		wantStorage []string
	}{
		{
			name: "get pool with qemu and storage members",
			apiMembers: []map[string]interface{}{
				{"id": "qemu/100", "type": "qemu", "vmid": 100, "node": "pve1", "status": "running"},
				{"id": "qemu/101", "type": "qemu", "vmid": 101, "node": "pve1", "status": "stopped"},
				{"id": "storage/pve1/local", "type": "storage", "storage": "local", "node": "pve1"},
			},
			wantVMs:     []int{100, 101},
			wantStorage: []string{"local"},
		},
		{
			name: "get pool with lxc member",
			apiMembers: []map[string]interface{}{
				{"id": "lxc/200", "type": "lxc", "vmid": 200, "node": "pve1", "status": "running"},
			},
			wantVMs:     []int{200},
			wantStorage: nil,
		},
		{
			name:        "get pool with no members",
			apiMembers:  []map[string]interface{}{},
			wantVMs:     nil,
			wantStorage: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					err := json.NewEncoder(w).Encode(map[string]interface{}{
						"data": []interface{}{map[string]interface{}{
							"poolid":  "test-pool",
							"comment": "test",
							"members": tt.apiMembers,
						}},
					})
					require.NoError(t, err)
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

			poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
			outputs, err := poolAdapter.Get(context.Background(), "test-pool")
			require.NoError(t, err)
			require.NotNil(t, outputs)

			assert.Equal(t, tt.wantVMs, outputs.VMs)
			assert.Equal(t, tt.wantStorage, outputs.Storage)
		})
	}
}

func TestPoolAdapterUpdateWithMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		inputs        proxmox.PoolInputs
		state         proxmox.PoolInputs
		wantRequests  int
		wantAddVMs    string
		wantRemoveVMs string
	}{
		{
			name: "add new VMs to pool",
			inputs: proxmox.PoolInputs{
				Name:    "test-pool",
				Comment: "updated",
				VMs:     []int{100, 101},
			},
			state:        proxmox.PoolInputs{},
			wantRequests: 2, // GET + PUT add
			wantAddVMs:   "100,101",
		},
		{
			name: "remove VMs from pool",
			inputs: proxmox.PoolInputs{
				Name:    "test-pool",
				Comment: "updated",
				VMs:     []int{},
			},
			state: proxmox.PoolInputs{
				VMs: []int{100},
			},
			wantRequests:  3, // GET + PUT add (comment changed) + PUT remove
			wantRemoveVMs: "100",
		},
		{
			name: "add and remove VMs",
			inputs: proxmox.PoolInputs{
				Name: "test-pool",
				VMs:  []int{102},
			},
			state: proxmox.PoolInputs{
				VMs: []int{100},
			},
			wantRequests:  3, // GET + PUT add 102 + PUT remove 100
			wantAddVMs:    "102",
			wantRemoveVMs: "100",
		},
		{
			name: "no member changes updates only comment",
			inputs: proxmox.PoolInputs{
				Name:    "test-pool",
				Comment: "new comment",
				VMs:     []int{100},
			},
			state: proxmox.PoolInputs{
				VMs: []int{100},
			},
			wantRequests: 2, // GET + PUT comment only
		},
		{
			name: "no changes at all — no PUT issued",
			inputs: proxmox.PoolInputs{
				Name:    "test-pool",
				Comment: "same",
				VMs:     []int{100},
				Storage: []string{"local"},
			},
			state: proxmox.PoolInputs{
				Comment: "same",
				VMs:     []int{100},
				Storage: []string{"local"},
			},
			wantRequests: 1, // GET only
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requestCount := 0
			var putBodies []map[string]interface{}

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, req *testutils.MockRequest) {
					requestCount++
					if r.Method == http.MethodGet {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						err := json.NewEncoder(w).Encode(map[string]interface{}{
							"data": []interface{}{map[string]interface{}{
								"poolid":  tt.inputs.Name,
								"comment": "old",
							}},
						})
						require.NoError(t, err)
					} else {
						var body map[string]interface{}
						_ = json.NewDecoder(strings.NewReader(req.Body)).Decode(&body)
						putBodies = append(putBodies, body)
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"data": null}`))
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

			poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)
			err = poolAdapter.Update(context.Background(), tt.inputs.Name, tt.state, tt.inputs)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRequests, requestCount)

			if tt.wantAddVMs != "" {
				require.NotEmpty(t, putBodies)
				assert.Equal(t, tt.wantAddVMs, putBodies[0]["vms"])
			}

			if tt.wantRemoveVMs != "" {
				lastPut := putBodies[len(putBodies)-1]
				assert.Equal(t, tt.wantRemoveVMs, lastPut["vms"])
				deleteVal, _ := lastPut["delete"].(bool)
				assert.True(t, deleteVal)
			}
		})
	}
}
