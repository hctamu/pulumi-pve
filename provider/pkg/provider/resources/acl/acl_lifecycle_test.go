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
	"testing"

	aclResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/acl"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// aclHealthyLifeCycleHelper validates create/read/delete using a mocked adapter.
func aclLHealthyLifeCycleHelper(t *testing.T, typ, ugid string) {
	createdCalled := false
	deletedCalled := false

	inputs := proxmox.ACLInputs{
		Path:      pathRoot,
		RoleID:    roleAdmin,
		Type:      typ,
		UGID:      ugid,
		Propagate: true,
	}
	id := pathRoot + "|" + roleAdmin + "|" + typ + "|" + ugid

	acl := &aclResource.ACL{ACLOps: &mockACLOperations{
		createFunc: func(ctx context.Context, in proxmox.ACLInputs) error {
			createdCalled = true
			assert.Equal(t, inputs, in)
			return nil
		},
		getFunc: func(ctx context.Context, gotID string) (*proxmox.ACLOutputs, error) {
			assert.Equal(t, id, gotID)
			return &proxmox.ACLOutputs{ACLInputs: inputs}, nil
		},
		deleteFunc: func(ctx context.Context, out proxmox.ACLOutputs) error {
			deletedCalled = true
			assert.Equal(t, inputs, out.ACLInputs)
			return nil
		},
	}}

	createResp, err := acl.Create(context.Background(), infer.CreateRequest[proxmox.ACLInputs]{
		Name:   id,
		Inputs: inputs,
	})
	require.NoError(t, err)
	assert.Equal(t, id, createResp.ID)
	assert.Equal(t, typ, createResp.Output.Type)
	assert.Equal(t, ugid, createResp.Output.UGID)

	readResp, err := acl.Read(context.Background(), infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID:     id,
		Inputs: inputs,
	})
	require.NoError(t, err)
	assert.Equal(t, id, readResp.ID)
	assert.Equal(t, pathRoot, readResp.State.Path)
	assert.Equal(t, roleAdmin, readResp.State.RoleID)
	assert.Equal(t, typ, readResp.State.Type)
	assert.Equal(t, ugid, readResp.State.UGID)
	assert.Equal(t, true, readResp.State.Propagate)

	_, err = acl.Delete(context.Background(), infer.DeleteRequest[proxmox.ACLOutputs]{
		ID:    id,
		State: readResp.State,
	})
	require.NoError(t, err)
	assert.True(t, createdCalled)
	assert.True(t, deletedCalled)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLHealthyLifeCycleGroup(t *testing.T) {
	aclLHealthyLifeCycleHelper(
		t,
		typeGroup,
		ugidGroup1,
	)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLHealthyLifeCycleUser(t *testing.T) {
	aclLHealthyLifeCycleHelper(
		t,
		typeUser,
		ugidUser1,
	)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLHealthyLifeCycleToken(t *testing.T) {
	aclLHealthyLifeCycleHelper(
		t,
		typeToken,
		ugidToken1,
	)
}
