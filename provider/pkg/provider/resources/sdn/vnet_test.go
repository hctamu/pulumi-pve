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

	sdnResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/sdn"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// mockSdnVnetOperations is a test double for proxmox.SdnVnetOperations.
// Set the func fields to customise behaviour per subtest; nil funcs are no-ops that succeed.
type mockSdnVnetOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.SdnVnetInputs) error
	getFunc    func(ctx context.Context, vnet string) (*proxmox.SdnVnetOutputs, error)
	updateFunc func(ctx context.Context, vnet string, inputs proxmox.SdnVnetInputs, old proxmox.SdnVnetOutputs) error
	deleteFunc func(ctx context.Context, vnet string) error
}

func (mock *mockSdnVnetOperations) Create(ctx context.Context, inputs proxmox.SdnVnetInputs) error {
	if mock.createFunc != nil {
		return mock.createFunc(ctx, inputs)
	}
	return nil
}

func (mock *mockSdnVnetOperations) Get(ctx context.Context, vnet string) (*proxmox.SdnVnetOutputs, error) {
	if mock.getFunc != nil {
		return mock.getFunc(ctx, vnet)
	}
	return &proxmox.SdnVnetOutputs{
		SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: vnet, Zone: "ringfence", Tag: 10001},
	}, nil
}

func (mock *mockSdnVnetOperations) Update(
	ctx context.Context,
	vnet string,
	inputs proxmox.SdnVnetInputs,
	old proxmox.SdnVnetOutputs,
) error {
	if mock.updateFunc != nil {
		return mock.updateFunc(ctx, vnet, inputs, old)
	}
	return nil
}

func (mock *mockSdnVnetOperations) Delete(ctx context.Context, vnet string) error {
	if mock.deleteFunc != nil {
		return mock.deleteFunc(ctx, vnet)
	}
	return nil
}

// --- Create ---

func TestSdnVnetCreate(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var gotInputs proxmox.SdnVnetInputs
		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				createFunc: func(_ context.Context, inputs proxmox.SdnVnetInputs) error {
					gotInputs = inputs
					return nil
				},
			},
		}

		req := infer.CreateRequest[proxmox.SdnVnetInputs]{
			Name:   "my-vnet",
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: "pool 1"},
		}
		resp, err := resource.Create(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "my-vnet", resp.ID)
		assert.Equal(t, "vpool1", resp.Output.Vnet)
		assert.Equal(t, "ringfence", resp.Output.Zone)
		assert.Equal(t, 10001, resp.Output.Tag)
		assert.Equal(t, "pool 1", resp.Output.Alias)
		assert.Equal(t, req.Inputs, gotInputs)
	})

	t.Run("dry run does not call ops", func(t *testing.T) {
		t.Parallel()

		called := false
		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				createFunc: func(_ context.Context, _ proxmox.SdnVnetInputs) error {
					called = true
					return nil
				},
			},
		}

		req := infer.CreateRequest[proxmox.SdnVnetInputs]{
			Name:   "my-vnet",
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			DryRun: true,
		}
		resp, err := resource.Create(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "my-vnet", resp.ID)
		assert.False(t, called, "createFunc must not be called on dry run")
	})

	t.Run("invalid name longer than 8 chars", func(t *testing.T) {
		t.Parallel()

		called := false
		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				createFunc: func(_ context.Context, _ proxmox.SdnVnetInputs) error {
					called = true
					return nil
				},
			},
		}

		req := infer.CreateRequest[proxmox.SdnVnetInputs]{
			Inputs: proxmox.SdnVnetInputs{Vnet: "vnetpool42", Zone: "ringfence", Tag: 10042},
		}
		_, err := resource.Create(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be 1-8 characters")
		assert.False(t, called, "createFunc must not be called when name is invalid")
	})

	t.Run("invalid name starts with digit", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{SdnVnetOps: &mockSdnVnetOperations{}}
		req := infer.CreateRequest[proxmox.SdnVnetInputs]{
			Inputs: proxmox.SdnVnetInputs{Vnet: "1vpool", Zone: "ringfence", Tag: 10001},
		}
		_, err := resource.Create(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be alphanumeric and start with a letter")
	})

	t.Run("invalid name also rejected on dry run", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{SdnVnetOps: &mockSdnVnetOperations{}}
		req := infer.CreateRequest[proxmox.SdnVnetInputs]{
			Inputs: proxmox.SdnVnetInputs{Vnet: "vnet-pool42", Zone: "ringfence", Tag: 10042},
			DryRun: true,
		}
		_, err := resource.Create(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid vnet name")
	})

	t.Run("nil ops returns error", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{}
		req := infer.CreateRequest[proxmox.SdnVnetInputs]{
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
		}
		_, err := resource.Create(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, "SdnVnetOperations not configured", err.Error())
	})

	t.Run("adapter error is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				createFunc: func(_ context.Context, _ proxmox.SdnVnetInputs) error {
					return errors.New("failed to create VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.CreateRequest[proxmox.SdnVnetInputs]{
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
		}
		_, err := resource.Create(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}

// --- Read ---

func TestSdnVnetRead(t *testing.T) {
	t.Parallel()

	t.Run("success maps outputs to inputs and state", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				getFunc: func(_ context.Context, vnet string) (*proxmox.SdnVnetOutputs, error) {
					assert.Equal(t, "vpool1", vnet)
					return &proxmox.SdnVnetOutputs{
						SdnVnetInputs: proxmox.SdnVnetInputs{
							Vnet: "vpool1", Zone: "ringfence", Tag: 10001, Alias: "pool 1",
						},
					}, nil
				},
			},
		}

		req := infer.ReadRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs]{
			ID:     "my-vnet",
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1"},
		}
		resp, err := resource.Read(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "vpool1", resp.State.Vnet)
		assert.Equal(t, "ringfence", resp.State.Zone)
		assert.Equal(t, 10001, resp.State.Tag)
		assert.Equal(t, "pool 1", resp.State.Alias)
		assert.Equal(t, resp.State.SdnVnetInputs, resp.Inputs)
	})

	t.Run("import fallback uses resource ID when Inputs.Vnet is empty", func(t *testing.T) {
		t.Parallel()

		var gotVnet string
		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				getFunc: func(_ context.Context, vnet string) (*proxmox.SdnVnetOutputs, error) {
					gotVnet = vnet
					return &proxmox.SdnVnetOutputs{
						SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: vnet, Zone: "ringfence", Tag: 10001},
					}, nil
				},
			},
		}

		req := infer.ReadRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs]{
			ID:     "vpool1",
			Inputs: proxmox.SdnVnetInputs{}, // empty — simulates import
		}
		_, err := resource.Read(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "vpool1", gotVnet)
	})

	t.Run("nil ops returns error", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{}
		req := infer.ReadRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs]{
			ID:     "my-vnet",
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1"},
		}
		_, err := resource.Read(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, "SdnVnetOperations not configured", err.Error())
	})

	t.Run("adapter error is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				getFunc: func(_ context.Context, _ string) (*proxmox.SdnVnetOutputs, error) {
					return nil, errors.New("failed to get VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.ReadRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs]{
			ID:     "my-vnet",
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1"},
		}
		_, err := resource.Read(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}

// --- Update ---

func TestSdnVnetUpdate(t *testing.T) {
	t.Parallel()

	t.Run("success propagates new inputs into output", func(t *testing.T) {
		t.Parallel()

		var gotVnet string
		var gotInputs proxmox.SdnVnetInputs
		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				updateFunc: func(_ context.Context, vnet string, inputs proxmox.SdnVnetInputs, _ proxmox.SdnVnetOutputs) error {
					gotVnet = vnet
					gotInputs = inputs
					return nil
				},
			},
		}

		newInputs := proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 20001}
		req := infer.UpdateRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs]{
			ID:     "my-vnet",
			Inputs: newInputs,
			State: proxmox.SdnVnetOutputs{
				SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			},
		}
		resp, err := resource.Update(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, newInputs, resp.Output.SdnVnetInputs)
		assert.Equal(t, "vpool1", gotVnet)
		assert.Equal(t, newInputs, gotInputs)
	})

	t.Run("dry run does not call ops", func(t *testing.T) {
		t.Parallel()

		called := false
		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				updateFunc: func(_ context.Context, _ string, _ proxmox.SdnVnetInputs, _ proxmox.SdnVnetOutputs) error {
					called = true
					return nil
				},
			},
		}

		req := infer.UpdateRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs]{
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			State:  proxmox.SdnVnetOutputs{},
			DryRun: true,
		}
		_, err := resource.Update(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, called, "updateFunc must not be called on dry run")
	})

	t.Run("nil ops returns error", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{}
		req := infer.UpdateRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs]{
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
		}
		_, err := resource.Update(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, "SdnVnetOperations not configured", err.Error())
	})

	t.Run("adapter error is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				updateFunc: func(_ context.Context, _ string, _ proxmox.SdnVnetInputs, _ proxmox.SdnVnetOutputs) error {
					return errors.New("failed to update VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.UpdateRequest[proxmox.SdnVnetInputs, proxmox.SdnVnetOutputs]{
			Inputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			State:  proxmox.SdnVnetOutputs{SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: "vpool1"}},
		}
		_, err := resource.Update(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}

// --- Delete ---

func TestSdnVnetDelete(t *testing.T) {
	t.Parallel()

	t.Run("success uses State.Vnet as identifier", func(t *testing.T) {
		t.Parallel()

		var gotVnet string
		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				deleteFunc: func(_ context.Context, vnet string) error {
					gotVnet = vnet
					return nil
				},
			},
		}

		req := infer.DeleteRequest[proxmox.SdnVnetOutputs]{
			State: proxmox.SdnVnetOutputs{
				SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: "vpool1", Zone: "ringfence", Tag: 10001},
			},
		}
		_, err := resource.Delete(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "vpool1", gotVnet)
	})

	t.Run("nil ops returns error", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{}
		req := infer.DeleteRequest[proxmox.SdnVnetOutputs]{
			State: proxmox.SdnVnetOutputs{SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: "vpool1"}},
		}
		_, err := resource.Delete(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, "SdnVnetOperations not configured", err.Error())
	})

	t.Run("adapter error is propagated", func(t *testing.T) {
		t.Parallel()

		resource := &sdnResource.SdnVnet{
			SdnVnetOps: &mockSdnVnetOperations{
				deleteFunc: func(_ context.Context, _ string) error {
					return errors.New("failed to delete VNet: 500 Internal Server Error")
				},
			},
		}
		req := infer.DeleteRequest[proxmox.SdnVnetOutputs]{
			State: proxmox.SdnVnetOutputs{SdnVnetInputs: proxmox.SdnVnetInputs{Vnet: "vpool1"}},
		}
		_, err := resource.Delete(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
	})
}
