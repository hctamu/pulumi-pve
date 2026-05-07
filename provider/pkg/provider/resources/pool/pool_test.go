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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	poolResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/pool"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
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
	updateFunc func(ctx context.Context, name string, state proxmox.PoolInputs, inputs proxmox.PoolInputs) error
	deleteFunc func(ctx context.Context, name string) error
}

func (mock *mockPoolOperations) Create(ctx context.Context, inputs proxmox.PoolInputs) error {
	if mock.createFunc != nil {
		return mock.createFunc(ctx, inputs)
	}
	return nil
}

func (mock *mockPoolOperations) Get(ctx context.Context, name string) (*proxmox.PoolOutputs, error) {
	if mock.getFunc != nil {
		return mock.getFunc(ctx, name)
	}
	return &proxmox.PoolOutputs{
		PoolInputs: proxmox.PoolInputs{
			Name:    name,
			Comment: comment1,
		},
	}, nil
}

func (mock *mockPoolOperations) Update(
	ctx context.Context,
	name string,
	state proxmox.PoolInputs,
	inputs proxmox.PoolInputs,
) error {
	if mock.updateFunc != nil {
		return mock.updateFunc(ctx, name, state, inputs)
	}
	return nil
}

func (mock *mockPoolOperations) Delete(ctx context.Context, name string) error {
	if mock.deleteFunc != nil {
		return mock.deleteFunc(ctx, name)
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
		updateFunc: func(ctx context.Context, name string, state proxmox.PoolInputs, inputs proxmox.PoolInputs) error {
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
		updateFunc: func(ctx context.Context, name string, state proxmox.PoolInputs, inputs proxmox.PoolInputs) error {
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
		updateFunc: func(ctx context.Context, name string, state proxmox.PoolInputs, inputs proxmox.PoolInputs) error {
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

func TestPoolCreateWithMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input proxmox.PoolInputs
	}{
		{
			name: "create with vms and storage",
			input: proxmox.PoolInputs{
				Name:    poolName1,
				Comment: comment1,
				VMs:     []int{100, 101},
				Storage: []string{"local", "ceph"},
			},
		},
		{
			name: "create with vms only",
			input: proxmox.PoolInputs{
				Name: poolName1,
				VMs:  []int{200},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var receivedInputs proxmox.PoolInputs
			mockOps := &mockPoolOperations{
				createFunc: func(ctx context.Context, inputs proxmox.PoolInputs) error {
					receivedInputs = inputs
					return nil
				},
			}

			p := &poolResource.Pool{PoolOps: mockOps}
			req := infer.CreateRequest[proxmox.PoolInputs]{
				Name:   resourceName,
				Inputs: tt.input,
			}
			resp, err := p.Create(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, tt.input.VMs, resp.Output.VMs)
			assert.Equal(t, tt.input.Storage, resp.Output.Storage)
			assert.Equal(t, tt.input.VMs, receivedInputs.VMs)
			assert.Equal(t, tt.input.Storage, receivedInputs.Storage)
		})
	}
}

func TestPoolReadWithMembers(t *testing.T) {
	t.Parallel()

	mockOps := &mockPoolOperations{
		getFunc: func(ctx context.Context, name string) (*proxmox.PoolOutputs, error) {
			return &proxmox.PoolOutputs{
				PoolInputs: proxmox.PoolInputs{
					Name:    name,
					Comment: comment1,
					VMs:     []int{100, 101},
					Storage: []string{"local"},
				},
			}, nil
		},
	}

	p := &poolResource.Pool{PoolOps: mockOps}
	req := infer.ReadRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     resourceName,
		Inputs: proxmox.PoolInputs{Name: poolName1},
	}
	resp, err := p.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, []int{100, 101}, resp.State.VMs)
	assert.Equal(t, []string{"local"}, resp.State.Storage)
}

func TestPoolDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputs      proxmox.PoolInputs
		state       proxmox.PoolOutputs
		wantChanges bool
		wantKeys    []string
	}{
		{
			name: "no changes",
			inputs: proxmox.PoolInputs{
				Name:    poolName1,
				Comment: comment1,
				VMs:     []int{100, 101},
				Storage: []string{"local"},
			},
			state: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{
				Name:    poolName1,
				Comment: comment1,
				VMs:     []int{100, 101},
				Storage: []string{"local"},
			}},
			wantChanges: false,
		},
		{
			name: "vms reordered — no change",
			inputs: proxmox.PoolInputs{
				Name: poolName1,
				VMs:  []int{101, 100},
			},
			state: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{
				Name: poolName1,
				VMs:  []int{100, 101},
			}},
			wantChanges: false,
		},
		{
			name: "storage reordered — no change",
			inputs: proxmox.PoolInputs{
				Name:    poolName1,
				Storage: []string{"ceph", "local"},
			},
			state: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{
				Name:    poolName1,
				Storage: []string{"local", "ceph"},
			}},
			wantChanges: false,
		},
		{
			name: "vms changed",
			inputs: proxmox.PoolInputs{
				Name: poolName1,
				VMs:  []int{100, 102},
			},
			state: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{
				Name: poolName1,
				VMs:  []int{100, 101},
			}},
			wantChanges: true,
			wantKeys:    []string{"vms"},
		},
		{
			name: "storage changed",
			inputs: proxmox.PoolInputs{
				Name:    poolName1,
				Storage: []string{"ceph"},
			},
			state: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{
				Name:    poolName1,
				Storage: []string{"local"},
			}},
			wantChanges: true,
			wantKeys:    []string{"storage"},
		},
		{
			name: "comment changed",
			inputs: proxmox.PoolInputs{
				Name:    poolName1,
				Comment: comment2,
			},
			state: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{
				Name:    poolName1,
				Comment: comment1,
			}},
			wantChanges: true,
			wantKeys:    []string{"comment"},
		},
		{
			name: "name changed triggers replace",
			inputs: proxmox.PoolInputs{
				Name: poolName2,
			},
			state: proxmox.PoolOutputs{PoolInputs: proxmox.PoolInputs{
				Name: poolName1,
			}},
			wantChanges: true,
			wantKeys:    []string{"name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &poolResource.Pool{PoolOps: &mockPoolOperations{}}
			req := infer.DiffRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
				ID:     resourceName,
				Inputs: tt.inputs,
				State:  tt.state,
			}
			resp, err := p.Diff(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, tt.wantChanges, resp.HasChanges)
			for _, key := range tt.wantKeys {
				assert.Contains(t, resp.DetailedDiff, key)
			}
		})
	}
}

// TestPoolReadImport tests Read behaviour during `pulumi import`, where Inputs.Name is
// empty and the pool name must be derived from the resource ID.
func TestPoolReadImport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          string
		inputsName  string
		wantGetName string
		wantName    string
	}{
		{
			name:        "import: Inputs.Name empty — falls back to ID",
			id:          poolName1,
			inputsName:  "",
			wantGetName: poolName1,
			wantName:    poolName1,
		},
		{
			name:        "normal read: Inputs.Name takes precedence over ID",
			id:          resourceName,
			inputsName:  poolName1,
			wantGetName: poolName1,
			wantName:    poolName1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotGetName string
			mockOps := &mockPoolOperations{
				getFunc: func(ctx context.Context, name string) (*proxmox.PoolOutputs, error) {
					gotGetName = name
					return &proxmox.PoolOutputs{
						PoolInputs: proxmox.PoolInputs{
							Name:    name,
							Comment: comment1,
						},
					}, nil
				},
			}

			p := &poolResource.Pool{PoolOps: mockOps}
			req := infer.ReadRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
				ID:     tt.id,
				Inputs: proxmox.PoolInputs{Name: tt.inputsName},
			}
			resp, err := p.Read(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, tt.wantGetName, gotGetName, "name passed to Get")
			assert.Equal(t, tt.wantName, resp.State.Name)
			assert.Equal(t, comment1, resp.State.Comment)
		})
	}
}

func TestPoolReadImportGetFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("failed to get Pool resource: 404 Not Found")
	mockOps := &mockPoolOperations{
		getFunc: func(ctx context.Context, name string) (*proxmox.PoolOutputs, error) {
			return nil, expectedErr
		},
	}

	p := &poolResource.Pool{PoolOps: mockOps}
	// Simulate import: Inputs.Name is empty, ID holds the pool name.
	req := infer.ReadRequest[proxmox.PoolInputs, proxmox.PoolOutputs]{
		ID:     poolName1,
		Inputs: proxmox.PoolInputs{},
	}
	_, err := p.Read(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}
