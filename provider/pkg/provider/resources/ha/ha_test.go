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

// Package ha_test contains unit tests for HA resource operations.
//
// Test Organization:
// - Unit tests: Direct method calls with mocked HAOperations interface
// - Error scenarios: Operation failures, validation errors, edge cases
// - Lifecycle tests: See ha_lifecycle_test.go
package ha_test

import (
	"context"
	"errors"
	"testing"

	haResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/ha"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Common test constants
const (
	resourceName = "test-ha"

	id100Float = 100.0

	group1 = "grp1"
	group2 = "grp2"
)

// mockHAOperations is a mock implementation of proxmox.HAOperations for testing
type mockHAOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.HAInputs) error
	getFunc    func(ctx context.Context, id int) (*proxmox.HAOutputs, error)
	updateFunc func(ctx context.Context, id int, inputs proxmox.HAInputs, oldOutputs proxmox.HAOutputs) error
	deleteFunc func(ctx context.Context, id int) error
}

func (m *mockHAOperations) Create(ctx context.Context, inputs proxmox.HAInputs) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, inputs)
	}
	return nil
}

func (m *mockHAOperations) Get(ctx context.Context, id int) (*proxmox.HAOutputs, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return &proxmox.HAOutputs{
		HAInputs: proxmox.HAInputs{
			ResourceID: id,
			State:      proxmox.HAStateStarted,
		},
	}, nil
}

func (m *mockHAOperations) Update(
	ctx context.Context,
	id int,
	inputs proxmox.HAInputs,
	oldOutputs proxmox.HAOutputs,
) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, id, inputs, oldOutputs)
	}
	return nil
}

func (m *mockHAOperations) Delete(ctx context.Context, id int) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func TestHACreateSuccess(t *testing.T) {
	t.Parallel()

	mockOps := &mockHAOperations{
		createFunc: func(ctx context.Context, inputs proxmox.HAInputs) error {
			// Verify inputs
			assert.Equal(t, int(id100Float), inputs.ResourceID)
			assert.Equal(t, group1, inputs.Group)
			assert.Equal(t, proxmox.HAStateStarted, inputs.State)
			return nil
		},
	}

	ha := &haResource.HA{HAOps: mockOps}
	req := infer.CreateRequest[proxmox.HAInputs]{
		Name:   resourceName,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float), Group: group1, State: proxmox.HAStateStarted},
	}
	resp, err := ha.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, resourceName, resp.ID)
	assert.Equal(t, int(id100Float), resp.Output.ResourceID)
	assert.Equal(t, group1, resp.Output.Group)
	assert.Equal(t, proxmox.HAStateStarted, resp.Output.State)
}

func TestHACreateBackendFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("failed to create HA resource: 500 Internal Server Error")
	mockOps := &mockHAOperations{
		createFunc: func(ctx context.Context, inputs proxmox.HAInputs) error {
			return expectedErr
		},
	}

	ha := &haResource.HA{HAOps: mockOps}
	req := infer.CreateRequest[proxmox.HAInputs]{
		Name:   resourceName,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float), Group: group1, State: proxmox.HAStateStarted},
	}
	_, err := ha.Create(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestHACreateClientAcquisitionFailure(t *testing.T) {
	t.Parallel()

	// Test with HAOps not configured (simulates client acquisition failure)
	ha := &haResource.HA{} // No HAOps injected
	req := infer.CreateRequest[proxmox.HAInputs]{
		Name:   resourceName,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float), Group: group1, State: proxmox.HAStateStarted},
	}
	_, err := ha.Create(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "HAOperations not configured", err.Error())
}

func TestHAReadSuccess(t *testing.T) {
	t.Parallel()

	mockOps := &mockHAOperations{
		getFunc: func(ctx context.Context, id int) (*proxmox.HAOutputs, error) {
			assert.Equal(t, int(id100Float), id)
			return &proxmox.HAOutputs{
				HAInputs: proxmox.HAInputs{
					ResourceID: int(id100Float),
					Group:      group1,
					State:      proxmox.HAStateStarted,
				},
			}, nil
		},
	}

	ha := &haResource.HA{HAOps: mockOps}
	req := infer.ReadRequest[proxmox.HAInputs, proxmox.HAOutputs]{
		ID:     id100,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float)},
		State:  proxmox.HAOutputs{HAInputs: proxmox.HAInputs{ResourceID: int(id100Float)}},
	}
	resp, err := ha.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int(id100Float), resp.State.ResourceID)
	assert.Equal(t, group1, resp.State.Group)
	assert.Equal(t, proxmox.HAState(stateStarted), resp.State.State)
}

func TestHAReadFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("failed to get HA resource: 500 Internal Server Error")
	mockOps := &mockHAOperations{
		getFunc: func(ctx context.Context, id int) (*proxmox.HAOutputs, error) {
			return nil, expectedErr
		},
	}

	ha := &haResource.HA{HAOps: mockOps}
	req := infer.ReadRequest[proxmox.HAInputs, proxmox.HAOutputs]{
		ID:     id100,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float)},
		State:  proxmox.HAOutputs{HAInputs: proxmox.HAInputs{ResourceID: int(id100Float)}},
	}
	_, err := ha.Read(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestHAReadClientAcquisitionFailure(t *testing.T) {
	t.Parallel()

	// Test with HAOps not configured
	ha := &haResource.HA{} // No HAOps injected
	req := infer.ReadRequest[proxmox.HAInputs, proxmox.HAOutputs]{
		ID:     id100,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float)},
		State:  proxmox.HAOutputs{HAInputs: proxmox.HAInputs{ResourceID: int(id100Float)}},
	}
	_, err := ha.Read(context.Background(), req)
	assert.Equal(t, "HAOperations not configured", err.Error())
}

func TestHAUpdateClientAcquisitionFailure(t *testing.T) {
	t.Parallel()

	// Test with HAOps not configured
	ha := &haResource.HA{} // No HAOps injected
	req := infer.UpdateRequest[proxmox.HAInputs, proxmox.HAOutputs]{
		ID:     id100,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float), State: proxmox.HAStateStarted},
		State:  proxmox.HAOutputs{HAInputs: proxmox.HAInputs{ResourceID: int(id100Float), State: proxmox.HAStateStopped}},
	}
	_, err := ha.Update(context.Background(), req)
	assert.Equal(t, "HAOperations not configured", err.Error())
}

func TestHAUpdateBackendFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("failed to update HA resource: 500 Internal Server Error")
	mockOps := &mockHAOperations{
		updateFunc: func(ctx context.Context, id int, inputs proxmox.HAInputs, oldOutputs proxmox.HAOutputs) error {
			return expectedErr
		},
	}

	ha := &haResource.HA{HAOps: mockOps}
	req := infer.UpdateRequest[proxmox.HAInputs, proxmox.HAOutputs]{
		ID:     id100,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float), State: proxmox.HAStateStopped, Group: group2},
		State: proxmox.HAOutputs{
			HAInputs: proxmox.HAInputs{ResourceID: int(id100Float), State: proxmox.HAStateStarted, Group: group1},
		},
	}
	_, err := ha.Update(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestHAUpdateRemoveGroupSuccess(t *testing.T) {
	t.Parallel()

	mockOps := &mockHAOperations{
		updateFunc: func(ctx context.Context, id int, inputs proxmox.HAInputs, oldOutputs proxmox.HAOutputs) error {
			// Verify that group is being removed
			assert.Empty(t, inputs.Group)
			assert.Equal(t, group1, oldOutputs.Group)
			return nil
		},
	}

	ha := &haResource.HA{HAOps: mockOps}
	req := infer.UpdateRequest[proxmox.HAInputs, proxmox.HAOutputs]{
		ID:     id100,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float), State: proxmox.HAStateStarted, Group: ""}, // clearing group
		State: proxmox.HAOutputs{
			HAInputs: proxmox.HAInputs{ResourceID: int(id100Float), State: proxmox.HAStateStarted, Group: group1},
		},
	}
	resp, err := ha.Update(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Output.Group)
}

func TestHADeleteSuccess(t *testing.T) {
	t.Parallel()

	mockOps := &mockHAOperations{
		deleteFunc: func(ctx context.Context, id int) error {
			assert.Equal(t, int(id100Float), id)
			return nil
		},
	}

	ha := &haResource.HA{HAOps: mockOps}
	req := infer.DeleteRequest[proxmox.HAOutputs]{
		ID:    id100,
		State: proxmox.HAOutputs{HAInputs: proxmox.HAInputs{ResourceID: int(id100Float)}},
	}
	_, err := ha.Delete(context.Background(), req)
	require.NoError(t, err)
}

func TestHADeleteBackendFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("failed to delete HA resource: 500 Internal Server Error")
	mockOps := &mockHAOperations{
		deleteFunc: func(ctx context.Context, id int) error {
			return expectedErr
		},
	}

	ha := &haResource.HA{HAOps: mockOps}
	req := infer.DeleteRequest[proxmox.HAOutputs]{
		ID:    id100,
		State: proxmox.HAOutputs{HAInputs: proxmox.HAInputs{ResourceID: int(id100Float)}},
	}
	_, err := ha.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestHADeleteClientAcquisitionFailure(t *testing.T) {
	t.Parallel()

	// Test with HAOps not configured (simulates client acquisition failure)
	ha := &haResource.HA{} // No HAOps injected
	req := infer.DeleteRequest[proxmox.HAOutputs]{
		ID:    id100,
		State: proxmox.HAOutputs{HAInputs: proxmox.HAInputs{ResourceID: int(id100Float)}},
	}
	_, err := ha.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "HAOperations not configured", err.Error())
}

// Edge case tests

func TestHACreateWithoutGroup(t *testing.T) {
	t.Parallel()

	mockOps := &mockHAOperations{
		createFunc: func(ctx context.Context, inputs proxmox.HAInputs) error {
			// Verify no group provided
			assert.Empty(t, inputs.Group)
			assert.Equal(t, int(id100Float), inputs.ResourceID)
			assert.Equal(t, proxmox.HAStateStarted, inputs.State)
			return nil
		},
	}

	ha := &haResource.HA{HAOps: mockOps}
	req := infer.CreateRequest[proxmox.HAInputs]{
		Name:   resourceName,
		Inputs: proxmox.HAInputs{ResourceID: int(id100Float), State: proxmox.HAStateStarted}, // No group
	}
	resp, err := ha.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int(id100Float), resp.Output.ResourceID)
	assert.Empty(t, resp.Output.Group)
	assert.Equal(t, proxmox.HAStateStarted, resp.Output.State)
}
