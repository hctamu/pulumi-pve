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

package group_test

import (
	"context"
	"testing"

	groupResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/group"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	groupID             = "testgroup"
	groupComment        = "comment"
	updatedGroupComment = "updated comment"
)

type mockGroupOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.GroupInputs) error
	getFunc    func(ctx context.Context, id string) (*proxmox.GroupOutputs, error)
	updateFunc func(ctx context.Context, id string, inputs proxmox.GroupInputs) error
	deleteFunc func(ctx context.Context, id string) error
}

func (m *mockGroupOperations) Create(ctx context.Context, inputs proxmox.GroupInputs) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, inputs)
	}
	return nil
}

func (m *mockGroupOperations) Get(ctx context.Context, id string) (*proxmox.GroupOutputs, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return &proxmox.GroupOutputs{
		GroupInputs: proxmox.GroupInputs{
			Name: id,
		},
	}, nil
}

func (m *mockGroupOperations) Update(ctx context.Context, id string, inputs proxmox.GroupInputs) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, id, inputs)
	}
	return nil
}

func (m *mockGroupOperations) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

type notFoundError struct{ msg string }

func (e *notFoundError) Error() string { return e.msg }

func TestGroupOperationsNotConfigured(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		group := &groupResource.Group{}
		_, err := group.Create(context.Background(), infer.CreateRequest[proxmox.GroupInputs]{
			Inputs: proxmox.GroupInputs{Name: groupID},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "GroupOperations not configured")
	})

	t.Run("read", func(t *testing.T) {
		t.Parallel()
		group := &groupResource.Group{}
		_, err := group.Read(context.Background(), infer.ReadRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
			ID:     groupID,
			Inputs: proxmox.GroupInputs{Name: groupID},
			State:  proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "GroupOperations not configured")
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		group := &groupResource.Group{}
		_, err := group.Update(context.Background(), infer.UpdateRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
			ID:     groupID,
			Inputs: proxmox.GroupInputs{Name: groupID, Comment: updatedGroupComment},
			State:  proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "GroupOperations not configured")
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		group := &groupResource.Group{}
		_, err := group.Delete(context.Background(), infer.DeleteRequest[proxmox.GroupOutputs]{
			State: proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "GroupOperations not configured")
	})
}

func TestGroupCreateSuccess(t *testing.T) {
	t.Parallel()

	called := false
	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{createFunc: func(ctx context.Context, inputs proxmox.GroupInputs) error {
			called = true
			assert.Equal(t, proxmox.GroupInputs{Name: groupID, Comment: groupComment}, inputs)
			return nil
		}},
	}

	resp, err := group.Create(context.Background(), infer.CreateRequest[proxmox.GroupInputs]{
		Name: groupID,
		Inputs: proxmox.GroupInputs{
			Name:    groupID,
			Comment: groupComment,
		},
	})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, groupID, resp.ID)
	assert.Equal(t, groupComment, resp.Output.Comment)
}

func TestGroupCreateDryRunDoesNotCallAdapter(t *testing.T) {
	t.Parallel()

	called := false
	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{createFunc: func(ctx context.Context, inputs proxmox.GroupInputs) error {
			called = true
			return nil
		}},
	}

	resp, err := group.Create(context.Background(), infer.CreateRequest[proxmox.GroupInputs]{
		Name:   groupID,
		DryRun: true,
		Inputs: proxmox.GroupInputs{Name: groupID, Comment: groupComment},
	})
	require.NoError(t, err)
	assert.False(t, called)
	assert.Equal(t, groupID, resp.ID)
}

func TestGroupCreateAdapterError(t *testing.T) {
	t.Parallel()

	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{createFunc: func(ctx context.Context, inputs proxmox.GroupInputs) error {
			return assert.AnError
		}},
	}

	_, err := group.Create(context.Background(), infer.CreateRequest[proxmox.GroupInputs]{
		Inputs: proxmox.GroupInputs{Name: groupID},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestGroupReadIDNotFound(t *testing.T) {
	t.Parallel()

	called := false
	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{getFunc: func(ctx context.Context, name string) (*proxmox.GroupOutputs, error) {
			called = true
			return &proxmox.GroupOutputs{}, nil
		}},
	}

	resp, err := group.Read(context.Background(), infer.ReadRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
		ID:     "",
		Inputs: proxmox.GroupInputs{Name: groupID},
		State:  proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
	})
	require.NoError(t, err)
	assert.Equal(t, "", resp.ID)
	assert.False(t, called)
}

func TestGroupReadNotFoundIsTreatedAsDeleted(t *testing.T) {
	t.Parallel()

	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{
			getFunc: func(ctx context.Context, id string) (*proxmox.GroupOutputs, error) {
				return nil, &notFoundError{msg: "group '" + groupID + "' does not exist\n"}
			},
		},
	}

	resp, err := group.Read(context.Background(), infer.ReadRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
		ID:     groupID,
		Inputs: proxmox.GroupInputs{Name: groupID},
		State:  proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.ID)
}

func TestGroupReadSuccess(t *testing.T) {
	t.Parallel()

	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{getFunc: func(ctx context.Context, name string) (*proxmox.GroupOutputs, error) {
			assert.Equal(t, groupID, name)
			return &proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{
				Name:    groupID,
				Comment: updatedGroupComment,
			}}, nil
		}},
	}

	resp, err := group.Read(context.Background(), infer.ReadRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
		ID: groupID,
		Inputs: proxmox.GroupInputs{
			Name: groupID,
		},
		State: proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID, Comment: groupComment}},
	})
	require.NoError(t, err)
	assert.Equal(t, groupID, resp.ID)
	assert.Equal(t, updatedGroupComment, resp.State.Comment)
}

func TestGroupReadAdapterError(t *testing.T) {
	t.Parallel()

	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{getFunc: func(ctx context.Context, name string) (*proxmox.GroupOutputs, error) {
			return nil, assert.AnError
		}},
	}

	_, err := group.Read(context.Background(), infer.ReadRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
		ID:     groupID,
		Inputs: proxmox.GroupInputs{Name: groupID},
		State:  proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestGroupUpdateSuccess(t *testing.T) {
	t.Parallel()

	var updatedID string
	var updatedInputs proxmox.GroupInputs

	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{updateFunc: func(ctx context.Context, id string, inputs proxmox.GroupInputs) error {
			updatedID = id
			updatedInputs = inputs
			return nil
		}},
	}

	state := proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{
		Name:    groupID,
		Comment: groupComment,
	}}

	resp, err := group.Update(context.Background(), infer.UpdateRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
		ID: groupID,
		Inputs: proxmox.GroupInputs{
			Name:    groupID,
			Comment: updatedGroupComment,
		},
		State: state,
	})
	require.NoError(t, err)

	assert.Equal(t, groupID, updatedID)
	assert.Equal(t, updatedGroupComment, updatedInputs.Comment)
	assert.Equal(t, updatedGroupComment, resp.Output.Comment)
}

func TestGroupUpdateDryRunDoesNotCallAdapter(t *testing.T) {
	t.Parallel()

	called := false
	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{updateFunc: func(ctx context.Context, id string, inputs proxmox.GroupInputs) error {
			called = true
			return nil
		}},
	}

	_, err := group.Update(context.Background(), infer.UpdateRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
		ID:     groupID,
		DryRun: true,
		Inputs: proxmox.GroupInputs{Name: groupID, Comment: updatedGroupComment},
		State:  proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID, Comment: groupComment}},
	})
	require.NoError(t, err)
	assert.False(t, called)
}

func TestGroupUpdateAdapterError(t *testing.T) {
	t.Parallel()

	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{updateFunc: func(ctx context.Context, id string, inputs proxmox.GroupInputs) error {
			return assert.AnError
		}},
	}

	_, err := group.Update(context.Background(), infer.UpdateRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
		ID:     groupID,
		Inputs: proxmox.GroupInputs{Name: groupID, Comment: updatedGroupComment},
		State:  proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID, Comment: groupComment}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestGroupDeleteSuccess(t *testing.T) {
	t.Parallel()

	var deleted string
	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{deleteFunc: func(ctx context.Context, name string) error {
			deleted = name
			return nil
		}},
	}

	_, err := group.Delete(context.Background(), infer.DeleteRequest[proxmox.GroupOutputs]{
		State: proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
	})
	require.NoError(t, err)
	assert.Equal(t, groupID, deleted)
}

func TestGroupDeleteAdapterError(t *testing.T) {
	t.Parallel()

	group := &groupResource.Group{
		GroupOps: &mockGroupOperations{deleteFunc: func(ctx context.Context, name string) error {
			return assert.AnError
		}},
	}

	_, err := group.Delete(context.Background(), infer.DeleteRequest[proxmox.GroupOutputs]{
		State: proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}
