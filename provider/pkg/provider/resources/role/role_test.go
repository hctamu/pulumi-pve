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

package role_test

import (
	"context"
	"testing"

	roleResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/role"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	roleID         = "testrole"
	rolePrivilege  = "some.privilege"
	rolePrivilege2 = "other.privilege"
)

type notFoundError struct{ msg string }

func (e *notFoundError) Error() string { return e.msg }

type mockRoleOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.RoleInputs) error
	getFunc    func(ctx context.Context, id string) (*proxmox.RoleOutputs, error)
	updateFunc func(ctx context.Context, id string, inputs proxmox.RoleInputs) error
	deleteFunc func(ctx context.Context, id string) error
}

func (m *mockRoleOperations) Create(ctx context.Context, inputs proxmox.RoleInputs) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, inputs)
	}
	return nil
}

func (m *mockRoleOperations) Get(ctx context.Context, id string) (*proxmox.RoleOutputs, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return &proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: id}}, nil
}

func (m *mockRoleOperations) Update(ctx context.Context, id string, inputs proxmox.RoleInputs) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, id, inputs)
	}
	return nil
}

func (m *mockRoleOperations) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func TestRoleOperationsNotConfigured(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		role := &roleResource.Role{}
		_, err := role.Create(context.Background(), infer.CreateRequest[proxmox.RoleInputs]{
			Inputs: proxmox.RoleInputs{
				Name: roleID,
			},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "RoleOperations not configured")
	})

	t.Run("read", func(t *testing.T) {
		t.Parallel()
		role := &roleResource.Role{}
		_, err := role.Read(context.Background(), infer.ReadRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
			ID:     roleID,
			Inputs: proxmox.RoleInputs{Name: roleID},
			State:  proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "RoleOperations not configured")
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		role := &roleResource.Role{}
		_, err := role.Update(context.Background(), infer.UpdateRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
			ID:     roleID,
			Inputs: proxmox.RoleInputs{Name: roleID, Privileges: []string{rolePrivilege}},
			State:  proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "RoleOperations not configured")
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		role := &roleResource.Role{}
		_, err := role.Delete(context.Background(), infer.DeleteRequest[proxmox.RoleOutputs]{
			State: proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "RoleOperations not configured")
	})
}

func TestRoleCreateSuccess(t *testing.T) {
	t.Parallel()

	called := false
	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{createFunc: func(ctx context.Context, inputs proxmox.RoleInputs) error {
			called = true
			assert.Equal(t, proxmox.RoleInputs{Name: roleID, Privileges: []string{rolePrivilege}}, inputs)
			return nil
		}},
	}

	resp, err := role.Create(context.Background(), infer.CreateRequest[proxmox.RoleInputs]{
		Name: roleID,
		Inputs: proxmox.RoleInputs{
			Name:       roleID,
			Privileges: []string{rolePrivilege},
		},
	})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, roleID, resp.ID)
	assert.Equal(t, []string{rolePrivilege}, resp.Output.Privileges)
}

func TestRoleCreateDryRunDoesNotCallAdapter(t *testing.T) {
	t.Parallel()

	called := false
	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{createFunc: func(ctx context.Context, inputs proxmox.RoleInputs) error {
			called = true
			return nil
		}},
	}

	resp, err := role.Create(context.Background(), infer.CreateRequest[proxmox.RoleInputs]{
		Name:   roleID,
		DryRun: true,
		Inputs: proxmox.RoleInputs{Name: roleID},
	})
	require.NoError(t, err)
	assert.False(t, called)
	assert.Equal(t, roleID, resp.ID)
}

func TestRoleCreateAdapterError(t *testing.T) {
	t.Parallel()

	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{createFunc: func(ctx context.Context, inputs proxmox.RoleInputs) error {
			return assert.AnError
		}},
	}

	_, err := role.Create(context.Background(), infer.CreateRequest[proxmox.RoleInputs]{
		Inputs: proxmox.RoleInputs{Name: roleID},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestRoleReadIDNotFound(t *testing.T) {
	t.Parallel()

	called := false
	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{getFunc: func(ctx context.Context, id string) (*proxmox.RoleOutputs, error) {
			called = true
			return &proxmox.RoleOutputs{}, nil
		}},
	}

	resp, err := role.Read(context.Background(), infer.ReadRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
		ID:     "",
		Inputs: proxmox.RoleInputs{Name: roleID},
		State:  proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID}},
	})
	require.NoError(t, err)
	assert.Equal(t, "", resp.ID)
	assert.False(t, called)
}

func TestRoleReadNotFoundIsTreatedAsDeleted(t *testing.T) {
	t.Parallel()

	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{getFunc: func(ctx context.Context, id string) (*proxmox.RoleOutputs, error) {
			return nil, &notFoundError{msg: "role '" + roleID + "' does not exist\n"}
		}},
	}

	resp, err := role.Read(context.Background(), infer.ReadRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
		ID:     roleID,
		Inputs: proxmox.RoleInputs{Name: roleID},
		State:  proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID}},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.ID)
}

func TestRoleReadSuccess(t *testing.T) {
	t.Parallel()

	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{getFunc: func(ctx context.Context, id string) (*proxmox.RoleOutputs, error) {
			assert.Equal(t, roleID, id)
			return &proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{
				Name:       roleID,
				Privileges: []string{rolePrivilege2, rolePrivilege},
			}}, nil
		}},
	}

	resp, err := role.Read(context.Background(), infer.ReadRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
		ID: roleID,
		Inputs: proxmox.RoleInputs{
			Name: roleID,
		},
		State: proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID}},
	})
	require.NoError(t, err)
	assert.Equal(t, roleID, resp.ID)
	assert.Equal(t, []string{rolePrivilege2, rolePrivilege}, resp.State.Privileges)
}

func TestRoleReadAdapterError(t *testing.T) {
	t.Parallel()

	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{getFunc: func(ctx context.Context, id string) (*proxmox.RoleOutputs, error) {
			return nil, assert.AnError
		}},
	}

	_, err := role.Read(context.Background(), infer.ReadRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
		ID:     roleID,
		Inputs: proxmox.RoleInputs{Name: roleID},
		State:  proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestRoleUpdateAdapterError(t *testing.T) {
	t.Parallel()

	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{updateFunc: func(ctx context.Context, id string, inputs proxmox.RoleInputs) error {
			return assert.AnError
		}},
	}

	_, err := role.Update(context.Background(), infer.UpdateRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
		Inputs: proxmox.RoleInputs{Name: roleID, Privileges: []string{rolePrivilege2, rolePrivilege}},
		State:  proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID, Privileges: []string{rolePrivilege}}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestRoleUpdateSuccess(t *testing.T) {
	t.Parallel()

	var updatedID string
	var updatedInputs proxmox.RoleInputs

	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{updateFunc: func(ctx context.Context, id string, inputs proxmox.RoleInputs) error {
			updatedID = id
			updatedInputs = inputs
			return nil
		}},
	}

	state := proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{
		Name:       roleID,
		Privileges: []string{rolePrivilege},
	}}

	resp, err := role.Update(context.Background(), infer.UpdateRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
		ID: roleID,
		Inputs: proxmox.RoleInputs{
			Name:       roleID,
			Privileges: []string{rolePrivilege2, rolePrivilege},
		},
		State: state,
	})
	require.NoError(t, err)

	assert.Equal(t, roleID, updatedID)
	assert.Equal(t, []string{rolePrivilege2, rolePrivilege}, updatedInputs.Privileges)
	assert.Equal(t, []string{rolePrivilege2, rolePrivilege}, resp.Output.Privileges)
}

func TestRoleUpdateDryRunDoesNotCallAdapter(t *testing.T) {
	t.Parallel()

	called := false
	role := &roleResource.Role{
		RoleOps: &mockRoleOperations{updateFunc: func(ctx context.Context, id string, inputs proxmox.RoleInputs) error {
			called = true
			return nil
		}},
	}

	_, err := role.Update(context.Background(), infer.UpdateRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
		ID:     roleID,
		DryRun: true,
		Inputs: proxmox.RoleInputs{Name: roleID, Privileges: []string{rolePrivilege2, rolePrivilege}},
		State:  proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID, Privileges: []string{rolePrivilege}}},
	})
	require.NoError(t, err)
	assert.False(t, called)
}

func TestRoleDeleteAdapterError(t *testing.T) {
	t.Parallel()

	role := &roleResource.Role{RoleOps: &mockRoleOperations{deleteFunc: func(ctx context.Context, id string) error {
		return assert.AnError
	}}}

	_, err := role.Delete(context.Background(), infer.DeleteRequest[proxmox.RoleOutputs]{
		State: proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestRoleDeleteSuccess(t *testing.T) {
	t.Parallel()

	var deleted string
	role := &roleResource.Role{RoleOps: &mockRoleOperations{deleteFunc: func(ctx context.Context, id string) error {
		deleted = id
		return nil
	}}}

	_, err := role.Delete(context.Background(), infer.DeleteRequest[proxmox.RoleOutputs]{
		State: proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{Name: roleID}},
	})
	require.NoError(t, err)
	assert.Equal(t, roleID, deleted)
}

func TestRoleDiffReplaceOnChange(t *testing.T) {
	t.Parallel()

	role := &roleResource.Role{}
	resp, err := role.Diff(context.Background(), infer.DiffRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
		ID: roleID,
		Inputs: proxmox.RoleInputs{
			Name: "newrole",
		},
		State: proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{
			Name: roleID,
		}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["name"].Kind) // UpdateReplace
}

func TestRoleDiffTreatsListsAsSets(t *testing.T) {
	t.Parallel()

	role := &roleResource.Role{}
	resp, err := role.Diff(context.Background(), infer.DiffRequest[proxmox.RoleInputs, proxmox.RoleOutputs]{
		ID: roleID,
		Inputs: proxmox.RoleInputs{
			Name:       roleID,
			Privileges: []string{rolePrivilege2, rolePrivilege},
		},
		State: proxmox.RoleOutputs{RoleInputs: proxmox.RoleInputs{
			Name:       roleID,
			Privileges: []string{rolePrivilege, rolePrivilege2},
		}},
	})
	require.NoError(t, err)
	assert.False(t, resp.HasChanges)
}
