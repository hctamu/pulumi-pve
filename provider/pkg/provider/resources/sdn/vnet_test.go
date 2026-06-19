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

package sdn_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	sdnResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/sdn"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// mockVnetOperations is a test double for proxmox.VnetOperations.
// Set the func fields to customise behaviour per subtest; nil funcs are no-ops that succeed.
type mockVnetOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.VnetInputs) error
	getFunc    func(ctx context.Context, vnet string) (*proxmox.VnetOutputs, error)
	updateFunc func(ctx context.Context, vnet string, inputs proxmox.VnetInputs, old proxmox.VnetOutputs) error
	deleteFunc func(ctx context.Context, vnet string) error
}

func (mock *mockVnetOperations) Create(ctx context.Context, inputs proxmox.VnetInputs) error {
	if mock.createFunc != nil {
		return mock.createFunc(ctx, inputs)
	}
	return nil
}

func (mock *mockVnetOperations) Get(ctx context.Context, vnet string) (*proxmox.VnetOutputs, error) {
	if mock.getFunc != nil {
		return mock.getFunc(ctx, vnet)
	}
	return &proxmox.VnetOutputs{
		VnetInputs: proxmox.VnetInputs{Vnet: vnet, Zone: "ringfence", Tag: 10001},
	}, nil
}

func (mock *mockVnetOperations) Update(
	ctx context.Context,
	vnet string,
	inputs proxmox.VnetInputs,
	old proxmox.VnetOutputs,
) error {
	if mock.updateFunc != nil {
		return mock.updateFunc(ctx, vnet, inputs, old)
	}
	return nil
}

func (mock *mockVnetOperations) Delete(ctx context.Context, vnet string) error {
	if mock.deleteFunc != nil {
		return mock.deleteFunc(ctx, vnet)
	}
	return nil
}

// --- Check ---

func TestVnetCheck(t *testing.T) {
	t.Parallel()

	newInputs := func(vnet, zone string, tag float64) property.Map {
		return property.NewMap(map[string]property.Value{
			"vnet": property.New(vnet),
			"zone": property.New(zone),
			"tag":  property.New(tag),
		})
	}

	tests := []struct {
		name            string
		vnet            string
		wantFailures    int
		wantFailureText string
	}{
		{name: "valid name passes", vnet: "vpool1"},
		{
			name:            "name longer than 8 chars",
			vnet:            "vnetpool42",
			wantFailures:    1,
			wantFailureText: "must start with a letter, contain only letters and numbers, and be at most 8 characters",
		},
		{
			name:            "empty name",
			vnet:            "",
			wantFailures:    1,
			wantFailureText: "must start with a letter, contain only letters and numbers, and be at most 8 characters",
		},
		{
			name:            "name starts with digit",
			vnet:            "1vpool",
			wantFailures:    1,
			wantFailureText: "must start with a letter, contain only letters and numbers, and be at most 8 characters",
		},
		{
			name:            "special chars in name",
			vnet:            "vnet-1",
			wantFailures:    1,
			wantFailureText: "must start with a letter, contain only letters and numbers, and be at most 8 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource := &sdnResource.Vnet{}
			req := infer.CheckRequest{NewInputs: newInputs(tt.vnet, "ringfence", 10001)}

			resp, err := resource.Check(context.Background(), req)
			require.NoError(t, err)
			assert.Len(t, resp.Failures, tt.wantFailures)
			if tt.wantFailures > 0 {
				assert.Equal(t, "vnet", resp.Failures[0].Property)
				assert.Contains(t, resp.Failures[0].Reason, tt.wantFailureText)
			}
		})
	}
}

// --- Create ---

func TestVnetCreate(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var gotInputs proxmox.VnetInputs
		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				createFunc: func(_ context.Context, inputs proxmox.VnetInputs) error {
					gotInputs = inputs
					return nil
				},
				getFunc: func(_ context.Context, _ string) (*proxmox.VnetOutputs, error) {
					return &proxmox.VnetOutputs{
						VnetInputs: proxmox.VnetInputs{
							Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: "pool 1",
						},
						State:  "new",
						Digest: "abc123",
					}, nil
				},
			},
		}

		req := infer.CreateRequest[proxmox.VnetInputs]{
			Name:   "my-vnet",
			Inputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: "pool 1"},
		}
		resp, err := resource.Create(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "my-vnet", resp.ID)
		assert.Equal(t, "vpool1", resp.Output.Vnet)
		assert.Equal(t, "ringfence", resp.Output.Zone)
		assert.Equal(t, 10001, resp.Output.Tag)
		assert.Equal(t, "pool 1", resp.Output.Alias)
		assert.Equal(t, "new", resp.Output.State)
		assert.Equal(t, "abc123", resp.Output.Digest)
		assert.Equal(t, req.Inputs, gotInputs)
	})

	t.Run("dry run does not call ops", func(t *testing.T) {
		t.Parallel()

		called := false
		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				createFunc: func(_ context.Context, _ proxmox.VnetInputs) error {
					called = true
					return nil
				},
			},
		}

		req := infer.CreateRequest[proxmox.VnetInputs]{
			Name:   "my-vnet",
			Inputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			DryRun: true,
		}
		resp, err := resource.Create(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "my-vnet", resp.ID)
		assert.False(t, called, "createFunc must not be called on dry run")
	})

	t.Run("nil ops returns error", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{}
		req := infer.CreateRequest[proxmox.VnetInputs]{
			Inputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
		}
		_, err := resource.Create(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, "VnetOperations not configured", err.Error())
	})

	t.Run("adapter error is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				createFunc: func(_ context.Context, _ proxmox.VnetInputs) error {
					return errors.New("failed to create VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.CreateRequest[proxmox.VnetInputs]{
			Inputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
		}
		_, err := resource.Create(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})

	t.Run("get error after create is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				getFunc: func(_ context.Context, _ string) (*proxmox.VnetOutputs, error) {
					return nil, errors.New("failed to get VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.CreateRequest[proxmox.VnetInputs]{
			Inputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
		}
		_, err := resource.Create(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}

// --- Read ---

func TestVnetRead(t *testing.T) {
	t.Parallel()

	t.Run("success maps outputs to inputs and state", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				getFunc: func(_ context.Context, vnet string) (*proxmox.VnetOutputs, error) {
					assert.Equal(t, "vpool1", vnet)
					return &proxmox.VnetOutputs{
						VnetInputs: proxmox.VnetInputs{
							Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: "pool 1",
							Vlanaware: true, IsolatePorts: true,
						},
						State:  "changed",
						Digest: "abc123",
					}, nil
				},
			},
		}

		req := infer.ReadRequest[proxmox.VnetInputs, proxmox.VnetOutputs]{
			ID:     "my-vnet",
			Inputs: proxmox.VnetInputs{Vnet: "vpool1"},
		}
		resp, err := resource.Read(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "vpool1", resp.State.Vnet)
		assert.Equal(t, "ringfence", resp.State.Zone)
		assert.Equal(t, 10001, resp.State.Tag)
		assert.Equal(t, "pool 1", resp.State.Alias)
		assert.True(t, resp.State.Vlanaware)
		assert.True(t, resp.State.IsolatePorts)
		assert.Equal(t, "changed", resp.State.State)
		assert.Equal(t, "abc123", resp.State.Digest)
		assert.Equal(t, resp.State.VnetInputs, resp.Inputs)
	})

	t.Run("import fallback uses resource ID when Inputs.Vnet is empty", func(t *testing.T) {
		t.Parallel()

		var gotVnet string
		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				getFunc: func(_ context.Context, vnet string) (*proxmox.VnetOutputs, error) {
					gotVnet = vnet
					return &proxmox.VnetOutputs{
						VnetInputs: proxmox.VnetInputs{Vnet: vnet, Zone: "ringfence", Tag: 10001},
					}, nil
				},
			},
		}

		req := infer.ReadRequest[proxmox.VnetInputs, proxmox.VnetOutputs]{
			ID:     "vpool1",
			Inputs: proxmox.VnetInputs{}, // empty — simulates import
		}
		_, err := resource.Read(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "vpool1", gotVnet)
	})

	t.Run("nil ops returns error", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{}
		req := infer.ReadRequest[proxmox.VnetInputs, proxmox.VnetOutputs]{
			ID:     "my-vnet",
			Inputs: proxmox.VnetInputs{Vnet: "vpool1"},
		}
		_, err := resource.Read(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, "VnetOperations not configured", err.Error())
	})

	t.Run("adapter error is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				getFunc: func(_ context.Context, _ string) (*proxmox.VnetOutputs, error) {
					return nil, errors.New("failed to get VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.ReadRequest[proxmox.VnetInputs, proxmox.VnetOutputs]{
			ID:     "my-vnet",
			Inputs: proxmox.VnetInputs{Vnet: "vpool1"},
		}
		_, err := resource.Read(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}

// --- Update ---

func TestVnetUpdate(t *testing.T) {
	t.Parallel()

	t.Run("success propagates new inputs into output", func(t *testing.T) {
		t.Parallel()

		var gotVnet string
		var gotInputs proxmox.VnetInputs
		newInputs := proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 20001}
		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				updateFunc: func(_ context.Context, vnet string, inputs proxmox.VnetInputs, _ proxmox.VnetOutputs) error {
					gotVnet = vnet
					gotInputs = inputs
					return nil
				},
				getFunc: func(_ context.Context, _ string) (*proxmox.VnetOutputs, error) {
					return &proxmox.VnetOutputs{
						VnetInputs: newInputs,
						State:      "changed",
						Digest:     "def456",
					}, nil
				},
			},
		}

		req := infer.UpdateRequest[proxmox.VnetInputs, proxmox.VnetOutputs]{
			ID:     "my-vnet",
			Inputs: newInputs,
			State: proxmox.VnetOutputs{
				VnetInputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			},
		}
		resp, err := resource.Update(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, newInputs, resp.Output.VnetInputs)
		assert.Equal(t, "changed", resp.Output.State)
		assert.Equal(t, "def456", resp.Output.Digest)
		assert.Equal(t, "vpool1", gotVnet)
		assert.Equal(t, newInputs, gotInputs)
	})

	t.Run("dry run does not call ops", func(t *testing.T) {
		t.Parallel()

		called := false
		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				updateFunc: func(_ context.Context, _ string, _ proxmox.VnetInputs, _ proxmox.VnetOutputs) error {
					called = true
					return nil
				},
			},
		}

		req := infer.UpdateRequest[proxmox.VnetInputs, proxmox.VnetOutputs]{
			Inputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			State:  proxmox.VnetOutputs{},
			DryRun: true,
		}
		_, err := resource.Update(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, called, "updateFunc must not be called on dry run")
	})

	t.Run("nil ops returns error", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{}
		req := infer.UpdateRequest[proxmox.VnetInputs, proxmox.VnetOutputs]{
			Inputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
		}
		_, err := resource.Update(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, "VnetOperations not configured", err.Error())
	})

	t.Run("adapter error is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				updateFunc: func(_ context.Context, _ string, _ proxmox.VnetInputs, _ proxmox.VnetOutputs) error {
					return errors.New("failed to update VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.UpdateRequest[proxmox.VnetInputs, proxmox.VnetOutputs]{
			Inputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			State:  proxmox.VnetOutputs{VnetInputs: proxmox.VnetInputs{Vnet: "vpool1"}},
		}
		_, err := resource.Update(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})

	t.Run("get error after update is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				getFunc: func(_ context.Context, _ string) (*proxmox.VnetOutputs, error) {
					return nil, errors.New("failed to get VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.UpdateRequest[proxmox.VnetInputs, proxmox.VnetOutputs]{
			Inputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			State:  proxmox.VnetOutputs{VnetInputs: proxmox.VnetInputs{Vnet: "vpool1"}},
		}
		_, err := resource.Update(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}

// --- Delete ---

func TestVnetDelete(t *testing.T) {
	t.Parallel()

	t.Run("success uses State.Vnet as identifier", func(t *testing.T) {
		t.Parallel()

		var gotVnet string
		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				deleteFunc: func(_ context.Context, vnet string) error {
					gotVnet = vnet
					return nil
				},
			},
		}

		req := infer.DeleteRequest[proxmox.VnetOutputs]{
			State: proxmox.VnetOutputs{
				VnetInputs: proxmox.VnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			},
		}
		_, err := resource.Delete(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "vpool1", gotVnet)
	})

	t.Run("nil ops returns error", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{}
		req := infer.DeleteRequest[proxmox.VnetOutputs]{
			State: proxmox.VnetOutputs{VnetInputs: proxmox.VnetInputs{Vnet: "vpool1"}},
		}
		_, err := resource.Delete(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, "VnetOperations not configured", err.Error())
	})

	t.Run("adapter error is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.Vnet{
			VnetOps: &mockVnetOperations{
				deleteFunc: func(_ context.Context, _ string) error {
					return errors.New("failed to delete VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.DeleteRequest[proxmox.VnetOutputs]{
			State: proxmox.VnetOutputs{VnetInputs: proxmox.VnetInputs{Vnet: "vpool1"}},
		}
		_, err := resource.Delete(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}
