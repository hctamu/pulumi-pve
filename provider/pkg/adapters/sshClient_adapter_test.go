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
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
)

func TestNewSSHAdapter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{
			name: "creates adapter with config",
			cfg: &config.Config{
				PveURL:   "https://test.proxmox.com:8006",
				PveUser:  "test@pam",
				PveToken: "test-token",
				SSHUser:  "root",
				SSHPass:  "password",
			},
		},
		{
			name: "creates adapter with nil config",
			cfg:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pxa := NewProxmoxAdapter(tt.cfg)
			adapter := NewSSHAdapter(pxa, tt.cfg)

			require.NotNil(t, adapter)
			assert.Equal(t, pxa, adapter.proxmoxAdapter)
			assert.Equal(t, tt.cfg, adapter.PVEConfig)
		})
	}
}

func TestSSHAdapterConnect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		nodesResponse  interface{}
		networkHandler func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		errContains    string
		wantTargetIP   string
	}{
		{
			name: "successful connection with matching NIC",
			nodesResponse: []map[string]interface{}{
				{"node": "pve1", "status": "online"},
			},
			networkHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]interface{}{
					"data": []map[string]interface{}{
						{"iface": "eth0", "address": "10.0.0.1"},
						{"iface": "vmbr1.606", "address": "192.168.1.100"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr:      false,
			wantTargetIP: "192.168.1.100",
		},
		{
			name: "successful connection with no matching NIC returns empty IP",
			nodesResponse: []map[string]interface{}{
				{"node": "pve1", "status": "online"},
			},
			networkHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]interface{}{
					"data": []map[string]interface{}{
						{"iface": "eth0", "address": "10.0.0.1"},
						{"iface": "vmbr0", "address": "10.0.0.2"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr:      false,
			wantTargetIP: "",
		},
		{
			name:          "fails when no nodes found",
			nodesResponse: []map[string]interface{}{},
			networkHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": []}`))
			},
			wantErr:     true,
			errContains: "no nodes found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/nodes") && r.Method == http.MethodGet:
					resp := map[string]interface{}{"data": tt.nodesResponse}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(resp)
				case strings.HasSuffix(r.URL.Path, "/network") && r.Method == http.MethodGet:
					tt.networkHandler(w, r)
				default:
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"data": null}`))
				}
			}))
			defer server.Close()

			cfg := &config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
				SSHUser:  "root",
				SSHPass:  "password",
			}

			pxa := NewProxmoxAdapter(cfg)
			err := pxa.Connect(context.Background())
			require.NoError(t, err)

			adapter := NewSSHAdapter(pxa, cfg)
			err = adapter.Connect(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantTargetIP, adapter.targetIP)
				assert.NotNil(t, adapter.sshConfig)
				assert.Equal(t, "root", adapter.sshConfig.User)
			}
		})
	}
}

func TestSSHAdapterConnectIdempotent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"node": "pve1"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		default:
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"iface": "vmbr1.606", "address": "192.168.1.100"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		PveURL:   server.URL,
		PveUser:  "test@pam",
		PveToken: "test-token",
		SSHUser:  "root",
		SSHPass:  "password",
	}

	pxa := NewProxmoxAdapter(cfg)
	err := pxa.Connect(context.Background())
	require.NoError(t, err)

	adapter := NewSSHAdapter(pxa, cfg)

	// Connect multiple times
	err1 := adapter.Connect(context.Background())
	require.NoError(t, err1)
	ip1 := adapter.targetIP

	err2 := adapter.Connect(context.Background())
	require.NoError(t, err2)
	ip2 := adapter.targetIP

	// Should be the same target IP (idempotent)
	assert.Equal(t, ip1, ip2)
}

func TestSSHAdapterConnectNodeAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"data": null}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		PveURL:   server.URL,
		PveUser:  "test@pam",
		PveToken: "test-token",
		SSHUser:  "root",
		SSHPass:  "password",
	}

	pxa := NewProxmoxAdapter(cfg)
	err := pxa.Connect(context.Background())
	require.NoError(t, err)

	adapter := NewSSHAdapter(pxa, cfg)
	err = adapter.Connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error getting nodes")
}

func TestSSHAdapterConnectNetworkAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/nodes") {
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"node": "pve1"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		// Network endpoint returns error
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"data": null}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		PveURL:   server.URL,
		PveUser:  "test@pam",
		PveToken: "test-token",
		SSHUser:  "root",
		SSHPass:  "password",
	}

	pxa := NewProxmoxAdapter(cfg)
	err := pxa.Connect(context.Background())
	require.NoError(t, err)

	adapter := NewSSHAdapter(pxa, cfg)
	err = adapter.Connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error getting networks")
}

func TestSSHAdapterConnectPanicsWithNilConfigAndNoContext(t *testing.T) {
	t.Parallel()

	adapter := NewSSHAdapter(NewProxmoxAdapter(nil), nil)

	// Attempting to connect with a plain context (no Pulumi config) should panic
	assert.Panics(t, func() {
		_ = adapter.Connect(context.Background())
	}, "Expected panic when trying to get config from non-Pulumi context")
}

func TestSSHAdapterConnectMultipleNodes(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"node": "pve1"},
					{"node": "pve2"},
					{"node": "pve3"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		default:
			// All nodes have vmbr1.606 with different IPs
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"iface": "vmbr1.606", "address": "192.168.1.100"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		PveURL:   server.URL,
		PveUser:  "test@pam",
		PveToken: "test-token",
		SSHUser:  "root",
		SSHPass:  "password",
	}

	pxa := NewProxmoxAdapter(cfg)
	err := pxa.Connect(context.Background())
	require.NoError(t, err)

	adapter := NewSSHAdapter(pxa, cfg)
	err = adapter.Connect(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "192.168.1.100", adapter.targetIP)
}
