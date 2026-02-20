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

func TestGroupHealthyLifeCycle(t *testing.T) {
	t.Parallel()

	var (
		created proxmox.GroupInputs
		updated proxmox.GroupInputs
		deleted string
	)

	ops := &mockGroupOperations{
		createFunc: func(ctx context.Context, inputs proxmox.GroupInputs) error {
			created = inputs
			return nil
		},
		getFunc: func(ctx context.Context, id string) (*proxmox.GroupOutputs, error) {
			require.Equal(t, groupID, id)
			return &proxmox.GroupOutputs{GroupInputs: updated}, nil
		},
		updateFunc: func(ctx context.Context, id string, inputs proxmox.GroupInputs) error {
			require.Equal(t, groupID, id)
			updated = inputs
			return nil
		},
		deleteFunc: func(ctx context.Context, id string) error {
			deleted = id
			return nil
		},
	}

	group := &groupResource.Group{GroupOps: ops}

	createResp, err := group.Create(context.Background(), infer.CreateRequest[proxmox.GroupInputs]{
		Name: groupID,
		Inputs: proxmox.GroupInputs{
			Name:    groupID,
			Comment: groupComment,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, groupID, createResp.ID)
	assert.Equal(t, proxmox.GroupInputs{Name: groupID, Comment: groupComment}, created)

	updateResp, err := group.Update(context.Background(), infer.UpdateRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
		ID: groupID,
		Inputs: proxmox.GroupInputs{
			Name:    groupID,
			Comment: updatedGroupComment,
		},
		State: proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{
			Name:    groupID,
			Comment: groupComment,
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, updatedGroupComment, updateResp.Output.Comment)
	assert.Equal(t, proxmox.GroupInputs{Name: groupID, Comment: updatedGroupComment}, updated)

	readResp, err := group.Read(context.Background(), infer.ReadRequest[proxmox.GroupInputs, proxmox.GroupOutputs]{
		ID: groupID,
		Inputs: proxmox.GroupInputs{
			Name: groupID,
		},
		State: proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
	})
	require.NoError(t, err)
	assert.Equal(t, groupID, readResp.State.Name)
	assert.Equal(t, updatedGroupComment, readResp.State.Comment)

	_, err = group.Delete(context.Background(), infer.DeleteRequest[proxmox.GroupOutputs]{
		State: proxmox.GroupOutputs{GroupInputs: proxmox.GroupInputs{Name: groupID}},
	})
	require.NoError(t, err)
	assert.Equal(t, groupID, deleted)
}
