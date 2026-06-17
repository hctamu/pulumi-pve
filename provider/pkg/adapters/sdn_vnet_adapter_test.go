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

	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

// newSdnVnetAdapter is a test helper that wires a ProxmoxAdapter pointing at the given
// mock server URL and returns a ready-to-use SdnVnetAdapter.
func newSdnVnetAdapter(t *testing.T, serverURL string) *adapters.SdnVnetAdapter {
	t.Helper()
	cfg := &config.Config{
		PveURL:   serverURL,
		PveUser:  "test@pam",
		PveToken: "test-token",
	}
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
	err := proxmoxAdapter.Connect(context.Background())
	require.NoError(t, err)
	return adapters.NewSdnVnetAdapter(proxmoxAdapter)
}

func TestSdnVnetAdapterCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputs      proxmox.SdnVnetInputs
		wantAlias   bool
		wantErrBody string
	}{
		{
			name:      "success with alias",
			inputs:    proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: "pool 1"},
			wantAlias: true,
		},
		{
			name:      "success without alias",
			inputs:    proxmox.SdnVnetInputs{Vnet: "vpool2", Zone: "ringfence", Tag: 10002},
			wantAlias: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, captured := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, capturedReq *testutils.MockRequest) {
					assert.Equal(t, http.MethodPost, r.Method)
					assert.Equal(t, "/cluster/sdn/vnets", r.URL.Path)

					var body proxmox.SdnVnetAPIObject
					err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&body)
					require.NoError(t, err)
					assert.Equal(t, tt.inputs.Vnet, body.Vnet)
					assert.Equal(t, tt.inputs.Zone, body.Zone)
					assert.Equal(t, tt.inputs.Tag, body.Tag)
					if tt.wantAlias {
						assert.Equal(t, tt.inputs.Alias, body.Alias)
					} else {
						assert.Empty(t, body.Alias)
					}

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"data": null}`))
				},
			)
			defer server.Close()

			err := newSdnVnetAdapter(t, server.URL).Create(context.Background(), tt.inputs)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, captured.Method)
			assert.Equal(t, "/cluster/sdn/vnets", captured.Path)
		})
	}

	t.Run("serializes booleans as proxmox 1/0", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.SdnVnetInputs{
			Vnet:         "vpool3",
			Zone:         "ringfence",
			Tag:          10003,
			Vlanaware:    true,
			IsolatePorts: true,
		}

		server, _ := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, _ *http.Request, capturedReq *testutils.MockRequest) {
				// api.IntOrBool marshals to 1/0, not true/false.
				assert.Contains(t, capturedReq.Body, "\"vlanaware\":1")
				assert.Contains(t, capturedReq.Body, "\"isolate-ports\":1")

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			},
		)
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Create(context.Background(), inputs)
		require.NoError(t, err)
	})

	t.Run("serializes false booleans explicitly", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.SdnVnetInputs{Vnet: "vpool4", Zone: "ringfence", Tag: 10004}

		server, _ := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, _ *http.Request, capturedReq *testutils.MockRequest) {
				// Pointers are always set, so false must be sent as 0 (not omitted).
				assert.Contains(t, capturedReq.Body, "\"vlanaware\":0")
				assert.Contains(t, capturedReq.Body, "\"isolate-ports\":0")

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			},
		)
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Create(context.Background(), inputs)
		require.NoError(t, err)
	})

	t.Run("API error", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *testutils.MockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors": "already exists"}`))
		})
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Create(
			context.Background(),
			proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
		)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to create VNet: 500 Internal Server Error")
	})
}

func TestSdnVnetAdapterGet(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		trueVal := api.IntOrBool(true)
		apiResponse := proxmox.SdnVnetAPIObject{
			Vnet:         "vpool1",
			Zone:         "ringfence",
			Tag:          10001,
			Alias:        "pool 1",
			Type:         "vnet",
			Vlanaware:    &trueVal,
			IsolatePorts: &trueVal,
			State:        "changed",
			Digest:       "abc123",
		}

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/cluster/sdn/vnets/vpool1", r.URL.Path)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(map[string]any{"data": apiResponse})
				require.NoError(t, err)
			},
		)
		defer server.Close()

		outputs, err := newSdnVnetAdapter(t, server.URL).Get(context.Background(), "vpool1")
		require.NoError(t, err)
		require.NotNil(t, outputs)

		assert.Equal(t, "vpool1", outputs.Vnet)
		assert.Equal(t, "ringfence", outputs.Zone)
		assert.Equal(t, 10001, outputs.Tag)
		assert.Equal(t, "pool 1", outputs.Alias)
		assert.True(t, outputs.Vlanaware)
		assert.True(t, outputs.IsolatePorts)
		assert.Equal(t, "changed", outputs.State)
		assert.Equal(t, "abc123", outputs.Digest)

		assert.Equal(t, http.MethodGet, captured.Method)
		assert.Equal(t, "/cluster/sdn/vnets/vpool1", captured.Path)
	})

	t.Run("success without alias", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, _ *http.Request, _ *testutils.MockRequest) {
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(map[string]any{"data": proxmox.SdnVnetAPIObject{
					Vnet: "vpool2",
					Zone: "ringfence",
					Tag:  10002,
					Type: "vnet",
				}})
				require.NoError(t, err)
			},
		)
		defer server.Close()

		outputs, err := newSdnVnetAdapter(t, server.URL).Get(context.Background(), "vpool2")
		require.NoError(t, err)
		assert.Empty(t, outputs.Alias)
		assert.Equal(t, 10002, outputs.Tag)
	})

	t.Run("API error", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *testutils.MockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		outputs, err := newSdnVnetAdapter(t, server.URL).Get(context.Background(), "vpool1")
		require.Error(t, err)
		assert.Nil(t, outputs)
		assert.EqualError(t, err, "failed to get VNet: 500 Internal Server Error")
	})
}

func TestSdnVnetAdapterUpdate(t *testing.T) {
	t.Parallel()

	t.Run("update zone and tag", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "zone2", Tag: 20001, Alias: "pool 1", Vlanaware: true}
		oldOutputs := proxmox.SdnVnetOutputs{
			SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: "pool 1"},
		}

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, capturedReq *testutils.MockRequest) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "/cluster/sdn/vnets/vpool1", r.URL.Path)

				var body proxmox.SdnVnetAPIObject
				err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&body)
				require.NoError(t, err)
				assert.Equal(t, "zone2", body.Zone)
				assert.Equal(t, 20001, body.Tag)
				assert.Equal(t, "pool 1", body.Alias)
				assert.Empty(t, body.Delete)
				assert.Contains(t, capturedReq.Body, "\"vlanaware\":1")

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			},
		)
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Update(context.Background(), "vpool1", inputs, oldOutputs)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Equal(t, "/cluster/sdn/vnets/vpool1", captured.Path)
	})

	t.Run("alias removal sends delete list", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: ""}
		oldOutputs := proxmox.SdnVnetOutputs{
			SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: "pool 1"},
		}

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, capturedReq *testutils.MockRequest) {
				var body proxmox.SdnVnetAPIObject
				err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&body)
				require.NoError(t, err)
				assert.Equal(t, []string{"alias"}, body.Delete)
				assert.Empty(t, body.Alias)

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			},
		)
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Update(context.Background(), "vpool1", inputs, oldOutputs)
		require.NoError(t, err)
		assert.Contains(t, captured.Body, "alias")
		assert.Contains(t, captured.Body, "delete")
	})

	t.Run("alias addition does not send delete list", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: "new alias"}
		oldOutputs := proxmox.SdnVnetOutputs{
			SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
		}

		server, _ := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, _ *http.Request, capturedReq *testutils.MockRequest) {
				var body proxmox.SdnVnetAPIObject
				err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&body)
				require.NoError(t, err)
				assert.Equal(t, "new alias", body.Alias)
				assert.Empty(t, body.Delete)

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			},
		)
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Update(context.Background(), "vpool1", inputs, oldOutputs)
		require.NoError(t, err)
	})

	t.Run("API error", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *testutils.MockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors": "update failed"}`))
		})
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Update(
			context.Background(),
			"vpool1",
			proxmox.SdnVnetInputs{Zone: "ringfence", Tag: 10001},
			proxmox.SdnVnetOutputs{},
		)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to update VNet: 500 Internal Server Error")
	})
}

func TestSdnVnetAdapterDelete(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, capturedReq *testutils.MockRequest) {
				assert.Equal(t, http.MethodDelete, r.Method)
				assert.Equal(t, "/cluster/sdn/vnets/vpool1", r.URL.Path)
				assert.Empty(t, capturedReq.Body)

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			},
		)
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Delete(context.Background(), "vpool1")
		require.NoError(t, err)
		assert.Equal(t, http.MethodDelete, captured.Method)
		assert.Equal(t, "/cluster/sdn/vnets/vpool1", captured.Path)
		assert.Empty(t, captured.Body)
	})

	t.Run("different vnet name in path", func(t *testing.T) {
		t.Parallel()

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
				assert.Equal(t, "/cluster/sdn/vnets/vpool42", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
			},
		)
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Delete(context.Background(), "vpool42")
		require.NoError(t, err)
		assert.Equal(t, "/cluster/sdn/vnets/vpool42", captured.Path)
	})

	t.Run("API error", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *testutils.MockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		err := newSdnVnetAdapter(t, server.URL).Delete(context.Background(), "vpool1")
		require.Error(t, err)
		assert.EqualError(t, err, "failed to delete VNet: 500 Internal Server Error")
	})
}
