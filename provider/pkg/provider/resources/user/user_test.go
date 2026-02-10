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

package user_test

import (
	"context"
	"testing"

	userResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/user"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

const (
	userID        = "testuser"
	userComment   = "updated comment"
	userPassword  = "super-secret"
	userEmail     = "testuser@example.com"
	userFirstname = "Test"
	userLastname  = "User"

	userGroup1 = "g1"
	userGroup2 = "g2"

	userKey1 = "ssh-ed25519"
	userKey2 = "ssh-rsa"
)

type mockUserOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.UserInputs) error
	getFunc    func(ctx context.Context, id string) (*proxmox.UserOutputs, error)
	updateFunc func(ctx context.Context, id string, inputs proxmox.UserInputs) error
	deleteFunc func(ctx context.Context, id string) error
}

func (m *mockUserOperations) Create(ctx context.Context, inputs proxmox.UserInputs) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, inputs)
	}
	return nil
}

func (m *mockUserOperations) Get(ctx context.Context, id string) (*proxmox.UserOutputs, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return &proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: id}}, nil
}

func (m *mockUserOperations) Update(ctx context.Context, id string, inputs proxmox.UserInputs) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, id, inputs)
	}
	return nil
}

func (m *mockUserOperations) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

type notFoundError struct{ msg string }

func (e *notFoundError) Error() string { return e.msg }

func TestUserOperationsNotConfigured(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		user := &userResource.User{}
		_, err := user.Create(context.Background(), infer.CreateRequest[proxmox.UserInputs]{
			Inputs: proxmox.UserInputs{Name: userID},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "UserOperations not configured")
	})

	t.Run("read", func(t *testing.T) {
		t.Parallel()
		user := &userResource.User{}
		_, err := user.Read(context.Background(), infer.ReadRequest[proxmox.UserInputs, proxmox.UserOutputs]{
			ID:     userID,
			Inputs: proxmox.UserInputs{Name: userID},
			State:  proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "UserOperations not configured")
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		user := &userResource.User{}
		_, err := user.Update(context.Background(), infer.UpdateRequest[proxmox.UserInputs, proxmox.UserOutputs]{
			ID:     userID,
			Inputs: proxmox.UserInputs{Name: userID, Comment: userComment},
			State:  proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "UserOperations not configured")
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		user := &userResource.User{}
		_, err := user.Delete(context.Background(), infer.DeleteRequest[proxmox.UserOutputs]{
			State: proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "UserOperations not configured")
	})
}

func TestUserCreateSuccess(t *testing.T) {
	t.Parallel()

	called := false
	user := &userResource.User{
		UserOps: &mockUserOperations{createFunc: func(ctx context.Context, inputs proxmox.UserInputs) error {
			called = true
			assert.Equal(t, proxmox.UserInputs{Name: userID, Email: userEmail}, inputs)
			return nil
		}},
	}

	resp, err := user.Create(context.Background(), infer.CreateRequest[proxmox.UserInputs]{
		Name: userID,
		Inputs: proxmox.UserInputs{
			Name:  userID,
			Email: userEmail,
		},
	})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, userID, resp.ID)
	assert.Equal(t, userEmail, resp.Output.Email)
}

func TestUserCreateDryRunDoesNotCallAdapter(t *testing.T) {
	t.Parallel()

	called := false
	user := &userResource.User{
		UserOps: &mockUserOperations{createFunc: func(ctx context.Context, inputs proxmox.UserInputs) error {
			called = true
			return nil
		}},
	}

	resp, err := user.Create(context.Background(), infer.CreateRequest[proxmox.UserInputs]{
		Name:   userID,
		DryRun: true,
		Inputs: proxmox.UserInputs{Name: userID},
	})
	require.NoError(t, err)
	assert.False(t, called)
	assert.Equal(t, userID, resp.ID)
}

func TestUserCreateAdapterError(t *testing.T) {
	t.Parallel()

	user := &userResource.User{
		UserOps: &mockUserOperations{createFunc: func(ctx context.Context, inputs proxmox.UserInputs) error {
			return assert.AnError
		}},
	}

	_, err := user.Create(context.Background(), infer.CreateRequest[proxmox.UserInputs]{
		Inputs: proxmox.UserInputs{Name: userID},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

// Test does not set global environment variable, therefore can be parallelized!
func TestUserReadIDNotFound(t *testing.T) {
	t.Parallel()

	called := false
	user := &userResource.User{
		UserOps: &mockUserOperations{getFunc: func(ctx context.Context, id string) (*proxmox.UserOutputs, error) {
			called = true
			return &proxmox.UserOutputs{}, nil
		}},
	}

	resp, err := user.Read(context.Background(), infer.ReadRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID:     "",
		Inputs: proxmox.UserInputs{Name: userID},
		State:  proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
	})
	require.NoError(t, err)
	assert.Equal(t, "", resp.ID)
	assert.False(t, called)
}

func TestUserReadNotFoundIsTreatedAsDeleted(t *testing.T) {
	t.Parallel()

	user := &userResource.User{
		UserOps: &mockUserOperations{getFunc: func(ctx context.Context, id string) (*proxmox.UserOutputs, error) {
			return nil, &notFoundError{msg: "user '" + userID + "' does not exist\n"}
		}},
	}

	resp, err := user.Read(context.Background(), infer.ReadRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID:     userID,
		Inputs: proxmox.UserInputs{Name: userID},
		State:  proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.ID)
}

func TestUserReadSuccess(t *testing.T) {
	t.Parallel()

	user := &userResource.User{
		UserOps: &mockUserOperations{getFunc: func(ctx context.Context, id string) (*proxmox.UserOutputs, error) {
			assert.Equal(t, userID, id)
			return &proxmox.UserOutputs{UserInputs: proxmox.UserInputs{
				Name:      userID,
				Comment:   userComment,
				Email:     userEmail,
				Enable:    true,
				Expire:    42,
				Firstname: userFirstname,
				Lastname:  userLastname,
				Groups:    []string{userGroup2, userGroup1},
				Keys:      []string{userKey2, userKey1},
			}}, nil
		}},
	}

	resp, err := user.Read(context.Background(), infer.ReadRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID: userID,
		Inputs: proxmox.UserInputs{
			Name:     userID,
			Password: userPassword,
		},
		State: proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
	})
	require.NoError(t, err)
	assert.Equal(t, userID, resp.ID)
	assert.Equal(t, userComment, resp.State.Comment)
	assert.Equal(t, userPassword, resp.State.Password)
}

func TestUserReadAdapterError(t *testing.T) {
	t.Parallel()

	user := &userResource.User{
		UserOps: &mockUserOperations{getFunc: func(ctx context.Context, id string) (*proxmox.UserOutputs, error) {
			return nil, assert.AnError
		}},
	}

	_, err := user.Read(context.Background(), infer.ReadRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID:     userID,
		Inputs: proxmox.UserInputs{Name: userID},
		State:  proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestUserUpdateAdapterError(t *testing.T) {
	t.Parallel()

	user := &userResource.User{
		UserOps: &mockUserOperations{updateFunc: func(ctx context.Context, id string, inputs proxmox.UserInputs) error {
			return assert.AnError
		}},
	}

	_, err := user.Update(context.Background(), infer.UpdateRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		Inputs: proxmox.UserInputs{Name: userID, Comment: "comment"},
		State:  proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID, Comment: "comment"}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestUserUpdateSuccess(t *testing.T) {
	t.Parallel()

	var updatedID string
	var updatedInputs proxmox.UserInputs

	user := &userResource.User{
		UserOps: &mockUserOperations{updateFunc: func(ctx context.Context, id string, inputs proxmox.UserInputs) error {
			updatedID = id
			updatedInputs = inputs
			return nil
		}},
	}

	state := proxmox.UserOutputs{UserInputs: proxmox.UserInputs{
		Name:      userID,
		Comment:   "old",
		Email:     userEmail,
		Enable:    true,
		Expire:    10,
		Firstname: userFirstname,
		Lastname:  userLastname,
		Groups:    []string{userGroup1, userGroup2},
		Keys:      []string{userKey1, userKey2},
	}}

	resp, err := user.Update(context.Background(), infer.UpdateRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID: userID,
		Inputs: proxmox.UserInputs{
			Name:    userID,
			Comment: userComment,
			// same sets, different order => should not update these fields
			Groups: []string{userGroup2, userGroup1},
			Keys:   []string{userKey2, userKey1},
		},
		State: state,
	})
	require.NoError(t, err)

	assert.Equal(t, userID, updatedID)
	assert.Equal(t, userComment, updatedInputs.Comment)
	assert.Equal(t, []string{userGroup1, userGroup2}, updatedInputs.Groups)
	assert.Equal(t, []string{userKey1, userKey2}, updatedInputs.Keys)
	assert.Equal(t, userComment, resp.Output.Comment)
}

func TestUserUpdateDryRunDoesNotCallAdapter(t *testing.T) {
	t.Parallel()

	called := false
	user := &userResource.User{
		UserOps: &mockUserOperations{updateFunc: func(ctx context.Context, id string, inputs proxmox.UserInputs) error {
			called = true
			return nil
		}},
	}

	_, err := user.Update(context.Background(), infer.UpdateRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID:     userID,
		DryRun: true,
		Inputs: proxmox.UserInputs{Name: userID, Comment: userComment},
		State:  proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID, Comment: "old"}},
	})
	require.NoError(t, err)
	assert.False(t, called)
}

func TestUserDeleteAdapterError(t *testing.T) {
	t.Parallel()

	user := &userResource.User{UserOps: &mockUserOperations{deleteFunc: func(ctx context.Context, id string) error {
		return assert.AnError
	}}}

	_, err := user.Delete(context.Background(), infer.DeleteRequest[proxmox.UserOutputs]{
		State: proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestUserDeleteSuccess(t *testing.T) {
	t.Parallel()

	var deleted string
	user := &userResource.User{UserOps: &mockUserOperations{deleteFunc: func(ctx context.Context, id string) error {
		deleted = id
		return nil
	}}}

	_, err := user.Delete(context.Background(), infer.DeleteRequest[proxmox.UserOutputs]{
		State: proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
	})
	require.NoError(t, err)
	assert.Equal(t, userID, deleted)
}

func TestUserDiffReplaceOnChange(t *testing.T) {
	t.Parallel()

	user := &userResource.User{}
	resp, err := user.Diff(context.Background(), infer.DiffRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID: userID,
		Inputs: proxmox.UserInputs{
			Name:     "newuser",
			Password: "newpass",
		},
		State: proxmox.UserOutputs{UserInputs: proxmox.UserInputs{
			Name:     userID,
			Password: userPassword,
		}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["userid"].Kind)
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["password"].Kind)
}

func TestUserDiffTreatsListsAsSets(t *testing.T) {
	t.Parallel()

	user := &userResource.User{}
	resp, err := user.Diff(context.Background(), infer.DiffRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID: userID,
		Inputs: proxmox.UserInputs{
			Name:   userID,
			Groups: []string{userGroup2, userGroup1},
			Keys:   []string{userKey2, userKey1},
		},
		State: proxmox.UserOutputs{UserInputs: proxmox.UserInputs{
			Name:   userID,
			Groups: []string{userGroup1, userGroup2},
			Keys:   []string{userKey1, userKey2},
		}},
	})
	require.NoError(t, err)
	assert.False(t, resp.HasChanges)
}
