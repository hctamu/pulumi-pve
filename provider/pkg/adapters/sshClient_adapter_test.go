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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
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
				PveURL:                "https://test.proxmox.com:8006",
				PveUser:               "test@pam",
				PveToken:              "test-token",
				SSHUser:               "root",
				SSHPass:               "password",
				InsecureIgnoreHostKey: true,
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

			proxmoxAdapter := NewProxmoxAdapter(tt.cfg)
			adapter := NewSSHAdapter(proxmoxAdapter, tt.cfg)

			require.NotNil(t, adapter)
			assert.Equal(t, proxmoxAdapter, adapter.proxmoxAdapter)
			assert.Equal(t, tt.cfg, adapter.PVEConfig)
		})
	}
}

func TestSSHAdapterConnect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sshInterface   string
		nodes          []nodeStatus
		networksByNode map[string][]networkInterface
		wantErr        string
	}{
		{
			name:         "fails when configured interface is not found",
			sshInterface: "vmbr9.999",
			nodes:        []nodeStatus{{Node: "pve1"}},
			networksByNode: map[string][]networkInterface{
				"pve1": {
					{Iface: "eth0", Address: "10.0.0.1"},
				},
			},
			wantErr: "configured SSH interface \"vmbr9.999\" not found",
		},
		{
			name:         "fails when configured interface has no ipv4",
			sshInterface: "vmbr1.606",
			nodes:        []nodeStatus{{Node: "pve1"}},
			networksByNode: map[string][]networkInterface{
				"pve1": {
					{Iface: "vmbr1.606", Address: "fe80::1"},
				},
			},
			wantErr: "configured SSH interface \"vmbr1.606\" has no IPv4 address",
		},
		{
			name:         "fails when no interface has ipv4 and interface is not configured",
			sshInterface: "",
			nodes:        []nodeStatus{{Node: "pve1"}},
			networksByNode: map[string][]networkInterface{
				"pve1": {
					{Iface: "vmbr1.606", Address: "fe80::1"},
				},
			},
			wantErr: "no network interface with IPv4 address found for SSH",
		},
		{
			name:         "fails when no nodes found",
			sshInterface: "",
			nodes:        []nodeStatus{},
			wantErr:      "no nodes found",
		},
		{
			name:         "fails when candidate interface is not reachable on ssh port",
			sshInterface: "vmbr1.606",
			nodes:        []nodeStatus{{Node: "pve1"}},
			networksByNode: map[string][]networkInterface{
				"pve1": {
					{Iface: "vmbr1.606", Address: "127.0.0.1"},
				},
			},
			wantErr: "no reachable SSH interface found after 1 attempts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					switch {
					case r.Method == http.MethodGet && r.URL.Path == "/nodes":
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(map[string]any{"data": tt.nodes})
					case r.Method == http.MethodGet &&
						strings.HasPrefix(r.URL.Path, "/nodes/") &&
						strings.HasSuffix(r.URL.Path, "/network"):
						nodeName := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/nodes/"), "/network")
						networks := tt.networksByNode[nodeName]
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(map[string]any{"data": networks})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				},
			)
			defer server.Close()

			cfg := &config.Config{
				PveURL:                server.URL,
				PveUser:               "test@pam",
				PveToken:              "test-token",
				SSHUser:               "root",
				SSHPass:               "password",
				SSHInterface:          tt.sshInterface,
				InsecureIgnoreHostKey: true,
			}

			adapter := NewSSHAdapter(NewProxmoxAdapter(cfg), cfg)
			err := adapter.Connect(context.Background())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestSSHAdapterConnectIdempotent(t *testing.T) {
	t.Parallel()

	var nodesCalls int32
	var networksCalls int32

	server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
		switch r.URL.Path {
		case "/nodes":
			atomic.AddInt32(&nodesCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []nodeStatus{{Node: "pve1"}}})
		case "/nodes/pve1/network":
			atomic.AddInt32(&networksCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).
				Encode(map[string]any{"data": []networkInterface{{Iface: "vmbr1.606", Address: "127.0.0.1"}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	cfg := &config.Config{
		PveURL:                server.URL,
		PveUser:               "test@pam",
		PveToken:              "test-token",
		SSHUser:               "root",
		SSHPass:               "password",
		SSHInterface:          "vmbr1.606",
		InsecureIgnoreHostKey: true,
	}

	adapter := NewSSHAdapter(NewProxmoxAdapter(cfg), cfg)
	err1 := adapter.Connect(context.Background())
	err2 := adapter.Connect(context.Background())

	require.Error(t, err1)
	require.Error(t, err2)
	assert.Equal(t, err1.Error(), err2.Error())
	assert.Equal(t, int32(1), atomic.LoadInt32(&nodesCalls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&networksCalls))
}

func TestSSHAdapterConnectNodeAPIError(t *testing.T) {
	t.Parallel()

	server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *testutils.MockRequest) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	cfg := &config.Config{
		PveURL:                server.URL,
		PveUser:               "test@pam",
		PveToken:              "test-token",
		SSHUser:               "root",
		SSHPass:               "password",
		InsecureIgnoreHostKey: true,
	}

	adapter := NewSSHAdapter(NewProxmoxAdapter(cfg), cfg)
	err := adapter.Connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error getting nodes")
}

func TestSSHAdapterConnectNetworkAPIError(t *testing.T) {
	t.Parallel()

	server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
		if r.URL.Path == "/nodes" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []nodeStatus{{Node: "pve1"}}})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	cfg := &config.Config{
		PveURL:                server.URL,
		PveUser:               "test@pam",
		PveToken:              "test-token",
		SSHUser:               "root",
		SSHPass:               "password",
		InsecureIgnoreHostKey: true,
	}

	adapter := NewSSHAdapter(NewProxmoxAdapter(cfg), cfg)
	err := adapter.Connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error getting networks for")
}

func TestSSHAdapterConnectPanicsWithNilConfigAndNoContext(t *testing.T) {
	t.Parallel()

	adapter := NewSSHAdapter(NewProxmoxAdapter(nil), nil)

	assert.Panics(t, func() {
		_ = adapter.Connect(context.Background())
	})
}

func TestSSHAdapterConnectMultipleNodes(t *testing.T) {
	t.Parallel()

	server, _ := testutils.CreateMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
		switch r.URL.Path {
		case "/nodes":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).
				Encode(map[string]any{"data": []nodeStatus{{Node: "pve1"}, {Node: "pve2"}, {Node: "pve3"}}})
		case "/nodes/pve1/network", "/nodes/pve2/network", "/nodes/pve3/network":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).
				Encode(map[string]any{"data": []networkInterface{{Iface: "vmbr1.606", Address: "127.0.0.1"}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	cfg := &config.Config{
		PveURL:                server.URL,
		PveUser:               "test@pam",
		PveToken:              "test-token",
		SSHUser:               "root",
		SSHPass:               "password",
		SSHInterface:          "vmbr1.606",
		InsecureIgnoreHostKey: true,
	}

	adapter := NewSSHAdapter(NewProxmoxAdapter(cfg), cfg)
	err := adapter.Connect(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no reachable SSH interface found after 1 attempts")
}

func TestNewHostKeyCallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		insecureIgnoreHostKey bool
		withKnownHosts        bool
		wantErr               bool
		errContains           string
		wantCallbackErr       bool
		knownHostsPathConfig  string
	}{
		{
			name:                  "returns insecure callback when option is enabled",
			insecureIgnoreHostKey: true,
			withKnownHosts:        false,
			wantErr:               false,
			wantCallbackErr:       false,
		},
		{
			name:                  "returns known_hosts callback when option is disabled and file exists",
			insecureIgnoreHostKey: false,
			withKnownHosts:        true,
			wantErr:               false,
			wantCallbackErr:       true,
		},
		{
			name:                  "uses configured known_hosts path when provided",
			insecureIgnoreHostKey: false,
			withKnownHosts:        true,
			wantErr:               false,
			wantCallbackErr:       true,
			knownHostsPathConfig:  "custom/known_hosts",
		},
		{
			name:                  "returns error when option is disabled and known_hosts is missing",
			insecureIgnoreHostKey: false,
			withKnownHosts:        false,
			wantErr:               true,
			errContains:           "known_hosts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			knownHostsPathConfig := tt.knownHostsPathConfig
			if knownHostsPathConfig == "" {
				knownHostsPathConfig = filepath.Join(t.TempDir(), "known_hosts")
			} else {
				knownHostsPathConfig = filepath.Join(t.TempDir(), knownHostsPathConfig)
			}

			if tt.withKnownHosts {
				require.NoError(t, os.MkdirAll(filepath.Dir(knownHostsPathConfig), 0o700))
				require.NoError(t, os.WriteFile(knownHostsPathConfig, []byte(""), 0o600))
			}

			callback, err := newHostKeyCallback(tt.insecureIgnoreHostKey, knownHostsPathConfig)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, callback)

			pubKey, _, _, _, parseErr := ssh.ParseAuthorizedKey([]byte(
				"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEd9fZxw2+P5lWQmWw7JXfR8zY8VzHi4vT8z3WQm9h8D test-key",
			))
			require.NoError(t, parseErr)

			cbErr := callback("example.local:22", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, pubKey)
			if tt.wantCallbackErr {
				require.Error(t, cbErr)
				return
			}
			require.NoError(t, cbErr)
		})
	}
}

func TestSelectSSHInterfaceIPv4(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		networks            []networkInterface
		configuredInterface string
		expectedCandidates  []string
		expectError         string
	}{
		{
			name: "single ipv4 from configured interface",
			networks: []networkInterface{
				{Iface: "vlan100", Address: "192.168.1.10"},
				{Iface: "vlan200", Address: "10.0.0.10"},
			},
			configuredInterface: "vlan100",
			expectedCandidates:  []string{"192.168.1.10"},
		},
		{
			name: "multiple ipv4 candidates in discovery mode",
			networks: []networkInterface{
				{Iface: "vlan100", Address: "192.168.1.10"},
				{Iface: "vlan200", Address: "10.0.0.10"},
				{Iface: "lo", Address: "127.0.0.1"},
			},
			configuredInterface: "",
			expectedCandidates:  []string{"192.168.1.10", "10.0.0.10", "127.0.0.1"},
		},
		{
			name: "cidr notation parsed correctly",
			networks: []networkInterface{
				{Iface: "vlan100", Address: "192.168.1.10/24"},
			},
			configuredInterface: "vlan100",
			expectedCandidates:  []string{"192.168.1.10"},
		},
		{
			name: "ipv6 skipped in discovery",
			networks: []networkInterface{
				{Iface: "vlan100", Address: "fe80::1"},
				{Iface: "vlan200", Address: "192.168.1.10"},
			},
			configuredInterface: "",
			expectedCandidates:  []string{"192.168.1.10"},
		},
		{
			name: "configured interface not found",
			networks: []networkInterface{
				{Iface: "vlan100", Address: "192.168.1.10"},
			},
			configuredInterface: "nonexistent",
			expectError:         "configured SSH interface \"nonexistent\" not found",
		},
		{
			name: "configured interface has ipv6 only",
			networks: []networkInterface{
				{Iface: "vlan100", Address: "fe80::1"},
			},
			configuredInterface: "vlan100",
			expectError:         "has no IPv4 address",
		},
		{
			name:                "no ipv4 in discovery mode",
			networks:            []networkInterface{{Iface: "vlan100", Address: "fe80::1"}},
			configuredInterface: "",
			expectError:         "no network interface with IPv4 address found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			candidates, err := selectSSHInterfaceIPv4(tt.networks, tt.configuredInterface)

			if tt.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedCandidates, candidates)
		})
	}
}

func TestFirstReachableSSHIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		candidates []string
		errText    string
	}{
		{
			name:       "empty candidates",
			candidates: []string{},
			errText:    "no reachable SSH interface found after 0 attempts",
		},
		{
			name:       "unreachable localhost ssh",
			candidates: []string{"127.0.0.1"},
			errText:    "no reachable SSH interface found after 1 attempts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reachableIP, err := firstReachableSSHIP(tt.candidates)
			require.Error(t, err)
			require.Empty(t, reachableIP)
			require.Contains(t, err.Error(), tt.errText)
		})
	}
}

func TestParseIPv4Address(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		address       string
		expectedIP    string
		expectedValid bool
	}{
		{name: "plain ipv4", address: "192.168.1.10", expectedIP: "192.168.1.10", expectedValid: true},
		{name: "cidr notation", address: "192.168.1.10/24", expectedIP: "192.168.1.10", expectedValid: true},
		{name: "ipv6 plain", address: "fe80::1", expectedIP: "", expectedValid: false},
		{name: "ipv6 cidr", address: "fe80::1/64", expectedIP: "", expectedValid: false},
		{name: "empty string", address: "", expectedIP: "", expectedValid: false},
		{name: "whitespace only", address: "   ", expectedIP: "", expectedValid: false},
		{name: "invalid cidr", address: "192.168.1.10/", expectedIP: "", expectedValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ip, valid := parseIPv4Address(tt.address)
			require.Equal(t, tt.expectedValid, valid)
			if valid {
				require.Equal(t, tt.expectedIP, ip)
			}
		})
	}
}
