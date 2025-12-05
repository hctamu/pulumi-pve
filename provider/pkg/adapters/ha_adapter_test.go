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

func TestHAAdapterCreate(t *testing.T) {
	t.Parallel()

	t.Run("successful create with group", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.HAInputs{
			Group:      "test-group",
			State:      proxmox.HAStateStarted,
			ResourceID: 100,
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, capturedReq *mockRequest) {
			// Verify request method and path
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/cluster/ha/resources/", r.URL.Path)

			// Verify request body
			var receivedBody proxmox.HaResource
			err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&receivedBody)
			require.NoError(t, err)
			assert.Equal(t, "test-group", receivedBody.Group)
			assert.Equal(t, "started", receivedBody.State)
			assert.Equal(t, "100", receivedBody.Sid)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Create(context.Background(), inputs)
		require.NoError(t, err)

		// Verify captured request
		assert.Equal(t, http.MethodPost, captured.Method)
		assert.Equal(t, "/cluster/ha/resources/", captured.Path)
		assert.Contains(t, captured.Body, "test-group")
		assert.Contains(t, captured.Body, "started")
		assert.Contains(t, captured.Body, "100")
	})

	t.Run("successful create without group", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.HAInputs{
			Group:      "",
			State:      proxmox.HAStateStopped,
			ResourceID: 101,
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, capturedReq *mockRequest) {
			// Verify request body
			var receivedBody proxmox.HaResource
			err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&receivedBody)
			require.NoError(t, err)
			assert.Empty(t, receivedBody.Group)
			assert.Equal(t, "stopped", receivedBody.State)
			assert.Equal(t, "101", receivedBody.Sid)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Create(context.Background(), inputs)
		require.NoError(t, err)

		// Verify captured request
		assert.Equal(t, http.MethodPost, captured.Method)
		assert.NotContains(t, captured.Body, "test-group")
		assert.Contains(t, captured.Body, "stopped")
	})

	t.Run("create with ignored state", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.HAInputs{
			Group:      "ha-group",
			State:      proxmox.HAStateIgnored,
			ResourceID: 102,
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, capturedReq *mockRequest) {
			var receivedBody proxmox.HaResource
			err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&receivedBody)
			require.NoError(t, err)
			assert.Equal(t, "ignored", receivedBody.State)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Create(context.Background(), inputs)
		require.NoError(t, err)

		assert.Contains(t, captured.Body, "ignored")
	})

	t.Run("create handles API error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.HAInputs{
			Group:      "test-group",
			State:      proxmox.HAStateStarted,
			ResourceID: 100,
		}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors": "resource already exists"}`))
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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Create(context.Background(), inputs)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to create HA resource: 500 Internal Server Error", "error message should match")
	})
}

func TestHAAdapterGet(t *testing.T) {
	t.Parallel()

	t.Run("successful get with group", func(t *testing.T) {
		t.Parallel()

		apiResponse := proxmox.HaResource{
			Group: "test-group",
			State: "started",
			Sid:   "100",
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			// Verify request method and path
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/cluster/ha/resources/100", r.URL.Path)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		outputs, err := haAdapter.Get(context.Background(), 100)
		require.NoError(t, err)
		require.NotNil(t, outputs)

		// Verify outputs
		assert.Equal(t, "test-group", outputs.Group)
		assert.Equal(t, proxmox.HAStateStarted, outputs.State)
		assert.Equal(t, 100, outputs.ResourceID)

		// Verify captured request
		assert.Equal(t, http.MethodGet, captured.Method)
		assert.Equal(t, "/cluster/ha/resources/100", captured.Path)
	})

	t.Run("successful get without group", func(t *testing.T) {
		t.Parallel()

		apiResponse := proxmox.HaResource{
			Group: "",
			State: "stopped",
			Sid:   "101",
		}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			assert.Equal(t, "/cluster/ha/resources/101", r.URL.Path)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		outputs, err := haAdapter.Get(context.Background(), 101)
		require.NoError(t, err)
		require.NotNil(t, outputs)

		assert.Empty(t, outputs.Group)
		assert.Equal(t, proxmox.HAStateStopped, outputs.State)
		assert.Equal(t, 101, outputs.ResourceID)
	})

	t.Run("get handles API error", func(t *testing.T) {
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

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		haAdapter := NewHAAdapter(proxmoxAdapter)
		outputs, err := haAdapter.Get(context.Background(), 999)
		require.Error(t, err)
		assert.Nil(t, outputs)
		assert.EqualError(t, err, "failed to get HA resource: 500 Internal Server Error", "error message should match")
	})

	t.Run("get with ignored state", func(t *testing.T) {
		t.Parallel()

		apiResponse := proxmox.HaResource{
			Group: "ha-group",
			State: "ignored",
			Sid:   "102",
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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		outputs, err := haAdapter.Get(context.Background(), 102)
		require.NoError(t, err)
		assert.Equal(t, proxmox.HAStateIgnored, outputs.State)
	})
}

func TestHAAdapterUpdate(t *testing.T) {
	t.Parallel()

	t.Run("update state only", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.HAInputs{
			Group:      "test-group",
			State:      proxmox.HAStateStopped,
			ResourceID: 100,
		}

		oldOutputs := proxmox.HAOutputs{
			HAInputs: proxmox.HAInputs{
				Group:      "test-group",
				State:      proxmox.HAStateStarted,
				ResourceID: 100,
			},
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			// Verify request method and path
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Equal(t, "/cluster/ha/resources/100", r.URL.Path)

			// Verify request body
			var receivedBody proxmox.HaResource
			err := json.NewDecoder(strings.NewReader(req.Body)).Decode(&receivedBody)
			require.NoError(t, err)
			assert.Equal(t, "stopped", receivedBody.State)
			assert.Equal(t, "test-group", receivedBody.Group)
			assert.Empty(t, receivedBody.Delete)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Update(context.Background(), 100, inputs, oldOutputs)
		require.NoError(t, err)

		// Verify captured request
		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Equal(t, "/cluster/ha/resources/100", captured.Path)
		assert.Contains(t, captured.Body, "stopped")
		assert.Contains(t, captured.Body, "test-group")
	})

	t.Run("update with group removal", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.HAInputs{
			Group:      "",
			State:      proxmox.HAStateStarted,
			ResourceID: 100,
		}

		oldOutputs := proxmox.HAOutputs{
			HAInputs: proxmox.HAInputs{
				Group:      "test-group",
				State:      proxmox.HAStateStarted,
				ResourceID: 100,
			},
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			// Verify request body contains delete field
			var receivedBody proxmox.HaResource
			err := json.NewDecoder(strings.NewReader(req.Body)).Decode(&receivedBody)
			require.NoError(t, err)
			assert.Equal(t, "started", receivedBody.State)
			assert.Equal(t, []string{"group"}, receivedBody.Delete)
			assert.Empty(t, receivedBody.Group)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Update(context.Background(), 100, inputs, oldOutputs)
		require.NoError(t, err)

		// Verify captured request
		assert.Contains(t, captured.Body, "delete")
		assert.Contains(t, captured.Body, "group")
	})

	t.Run("update with group addition", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.HAInputs{
			Group:      "new-group",
			State:      proxmox.HAStateStarted,
			ResourceID: 100,
		}

		oldOutputs := proxmox.HAOutputs{
			HAInputs: proxmox.HAInputs{
				Group:      "",
				State:      proxmox.HAStateStarted,
				ResourceID: 100,
			},
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			var receivedBody proxmox.HaResource
			err := json.NewDecoder(strings.NewReader(req.Body)).Decode(&receivedBody)
			require.NoError(t, err)
			assert.Equal(t, "new-group", receivedBody.Group)
			assert.Equal(t, "started", receivedBody.State)
			assert.Empty(t, receivedBody.Delete)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Update(context.Background(), 100, inputs, oldOutputs)
		require.NoError(t, err)

		assert.Contains(t, captured.Body, "new-group")
		assert.NotContains(t, captured.Body, "delete")
	})

	t.Run("update with group change", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.HAInputs{
			Group:      "new-group",
			State:      proxmox.HAStateIgnored,
			ResourceID: 100,
		}

		oldOutputs := proxmox.HAOutputs{
			HAInputs: proxmox.HAInputs{
				Group:      "old-group",
				State:      proxmox.HAStateStarted,
				ResourceID: 100,
			},
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			var receivedBody proxmox.HaResource
			err := json.NewDecoder(strings.NewReader(req.Body)).Decode(&receivedBody)
			require.NoError(t, err)
			assert.Equal(t, "new-group", receivedBody.Group)
			assert.Equal(t, "ignored", receivedBody.State)
			assert.Empty(t, receivedBody.Delete)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Update(context.Background(), 100, inputs, oldOutputs)
		require.NoError(t, err)

		assert.Contains(t, captured.Body, "new-group")
		assert.NotContains(t, captured.Body, "old-group")
		assert.Contains(t, captured.Body, "ignored")
	})

	t.Run("update handles API error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.HAInputs{
			Group:      "test-group",
			State:      proxmox.HAStateStopped,
			ResourceID: 100,
		}

		oldOutputs := proxmox.HAOutputs{
			HAInputs: proxmox.HAInputs{
				Group:      "test-group",
				State:      proxmox.HAStateStarted,
				ResourceID: 100,
			},
		}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors": "update failed"}`))
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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Update(context.Background(), 100, inputs, oldOutputs)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to update HA resource: 500 Internal Server Error", "error message should match")
	})
}

func TestHAAdapterDelete(t *testing.T) {
	t.Parallel()

	t.Run("successful delete", func(t *testing.T) {
		t.Parallel()

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			// Verify request method and path
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "/cluster/ha/resources/100", r.URL.Path)

			// Verify no body
			assert.Empty(t, req.Body)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Delete(context.Background(), 100)
		require.NoError(t, err)

		// Verify captured request
		assert.Equal(t, http.MethodDelete, captured.Method)
		assert.Equal(t, "/cluster/ha/resources/100", captured.Path)
		assert.Empty(t, captured.Body)
	})

	t.Run("delete different resource ID", func(t *testing.T) {
		t.Parallel()

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			assert.Equal(t, "/cluster/ha/resources/999", r.URL.Path)

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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Delete(context.Background(), 999)
		require.NoError(t, err)

		assert.Equal(t, "/cluster/ha/resources/999", captured.Path)
	})

	t.Run("delete handles API error", func(t *testing.T) {
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

		proxmoxAdapter := NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		haAdapter := NewHAAdapter(proxmoxAdapter)
		err = haAdapter.Delete(context.Background(), 100)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to delete HA resource: 500 Internal Server Error", "error message should match")
	})
}

func TestNewHAAdapter(t *testing.T) {
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

		haAdapter := NewHAAdapter(proxmoxAdapter)
		require.NotNil(t, haAdapter)
		assert.NotNil(t, haAdapter.client)
	})
}
