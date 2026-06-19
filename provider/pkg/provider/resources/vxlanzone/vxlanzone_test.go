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

package vxlanzone_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	sdnvxlanzoneResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vxlanzone"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

func intPtr(value int) *int {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

type mockVxlanZoneOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.VxlanZoneInputs) error
	getFunc    func(ctx context.Context, name string) (*proxmox.VxlanZoneOutputs, error)
	updateFunc func(
		ctx context.Context,
		name string,
		inputs proxmox.VxlanZoneInputs,
		oldOutputs proxmox.VxlanZoneOutputs,
	) error
	deleteFunc func(ctx context.Context, name string) error
}

func (mock *mockVxlanZoneOperations) Create(ctx context.Context, inputs proxmox.VxlanZoneInputs) error {
	if mock.createFunc != nil {
		return mock.createFunc(ctx, inputs)
	}
	return nil
}

func (mock *mockVxlanZoneOperations) Get(ctx context.Context, name string) (*proxmox.VxlanZoneOutputs, error) {
	if mock.getFunc != nil {
		return mock.getFunc(ctx, name)
	}
	return &proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: name}}, nil
}

func (mock *mockVxlanZoneOperations) Update(
	ctx context.Context,
	name string,
	inputs proxmox.VxlanZoneInputs,
	oldOutputs proxmox.VxlanZoneOutputs,
) error {
	if mock.updateFunc != nil {
		return mock.updateFunc(ctx, name, inputs, oldOutputs)
	}
	return nil
}

func (mock *mockVxlanZoneOperations) Delete(ctx context.Context, name string) error {
	if mock.deleteFunc != nil {
		return mock.deleteFunc(ctx, name)
	}
	return nil
}

func TestVxlanZoneCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dryRun    bool
		ops       proxmox.VxlanZoneOperations
		inputs    proxmox.VxlanZoneInputs
		expectErr bool
		errMsg    string
	}{
		{
			name:   "success",
			inputs: proxmox.VxlanZoneInputs{Name: "vxlan-1", MTU: intPtr(1450), VXLANPort: intPtr(4789)},
			ops: &mockVxlanZoneOperations{
				createFunc: func(_ context.Context, inputs proxmox.VxlanZoneInputs) error {
					assert.Equal(t, "vxlan-1", inputs.Name)
					return nil
				},
			},
		},
		{
			name:   "preview skips create",
			dryRun: true,
			inputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"},
			ops: &mockVxlanZoneOperations{
				createFunc: func(_ context.Context, _ proxmox.VxlanZoneInputs) error {
					return errors.New("must not be called")
				},
			},
		},
		{
			name:      "missing ops",
			inputs:    proxmox.VxlanZoneInputs{Name: "vxlan-1"},
			expectErr: true,
			errMsg:    "VxlanZoneOperations not configured",
		},
		{
			name:   "backend failure",
			inputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"},
			ops: &mockVxlanZoneOperations{
				createFunc: func(_ context.Context, _ proxmox.VxlanZoneInputs) error {
					return errors.New("failed to create SDN VXLAN zone: 500 Internal Server Error")
				},
			},
			expectErr: true,
			errMsg:    "failed to create SDN VXLAN zone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnvxlanzoneResource.VxlanZone{VxlanZoneOps: tt.ops}
			resp, err := resource.Create(context.Background(), infer.CreateRequest[proxmox.VxlanZoneInputs]{
				Name:   tt.inputs.Name,
				Inputs: tt.inputs,
				DryRun: tt.dryRun,
			})

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.inputs.Name, resp.ID)
			assert.Equal(t, tt.inputs.Name, resp.Output.Name)
		})
	}
}

func TestVxlanZoneUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dryRun    bool
		ops       proxmox.VxlanZoneOperations
		inputs    proxmox.VxlanZoneInputs
		state     proxmox.VxlanZoneOutputs
		expectErr bool
		errMsg    string
	}{
		{
			name:   "success",
			inputs: proxmox.VxlanZoneInputs{Name: "vxlan-1", DNS: stringPtr("pdns")},
			state: proxmox.VxlanZoneOutputs{
				VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: "vxlan-1", DNS: stringPtr("old")},
			},
			ops: &mockVxlanZoneOperations{
				updateFunc: func(
					_ context.Context,
					name string,
					inputs proxmox.VxlanZoneInputs,
					_ proxmox.VxlanZoneOutputs,
				) error {
					assert.Equal(t, "vxlan-1", name)
					require.NotNil(t, inputs.DNS)
					assert.Equal(t, "pdns", *inputs.DNS)
					return nil
				},
			},
		},
		{
			name:   "preview skips update",
			dryRun: true,
			inputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"},
			state:  proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"}},
			ops: &mockVxlanZoneOperations{
				updateFunc: func(_ context.Context, _ string, _ proxmox.VxlanZoneInputs, _ proxmox.VxlanZoneOutputs) error {
					return errors.New("must not be called")
				},
			},
		},
		{
			name:      "missing ops",
			inputs:    proxmox.VxlanZoneInputs{Name: "vxlan-1"},
			state:     proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"}},
			expectErr: true,
			errMsg:    "VxlanZoneOperations not configured",
		},
		{
			name:   "backend failure",
			inputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"},
			state:  proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"}},
			ops: &mockVxlanZoneOperations{
				updateFunc: func(_ context.Context, _ string, _ proxmox.VxlanZoneInputs, _ proxmox.VxlanZoneOutputs) error {
					return errors.New("failed to update SDN VXLAN zone: 500 Internal Server Error")
				},
			},
			expectErr: true,
			errMsg:    "failed to update SDN VXLAN zone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnvxlanzoneResource.VxlanZone{VxlanZoneOps: tt.ops}
			resp, err := resource.Update(
				context.Background(),
				infer.UpdateRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]{
					ID:     tt.inputs.Name,
					Inputs: tt.inputs,
					State:  tt.state,
					DryRun: tt.dryRun,
				},
			)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.inputs.Name, resp.Output.Name)
		})
	}
}

func TestVxlanZoneDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ops       proxmox.VxlanZoneOperations
		state     proxmox.VxlanZoneOutputs
		expectErr bool
		errMsg    string
	}{
		{
			name:  "success",
			state: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"}},
			ops: &mockVxlanZoneOperations{
				deleteFunc: func(_ context.Context, name string) error {
					assert.Equal(t, "vxlan-1", name)
					return nil
				},
			},
		},
		{
			name:      "missing ops",
			state:     proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"}},
			expectErr: true,
			errMsg:    "VxlanZoneOperations not configured",
		},
		{
			name:  "backend failure",
			state: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"}},
			ops: &mockVxlanZoneOperations{
				deleteFunc: func(_ context.Context, _ string) error {
					return errors.New("failed to delete SDN VXLAN zone: 500 Internal Server Error")
				},
			},
			expectErr: true,
			errMsg:    "failed to delete SDN VXLAN zone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnvxlanzoneResource.VxlanZone{VxlanZoneOps: tt.ops}
			_, err := resource.Delete(
				context.Background(),
				infer.DeleteRequest[proxmox.VxlanZoneOutputs]{
					ID:    "vxlan-1",
					State: tt.state,
				},
			)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestVxlanZoneRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ops        proxmox.VxlanZoneOperations
		request    infer.ReadRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]
		expectErr  bool
		errMsg     string
		expectID   string
		checkPeers bool
	}{
		{
			name: "success preserves peers and prefers input name",
			ops: &mockVxlanZoneOperations{
				getFunc: func(_ context.Context, name string) (*proxmox.VxlanZoneOutputs, error) {
					assert.Equal(t, "vxlan-from-inputs", name)
					return &proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
						Name:      name,
						MTU:       intPtr(1450),
						VXLANPort: intPtr(4789),
					}}, nil
				},
			},
			request: infer.ReadRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]{
				ID:     "vxlan-from-id",
				Inputs: proxmox.VxlanZoneInputs{Name: "vxlan-from-inputs"},
				State: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
					Name:  "vxlan-from-inputs",
					Peers: []string{"10.0.0.1", "10.0.0.2"},
				}},
			},
			expectID:   "vxlan-from-id",
			checkPeers: true,
		},
		{
			name: "success falls back to ID when input name empty",
			ops: &mockVxlanZoneOperations{
				getFunc: func(_ context.Context, name string) (*proxmox.VxlanZoneOutputs, error) {
					assert.Equal(t, "vxlan-import-id", name)
					return &proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
						Name:      name,
						MTU:       intPtr(1450),
						VXLANPort: intPtr(4789),
					}}, nil
				},
			},
			request: infer.ReadRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]{
				ID:     "vxlan-import-id",
				Inputs: proxmox.VxlanZoneInputs{},
				State: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
					Peers: []string{"10.0.0.1", "10.0.0.2"},
				}},
			},
			expectID:   "vxlan-import-id",
			checkPeers: true,
		},
		{
			name: "not found clears resource",
			ops: &mockVxlanZoneOperations{
				getFunc: func(_ context.Context, _ string) (*proxmox.VxlanZoneOutputs, error) {
					return nil, errors.New("failed to get SDN VXLAN zone: 404 Not Found")
				},
			},
			request: infer.ReadRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]{
				ID:    "vxlan-1",
				State: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{Name: "vxlan-1"}},
			},
			expectID: "",
		},
		{
			name:      "missing ops",
			request:   infer.ReadRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]{ID: "vxlan-1"},
			expectErr: true,
			errMsg:    "VxlanZoneOperations not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnvxlanzoneResource.VxlanZone{VxlanZoneOps: tt.ops}
			resp, err := resource.Read(context.Background(), tt.request)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectID, resp.ID)
			if tt.checkPeers {
				assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, resp.State.Peers)
			}
		})
	}
}

func TestVxlanZoneCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		newInputs    property.Map
		expectFail   bool
		failProperty string
	}{
		{
			name: "only fabric",
			newInputs: property.NewMap(map[string]property.Value{
				"name":   property.New("vxlan1"),
				"fabric": property.New("fabric-1"),
				"ipam":   property.New("pve"),
			}),
		},
		{
			name: "only peers",
			newInputs: property.NewMap(map[string]property.Value{
				"name":  property.New("vxlan1"),
				"peers": property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":  property.New("pve"),
			}),
		},
		{
			name: "neither peers nor fabric",
			newInputs: property.NewMap(map[string]property.Value{
				"name": property.New("vxlan1"),
				"ipam": property.New("pve"),
			}),
			expectFail:   true,
			failProperty: "peers",
		},
		{
			name: "both peers and fabric",
			newInputs: property.NewMap(map[string]property.Value{
				"name":   property.New("vxlan1"),
				"fabric": property.New("fabric-1"),
				"peers":  property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":   property.New("pve"),
			}),
			expectFail:   true,
			failProperty: "peers",
		},
		{
			name: "name starts with digit",
			newInputs: property.NewMap(map[string]property.Value{
				"name":  property.New("1vxlan"),
				"peers": property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":  property.New("pve"),
			}),
			expectFail:   true,
			failProperty: "name",
		},
		{
			name: "name contains hyphen",
			newInputs: property.NewMap(map[string]property.Value{
				"name":  property.New("vxlan-1"),
				"peers": property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":  property.New("pve"),
			}),
			expectFail:   true,
			failProperty: "name",
		},
		{
			name: "name too long",
			newInputs: property.NewMap(map[string]property.Value{
				"name":  property.New("verylongname"),
				"peers": property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":  property.New("pve"),
			}),
			expectFail:   true,
			failProperty: "name",
		},
		{
			name: "name exactly 8 chars",
			newInputs: property.NewMap(map[string]property.Value{
				"name":  property.New("vxlan123"),
				"peers": property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":  property.New("pve"),
			}),
		},
		{
			name: "mtu zero",
			newInputs: property.NewMap(map[string]property.Value{
				"name":  property.New("vxlan1"),
				"peers": property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":  property.New("pve"),
				"mtu":   property.New(float64(0)),
			}),
			expectFail:   true,
			failProperty: "mtu",
		},
		{
			name: "mtu negative",
			newInputs: property.NewMap(map[string]property.Value{
				"name":  property.New("vxlan1"),
				"peers": property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":  property.New("pve"),
				"mtu":   property.New(float64(-1)),
			}),
			expectFail:   true,
			failProperty: "mtu",
		},
		{
			name: "mtu at limit 65000",
			newInputs: property.NewMap(map[string]property.Value{
				"name":  property.New("vxlan1"),
				"peers": property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":  property.New("pve"),
				"mtu":   property.New(float64(65000)),
			}),
			expectFail:   true,
			failProperty: "mtu",
		},
		{
			name: "mtu valid",
			newInputs: property.NewMap(map[string]property.Value{
				"name":  property.New("vxlan1"),
				"peers": property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":  property.New("pve"),
				"mtu":   property.New(float64(1450)),
			}),
		},
		{
			name: "vxlanPort zero",
			newInputs: property.NewMap(map[string]property.Value{
				"name":      property.New("vxlan1"),
				"peers":     property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":      property.New("pve"),
				"vxlanPort": property.New(float64(0)),
			}),
			expectFail:   true,
			failProperty: "vxlanPort",
		},
		{
			name: "vxlanPort above 65535",
			newInputs: property.NewMap(map[string]property.Value{
				"name":      property.New("vxlan1"),
				"peers":     property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":      property.New("pve"),
				"vxlanPort": property.New(float64(65536)),
			}),
			expectFail:   true,
			failProperty: "vxlanPort",
		},
		{
			name: "vxlanPort valid",
			newInputs: property.NewMap(map[string]property.Value{
				"name":      property.New("vxlan1"),
				"peers":     property.New(property.NewArray([]property.Value{property.New("10.0.0.1")})),
				"ipam":      property.New("pve"),
				"vxlanPort": property.New(float64(4789)),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnvxlanzoneResource.VxlanZone{}
			resp, err := resource.Check(context.Background(), infer.CheckRequest{
				NewInputs: tt.newInputs,
			})

			require.NoError(t, err)
			if tt.expectFail {
				require.NotEmpty(t, resp.Failures)
				assert.Equal(t, tt.failProperty, resp.Failures[0].Property)
			} else {
				assert.Empty(t, resp.Failures)
			}
		})
	}
}

func TestVxlanZoneDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputs      proxmox.VxlanZoneInputs
		state       proxmox.VxlanZoneOutputs
		wantChanges bool
		wantKeys    []string
	}{
		{
			name: "no changes",
			inputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Peers: []string{"10.0.0.1", "10.0.0.2"},
				Nodes: []string{"node1", "node2"},
			},
			state: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Peers: []string{"10.0.0.1", "10.0.0.2"},
				Nodes: []string{"node1", "node2"},
			}},
			wantChanges: false,
		},
		{
			name: "peers reordered — no change",
			inputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Peers: []string{"10.0.0.2", "10.0.0.1"},
			},
			state: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Peers: []string{"10.0.0.1", "10.0.0.2"},
			}},
			wantChanges: false,
		},
		{
			name: "nodes reordered — no change",
			inputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Nodes: []string{"node2", "node1"},
			},
			state: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Nodes: []string{"node1", "node2"},
			}},
			wantChanges: false,
		},
		{
			name: "peers changed",
			inputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Peers: []string{"10.0.0.1", "10.0.0.3"},
			},
			state: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Peers: []string{"10.0.0.1", "10.0.0.2"},
			}},
			wantChanges: true,
			wantKeys:    []string{"peers"},
		},
		{
			name: "nodes changed",
			inputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Nodes: []string{"node1", "node3"},
			},
			state: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
				Name:  "vxlan-1",
				Nodes: []string{"node1", "node2"},
			}},
			wantChanges: true,
			wantKeys:    []string{"nodes"},
		},
		{
			name: "name changed triggers replace",
			inputs: proxmox.VxlanZoneInputs{
				Name: "vxlan-2",
			},
			state: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
				Name: "vxlan-1",
			}},
			wantChanges: true,
			wantKeys:    []string{"name"},
		},
		{
			name: "mtu changed",
			inputs: proxmox.VxlanZoneInputs{
				Name: "vxlan-1",
				MTU:  intPtr(1400),
			},
			state: proxmox.VxlanZoneOutputs{VxlanZoneInputs: proxmox.VxlanZoneInputs{
				Name: "vxlan-1",
				MTU:  intPtr(1500),
			}},
			wantChanges: true,
			wantKeys:    []string{"mtu"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnvxlanzoneResource.VxlanZone{}
			resp, err := resource.Diff(
				context.Background(),
				infer.DiffRequest[proxmox.VxlanZoneInputs, proxmox.VxlanZoneOutputs]{
					ID:     tt.inputs.Name,
					Inputs: tt.inputs,
					State:  tt.state,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, tt.wantChanges, resp.HasChanges)
			for _, key := range tt.wantKeys {
				assert.Contains(t, resp.DetailedDiff, key)
			}
		})
	}
}
