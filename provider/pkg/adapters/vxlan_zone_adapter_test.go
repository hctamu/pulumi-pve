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
	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

func intPtr(value int) *int {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func TestVxlanZoneAdapterCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputs     proxmox.VxlanZoneInputs
		statusCode int
		errMsg     string
		expectErr  bool
	}{
		{
			name: "successful create",
			inputs: proxmox.VxlanZoneInputs{
				Name:       "vxlan-1",
				Fabric:     stringPtr("fabric-1"),
				Peers:      []string{"10.0.0.1", "10.0.0.2"},
				MTU:        intPtr(1450),
				VXLANPort:  intPtr(4789),
				Nodes:      []string{"node1", "node2"},
				DNS:        stringPtr("pdns"),
				DNSZone:    stringPtr("example.local"),
				ReverseDNS: stringPtr("pdns-rev"),
				IPAM:       stringPtr("pve"),
			},
			statusCode: http.StatusOK,
		},
		{
			name: "api error",
			inputs: proxmox.VxlanZoneInputs{
				Name: "vxlan-1",
			},
			statusCode: http.StatusInternalServerError,
			errMsg:     "failed to create SDN VXLAN zone",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, captured := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, capturedReq *testutils.MockRequest) {
					if r.Method == http.MethodPost && r.URL.Path == "/cluster/sdn/zones" {
						if tt.statusCode == http.StatusOK {
							var body proxmox.VxlanZoneAPIResource
							err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&body)
							require.NoError(t, err)
							assert.Equal(t, tt.inputs.Name, body.Zone)
							assert.Equal(t, "vxlan", body.Type)
							assert.Equal(t, tt.inputs.Fabric, body.Fabric)
							require.NotEmpty(t, body.Peers)
							assert.Equal(t, utils.CommaSeparatedList(tt.inputs.Peers), body.Peers)
							require.NotNil(t, body.MTU)
							require.NotNil(t, tt.inputs.MTU)
							assert.Equal(t, *tt.inputs.MTU, *body.MTU)
							assert.Equal(t, tt.inputs.VXLANPort, body.VXLANPort)
							require.NotEmpty(t, body.Nodes)
							assert.Equal(t, utils.CommaSeparatedList(tt.inputs.Nodes), body.Nodes)
							assert.Equal(t, tt.inputs.DNS, body.DNS)
							assert.Equal(t, tt.inputs.DNSZone, body.DNSZone)
							assert.Equal(t, tt.inputs.ReverseDNS, body.ReverseDNS)

							assert.Equal(t, tt.inputs.IPAM, body.IPAM)
						}
						w.WriteHeader(tt.statusCode)
						_, _ = w.Write([]byte(`{"data":null}`))
						return
					}

					w.WriteHeader(http.StatusNotFound)
				},
			)
			defer server.Close()

			cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "test-token"}
			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			zoneAdapter := adapters.NewVxlanZoneAdapter(proxmoxAdapter)
			err = zoneAdapter.Create(context.Background(), tt.inputs)

			assert.Equal(t, http.MethodPost, captured.Method)
			assert.Equal(t, "/cluster/sdn/zones", captured.Path)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestVxlanZoneAdapterGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		response   string
		expectErr  bool
		errMsg     string
	}{
		{
			name:       "successful get",
			statusCode: http.StatusOK,
			response: `{"data":{"zone":"vxlan-1","type":"vxlan","fabric":"fabric-1","mtu":1450,"vxlan-port":4789,` +
				`"nodes":["node1","node2"],"dns":"pdns","dnszone":"example.local",` +
				`"reversedns":"pdns-rev","ipam":"pve"}}`,
		},
		{
			name:       "api error",
			statusCode: http.StatusNotFound,
			response:   `{"data":null}`,
			expectErr:  true,
			errMsg:     "failed to get SDN VXLAN zone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, captured := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					if r.Method == http.MethodGet && r.URL.Path == "/cluster/sdn/zones/vxlan-1" {
						w.WriteHeader(tt.statusCode)
						_, _ = w.Write([]byte(tt.response))
						return
					}
					w.WriteHeader(http.StatusNotFound)
				},
			)
			defer server.Close()

			cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "test-token"}
			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			zoneAdapter := adapters.NewVxlanZoneAdapter(proxmoxAdapter)
			outputs, err := zoneAdapter.Get(context.Background(), "vxlan-1")

			assert.Equal(t, http.MethodGet, captured.Method)
			assert.Equal(t, "/cluster/sdn/zones/vxlan-1", captured.Path)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, outputs)
			assert.Equal(t, "vxlan-1", outputs.Name)
			require.NotNil(t, outputs.Fabric)
			assert.Equal(t, "fabric-1", *outputs.Fabric)
			require.NotNil(t, outputs.MTU)
			assert.Equal(t, 1450, *outputs.MTU)
			require.NotNil(t, outputs.VXLANPort)
			assert.Equal(t, 4789, *outputs.VXLANPort)
			assert.Equal(t, []string{"node1", "node2"}, outputs.Nodes)
			require.NotNil(t, outputs.DNS)
			assert.Equal(t, "pdns", *outputs.DNS)
			require.NotNil(t, outputs.DNSZone)
			assert.Equal(t, "example.local", *outputs.DNSZone)
			require.NotNil(t, outputs.ReverseDNS)
			assert.Equal(t, "pdns-rev", *outputs.ReverseDNS)
			require.NotNil(t, outputs.IPAM)
			assert.Equal(t, "pve", *outputs.IPAM)
		})
	}
}

func TestVxlanZoneAdapterUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputs     proxmox.VxlanZoneInputs
		old        proxmox.VxlanZoneOutputs
		statusCode int
		expectErr  bool
		errMsg     string
		assertBody func(t *testing.T, body proxmox.VxlanZoneAPIResource)
	}{
		{
			name: "successful update with delete fields",
			inputs: proxmox.VxlanZoneInputs{
				Name:      "vxlan-1",
				Peers:     []string{"10.0.0.3"},
				MTU:       intPtr(1450),
				VXLANPort: intPtr(4789),
				Nodes:     []string{"node1"},
				IPAM:      stringPtr("pve"),
			},
			old: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
				Name:       "vxlan-1",
				Fabric:     stringPtr("fabric-1"),
				Peers:      []string{"10.0.0.1", "10.0.0.2"},
				MTU:        intPtr(1450),
				VXLANPort:  intPtr(4789),
				Nodes:      []string{"node1", "node2"},
				DNS:        stringPtr("pdns"),
				DNSZone:    stringPtr("example.local"),
				ReverseDNS: stringPtr("pdns-rev"),
				IPAM:       stringPtr("pve"),
			}},
			statusCode: http.StatusOK,
			assertBody: func(t *testing.T, body proxmox.VxlanZoneAPIResource) {
				assert.Empty(t, body.Zone)
				require.NotEmpty(t, body.Peers)
				assert.Equal(t, utils.CommaSeparatedList{"10.0.0.3"}, body.Peers)
				require.NotEmpty(t, body.Nodes)
				assert.Equal(t, utils.CommaSeparatedList{"node1"}, body.Nodes)
				require.NotNil(t, body.IPAM)
				assert.Equal(t, "pve", *body.IPAM)
				assert.ElementsMatch(t, []string{"dns", "dnszone", "reversedns", "fabric"}, body.Delete)
			},
		},
		{
			name: "api error",
			inputs: proxmox.VxlanZoneInputs{
				Name: "vxlan-1",
			},
			old:        proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"}},
			statusCode: http.StatusInternalServerError,
			expectErr:  true,
			errMsg:     "failed to update SDN VXLAN zone",
			assertBody: func(_ *testing.T, _ proxmox.VxlanZoneAPIResource) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, captured := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, capturedReq *testutils.MockRequest) {
					if r.Method == http.MethodPut && r.URL.Path == "/cluster/sdn/zones/vxlan-1" {
						var body proxmox.VxlanZoneAPIResource
						err := json.NewDecoder(strings.NewReader(capturedReq.Body)).Decode(&body)
						require.NoError(t, err)
						tt.assertBody(t, body)
						w.WriteHeader(tt.statusCode)
						_, _ = w.Write([]byte(`{"data":null}`))
						return
					}
					w.WriteHeader(http.StatusNotFound)
				},
			)
			defer server.Close()

			cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "test-token"}
			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			zoneAdapter := adapters.NewVxlanZoneAdapter(proxmoxAdapter)
			err = zoneAdapter.Update(context.Background(), "vxlan-1", tt.inputs, tt.old)

			assert.Equal(t, http.MethodPut, captured.Method)
			assert.Equal(t, "/cluster/sdn/zones/vxlan-1", captured.Path)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestVxlanZoneAdapterDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		expectErr  bool
		errMsg     string
	}{
		{name: "successful delete", statusCode: http.StatusOK},
		{
			name:       "api error",
			statusCode: http.StatusInternalServerError,
			expectErr:  true,
			errMsg:     "failed to delete SDN VXLAN zone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, captured := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					if r.Method == http.MethodDelete && r.URL.Path == "/cluster/sdn/zones/vxlan-1" {
						w.WriteHeader(tt.statusCode)
						_, _ = w.Write([]byte(`{"data":null}`))
						return
					}
					w.WriteHeader(http.StatusNotFound)
				},
			)
			defer server.Close()

			cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "test-token"}
			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			zoneAdapter := adapters.NewVxlanZoneAdapter(proxmoxAdapter)
			err = zoneAdapter.Delete(context.Background(), "vxlan-1")

			assert.Equal(t, http.MethodDelete, captured.Method)
			assert.Equal(t, "/cluster/sdn/zones/vxlan-1", captured.Path)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
		})
	}
}
