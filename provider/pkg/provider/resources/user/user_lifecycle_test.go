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
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestUserHealthyLifeCycle(t *testing.T) {
	t.Parallel()

	var (
		created proxmox.UserInputs
		updated proxmox.UserInputs
		deleted string
	)

	ops := &mockUserOperations{
		createFunc: func(ctx context.Context, inputs proxmox.UserInputs) error {
			created = inputs
			return nil
		},
		getFunc: func(ctx context.Context, id string) (*proxmox.UserOutputs, error) {
			return &proxmox.UserOutputs{UserInputs: updated}, nil
		},
		updateFunc: func(ctx context.Context, id string, inputs proxmox.UserInputs) error {
			assert.Equal(t, userID, id)
			updated = inputs
			return nil
		},
		deleteFunc: func(ctx context.Context, id string) error {
			deleted = id
			return nil
		},
	}

	user := &userResource.User{UserOps: ops}

	createResp, err := user.Create(context.Background(), infer.CreateRequest[proxmox.UserInputs]{
		Name: userID,
		Inputs: proxmox.UserInputs{
			Name: userID,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, userID, createResp.ID)
	assert.Equal(t, proxmox.UserInputs{Name: userID}, created)

	updateResp, err := user.Update(context.Background(), infer.UpdateRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID: userID,
		Inputs: proxmox.UserInputs{
			Name:      userID,
			Comment:   userComment,
			Enable:    true,
			Expire:    42,
			Firstname: userFirstname,
			Lastname:  userLastname,
			Email:     userEmail,
			Groups:    []string{"g2", "g1"},
			Keys:      []string{"ssh-rsa", "ssh-ed25519"},
		},
		State: proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
	})
	require.NoError(t, err)
	assert.Equal(t, userComment, updateResp.Output.Comment)
	assert.Equal(t, true, updateResp.Output.Enable)
	assert.Equal(t, 42, updateResp.Output.Expire)
	assert.Equal(t, userFirstname, updateResp.Output.Firstname)
	assert.Equal(t, userLastname, updateResp.Output.Lastname)
	assert.Equal(t, userEmail, updateResp.Output.Email)
	assert.Equal(t, utils.SliceToString([]string{"g1", "g2"}), utils.SliceToString(updateResp.Output.Groups))
	assert.Equal(t, utils.SliceToString([]string{"ssh-ed25519", "ssh-rsa"}), utils.SliceToString(updateResp.Output.Keys))

	readResp, err := user.Read(context.Background(), infer.ReadRequest[proxmox.UserInputs, proxmox.UserOutputs]{
		ID: userID,
		Inputs: proxmox.UserInputs{
			Name:     userID,
			Password: userPassword,
		},
		State: proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
	})
	require.NoError(t, err)
	assert.Equal(t, userID, readResp.State.Name)
	assert.Equal(t, userPassword, readResp.State.Password)

	_, err = user.Delete(context.Background(), infer.DeleteRequest[proxmox.UserOutputs]{
		State: proxmox.UserOutputs{UserInputs: proxmox.UserInputs{Name: userID}},
	})
	require.NoError(t, err)
	assert.Equal(t, userID, deleted)
}
