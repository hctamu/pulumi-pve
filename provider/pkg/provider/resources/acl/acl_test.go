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

package acl_test

import (
	"context"
	"errors"
	"testing"

	aclResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/acl"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Common test constants
const (
	pathRoot  = "/"
	roleAdmin = "PVEAdmin"

	typeUser  = "user"
	typeGroup = "group"
	typeToken = "token"

	ugidUser1  = "testuser"
	ugidGroup1 = "testgroup"
	ugidToken1 = "svc!apitoken"

	idUser  = pathRoot + "|" + roleAdmin + "|" + typeUser + "|" + ugidUser1
	idGroup = pathRoot + "|" + roleAdmin + "|" + typeGroup + "|" + ugidGroup1
	idToken = pathRoot + "|" + roleAdmin + "|" + typeToken + "|" + ugidToken1
)

type mockACLOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.ACLInputs) error
	getFunc    func(ctx context.Context, id string) (*proxmox.ACLOutputs, error)
	updateFunc func(ctx context.Context, id string, inputs proxmox.ACLInputs) error
	deleteFunc func(ctx context.Context, outputs proxmox.ACLOutputs) error
}

func (m *mockACLOperations) Create(ctx context.Context, inputs proxmox.ACLInputs) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, inputs)
	}
	return nil
}

func (m *mockACLOperations) Get(ctx context.Context, id string) (*proxmox.ACLOutputs, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return &proxmox.ACLOutputs{ACLInputs: proxmox.ACLInputs{}}, nil
}

func (m *mockACLOperations) Update(ctx context.Context, id string, inputs proxmox.ACLInputs) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, id, inputs)
	}
	return nil
}

func (m *mockACLOperations) Delete(ctx context.Context, outputs proxmox.ACLOutputs) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, outputs)
	}
	return nil
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateGroupNotFound(t *testing.T) {
	aclCreateTypeNotFoundHelper(t, typeGroup)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateUserNotFound(t *testing.T) {
	aclCreateTypeNotFoundHelper(t, typeUser)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateInvalidType(t *testing.T) {
	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{createFunc: func(ctx context.Context, inputs proxmox.ACLInputs) error {
			return errors.New("invalid type (must be 'user', 'group', or 'token')")
		}},
	}
	request := infer.CreateRequest[proxmox.ACLInputs]{
		Name: pathRoot + "|" + roleAdmin + "|invalid|testinvalid",
		Inputs: proxmox.ACLInputs{
			Path:      pathRoot,
			RoleID:    roleAdmin,
			Type:      "invalid",
			UGID:      "testinvalid",
			Propagate: true,
		},
	}
	_, err := acl.Create(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "invalid type (must be 'user', 'group', or 'token')")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateCreationError(t *testing.T) {
	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{createFunc: func(ctx context.Context, inputs proxmox.ACLInputs) error {
			return errors.New("failed to create ACL resource: 500 Internal Server Error")
		}},
	}
	request := infer.CreateRequest[proxmox.ACLInputs]{
		Name: idUser,
		Inputs: proxmox.ACLInputs{
			Path:   pathRoot,
			RoleID: roleAdmin,
			Type:   typeUser,
			UGID:   ugidUser1,
		},
	}
	_, err := acl.Create(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to create ACL resource: 500 Internal Server Error")
}

// Test does not set global environment variable, therefore can be parallelized!
func TestACLDeleteInvalidType(t *testing.T) {
	t.Parallel()

	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{deleteFunc: func(ctx context.Context, outputs proxmox.ACLOutputs) error {
			return errors.New("invalid type (must be 'user', 'group', or 'token')")
		}},
	}
	request := infer.DeleteRequest[proxmox.ACLOutputs]{
		ID: pathRoot + "|" + roleAdmin + "|invalid|testinvalid",
		State: proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{
				Path:      pathRoot,
				RoleID:    roleAdmin,
				Type:      "invalid",
				UGID:      "testinvalid",
				Propagate: true,
			},
		},
	}
	_, err := acl.Delete(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "invalid type (must be 'user', 'group', or 'token')")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLDeleteClientError(t *testing.T) {
	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{deleteFunc: func(ctx context.Context, outputs proxmox.ACLOutputs) error {
			return errors.New("client error")
		}},
	}
	request := infer.DeleteRequest[proxmox.ACLOutputs]{
		ID: idUser,
		State: proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{
				Path:      pathRoot,
				RoleID:    roleAdmin,
				Type:      typeUser,
				UGID:      ugidUser1,
				Propagate: true,
			},
		},
	}
	_, err := acl.Delete(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

// aclCreateTypeNotFoundHelper is used to test Create failure when the specified type entity is not found.
func aclCreateTypeNotFoundHelper(t *testing.T, typ string) {
	res := &aclResource.ACL{
		ACLOps: &mockACLOperations{createFunc: func(ctx context.Context, inputs proxmox.ACLInputs) error {
			return errors.New("failed to find " + inputs.Type + " " + inputs.UGID + " for ACL: 500 Internal Server Error")
		}},
	}
	response, err := res.Create(context.Background(), infer.CreateRequest[proxmox.ACLInputs]{
		Name: "test" + typ,
		Inputs: proxmox.ACLInputs{
			Path:      pathRoot,
			RoleID:    roleAdmin,
			Type:      typ,
			UGID:      "test" + typ,
			Propagate: true,
		},
	})
	assert.Error(t, err)
	assert.EqualError(t, err, "failed to find "+typ+" "+response.Output.UGID+" for ACL: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadSuccess(t *testing.T) {
	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{getFunc: func(ctx context.Context, id string) (*proxmox.ACLOutputs, error) {
			return &proxmox.ACLOutputs{ACLInputs: proxmox.ACLInputs{
				Path:      pathRoot,
				RoleID:    roleAdmin,
				Type:      typeGroup,
				UGID:      ugidGroup1,
				Propagate: false,
			}}, nil
		}},
	}
	id := idGroup
	response, err := acl.Read(context.Background(), infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID: id,
		Inputs: proxmox.ACLInputs{
			Path: pathRoot, RoleID: roleAdmin, Type: typeGroup, UGID: ugidGroup1, Propagate: false,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, response.ID, id)
	assert.Equal(t, response.State.Path, pathRoot)
	assert.Equal(t, response.State.RoleID, roleAdmin)
	assert.Equal(t, response.State.UGID, ugidGroup1)
	assert.Equal(t, response.State.Propagate, false)
}

// Test does not set global environment variable, therefore can be parallelized!
func TestACLReadIDNotFound(t *testing.T) {
	t.Parallel()

	acl := &aclResource.ACL{ACLOps: &mockACLOperations{}}
	request := infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID:     "",
		Inputs: proxmox.ACLInputs{Path: pathRoot, RoleID: roleAdmin, Type: typeGroup, UGID: ugidGroup1, Propagate: false},
		State: proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{
				Path:      pathRoot,
				RoleID:    roleAdmin,
				Type:      typeGroup,
				UGID:      ugidGroup1,
				Propagate: false,
			},
		},
	}
	_, err := acl.Read(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "missing acl ID")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadClientError(t *testing.T) {
	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{getFunc: func(ctx context.Context, id string) (*proxmox.ACLOutputs, error) {
			return nil, errors.New("client error")
		}},
	}
	_, err := acl.Read(context.Background(), infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID: idGroup,
		Inputs: proxmox.ACLInputs{ // simulate previous inputs
			Path: pathRoot, RoleID: roleAdmin, Type: typeGroup, UGID: ugidGroup1, Propagate: false,
		},
	})
	assert.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadACLIDError(t *testing.T) {
	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{getFunc: func(ctx context.Context, id string) (*proxmox.ACLOutputs, error) {
			return nil, errors.New("invalid ACL ID: " + id)
		}},
	}
	_, err := acl.Read(context.Background(), infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID: "invalid-acl-id-format",
	})
	assert.Error(t, err)
	assert.EqualError(t, err, "invalid ACL ID: invalid-acl-id-format")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadNotFound(t *testing.T) {
	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{getFunc: func(ctx context.Context, id string) (*proxmox.ACLOutputs, error) {
			return nil, errors.New("acl not found")
		}},
	}
	request := infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID: idUser,
	}
	_, err := acl.Read(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "acl not found")
}
