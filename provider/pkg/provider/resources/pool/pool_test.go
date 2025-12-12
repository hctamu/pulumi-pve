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

// Package pool_test contains unit tests for Pool resource operations.
//
// Test Organization:
// - Unit tests: Direct method calls with mocked PoolOperations interface
// - Error scenarios: Operation failures, validation errors, edge cases
// - Lifecycle tests: See pool_lifecycle_test.go
package pool_test

import (
	"context"
	"errors"
	"testing"

	poolResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/pool"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Common test constants
const (
	resourceName = "test-pool"
	poolName1    = "pool1"
	poolName2    = "pool2"
	comment1     = "Test pool comment"
	comment2     = "Updated pool comment"
)

// mockPoolOperations is a mock implementation of proxmox.PoolOperations for testing
type mockPoolOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.PoolInputs) error
	getFunc    func(ctx context.Context, name string) (*proxmox.PoolOutputs, error)
	updateFunc func(ctx context.Context, name string, inputs proxmox.PoolInputs) error
	deleteFunc func(ctx context.Context, name string) error
}

func (m *mockPoolOperations) Create(ctx context.Context, inputs proxmox.PoolInputs) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, inputs)
	}
	return nil
}

func (m *mockPoolOperations) Get(ctx context.Context, name string) (*proxmox.PoolOutputs, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, name)
	}
	return &proxmox.PoolOutputs{
		PoolInputs: proxmox.PoolInputs{
			Name:    name,
			Comment: comment1,
		},
	}, nil
}

func (m *mockPoolOperations) Update(ctx context.Context, name string, inputs proxmox.PoolInputs) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, name, inputs)
	}
	return nil
}

func (m *mockPoolOperations) Delete(ctx context.Context, name string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, name)
	}
	return nil
}

func TestPoolCreateSuccess(t *testing.T) {
	t.Parallel()

	mockOps := &mockPoolOperations{
		createFunc: func(ctx context.Context, inputs proxmox.PoolInputs) error {
			// Verify inputs
			assert.Equal(t, poolName1, inputs.Name)
			assert.Equal(t, comment1, inputs.Comment)
			return nil
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.CreateRequest[proxmox.PoolInputs]{
		Name:   resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1, Comment: comment1},
	}
	resp, err := pool.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, resourceName, resp.ID)
	assert.Equal(t, poolName1, resp.Output.Name)
	assert.Equal(t, comment1, resp.Output.Comment)
}

func TestPoolCreateBackendFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("failed to create Pool resource: 500 Internal Server Error")
	mockOps := &mockPoolOperations{
		createFunc: func(ctx context.Context, inputs proxmox.PoolInputs) error {
			return expectedErr
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.CreateRequest[proxmox.PoolInputs]{
		Name:   resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1, Comment: comment1},
	}
	_, err := pool.Create(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestPoolCreateClientAcquisitionFailure(t *testing.T) {
	t.Parallel()

	// Test with PoolOps not configured (simulates client acquisition failure)
	pool := &poolResource.Pool{} // No PoolOps injected
	req := infer.CreateRequest[proxmox.PoolInputs]{
		Name:   resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1, Comment: comment1},
	}
	_, err := pool.Create(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PoolOperations not configured")
}

func TestPoolCreateDryRun(t *testing.T) {
	t.Parallel()

	mockOps := &mockPoolOperations{
		createFunc: func(ctx context.Context, inputs proxmox.PoolInputs) error {
			t.Error("Create should not be called during dry run")
			return nil
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.CreateRequest[proxmox.PoolInputs]{
		Name:   resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1, Comment: comment1},
		DryRun: true,
	}
	resp, err := pool.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, resourceName, resp.ID)
	assert.Equal(t, poolName1, resp.Output.Name)
}

func TestPoolReadSuccess(t *testing.T) {
	t.Parallel()

	mockOps := &mockPoolOperations{
		getFunc: func(ctx context.Context, name string) (*proxmox.PoolOutputs, error) {
			assert.Equal(t, poolName1, name)
			return &proxmox.PoolOutputs{
				PoolInputs: proxmox.PoolInputs{
					Name:    poolName1,
					Comment: comment1,
				},
			}, nil
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.ReadRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1},
	}
	resp, err := pool.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, poolName1, resp.State.Name)
	assert.Equal(t, comment1, resp.State.Comment)
}

func TestPoolReadFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("failed to get Pool resource: 404 Not Found")
	mockOps := &mockPoolOperations{
		getFunc: func(ctx context.Context, name string) (*proxmox.PoolOutputs, error) {
			return nil, expectedErr
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.ReadRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1},
	}
	_, err := pool.Read(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestPoolReadClientAcquisitionFailure(t *testing.T) {
	t.Parallel()

	// Test with PoolOps not configured
	pool := &poolResource.Pool{} // No PoolOps injected
	req := infer.ReadRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1},
	}
	_, err := pool.Read(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PoolOperations not configured")
}

func TestPoolReadMissingID(t *testing.T) {
	t.Parallel()

	mockOps := &mockPoolOperations{}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.ReadRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     "", // Missing ID
		Inputs: proxmox.PoolInputs{Name: poolName1},
	}
	_, err := pool.Read(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing pool ID")
}

func TestPoolUpdateSuccess(t *testing.T) {
	t.Parallel()

	mockOps := &mockPoolOperations{
		updateFunc: func(ctx context.Context, name string, inputs proxmox.PoolInputs) error {
			assert.Equal(t, poolName1, name)
			assert.Equal(t, comment2, inputs.Comment)
			return nil
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.UpdateRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1, Comment: comment2},
		State:  proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{Name: poolName1, Comment: comment1}},
	}
	resp, err := pool.Update(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, poolName1, resp.Output.Name)
	assert.Equal(t, comment2, resp.Output.Comment)
}

func TestPoolUpdateBackendFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("failed to update Pool resource: 500 Internal Server Error")
	mockOps := &mockPoolOperations{
		updateFunc: func(ctx context.Context, name string, inputs proxmox.PoolInputs) error {
			return expectedErr
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.UpdateRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1, Comment: comment2},
		State:  proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{Name: poolName1, Comment: comment1}},
	}
	_, err := pool.Update(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestPoolUpdateClientAcquisitionFailure(t *testing.T) {
	t.Parallel()

	// Test with PoolOps not configured
	pool := &poolResource.Pool{} // No PoolOps injected
	req := infer.UpdateRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1, Comment: comment2},
		State:  proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{Name: poolName1, Comment: comment1}},
	}
	_, err := pool.Update(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PoolOperations not configured")
}

func TestPoolUpdateDryRun(t *testing.T) {
	t.Parallel()

	mockOps := &mockPoolOperations{
		updateFunc: func(ctx context.Context, name string, inputs proxmox.PoolInputs) error {
			t.Error("Update should not be called during dry run")
			return nil
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.UpdateRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1, Comment: comment2},
		State:  proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{Name: poolName1, Comment: comment1}},
		DryRun: true,
	}
	resp, err := pool.Update(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, comment1, resp.Output.Comment) // Should return old state during dry run
}

func TestPoolDeleteSuccess(t *testing.T) {
	t.Parallel()

	mockOps := &mockPoolOperations{
		deleteFunc: func(ctx context.Context, name string) error {
			assert.Equal(t, poolName1, name)
			return nil
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.DeleteRequest[proxmox.PoolOutputs]{
		ID:    resourceName,
		State: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{Name: poolName1}},
	}
	_, err := pool.Delete(context.Background(), req)
	require.NoError(t, err)
}

func TestPoolDeleteBackendFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("failed to delete Pool resource: 500 Internal Server Error")
	mockOps := &mockPoolOperations{
		deleteFunc: func(ctx context.Context, name string) error {
			return expectedErr
		},
	}

	pool := &poolResource.Pool{PoolOps: mockOps}
	req := infer.DeleteRequest[proxmox.PoolOutputs]{
		ID:    resourceName,
		State: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{Name: poolName1}},
	}
	_, err := pool.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestPoolDeleteClientAcquisitionFailure(t *testing.T) {
	t.Parallel()

	// Test with PoolOps not configured (simulates client acquisition failure)
	pool := &poolResource.Pool{} // No PoolOps injected
	req := infer.DeleteRequest[proxmox.PoolOutputs]{
		ID:    resourceName,
		State: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{Name: poolName1}},
	}
	_, err := pool.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PoolOperations not configured")
}
