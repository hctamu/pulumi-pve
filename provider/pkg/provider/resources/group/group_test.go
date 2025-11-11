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
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	utils "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources"
	groupResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/group"
	"github.com/hctamu/pulumi-pve/provider/px"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Implement a custom PostAction
type toggleMocksPostAction struct {
	toDisable []*mocha.Scoped
	toEnable  []*mocha.Scoped
}

func (a *toggleMocksPostAction) Run(args mocha.PostActionArgs) error {
	for _, m := range a.toDisable {
		m.Disable()
	}

	for _, m := range a.toEnable {
		m.Enable()
	}

	return nil
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupHealthyLifeCycle(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// fetch created resource to confirm
	getGroup := mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/groups/testgroup")).
			Reply(reply.OK().BodyString(`{"data": {
				"comment": "comment",
				"members": []
			}}`)),
	)

	// perform create
	createGroup := mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/groups")).Reply(reply.Created().BodyString(`{
            "data": {
                "groupid": "testgroup",
                "name": "testgroup",
                "comment": "comment"
            }
        }`)).PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{getGroup}}),
	)

	// perform delete
	deleteGroup := mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/groups/testgroup")).Reply(
			reply.OK()))

	// perform update
	updateGroup := mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/groups/testgroup")).Reply(reply.OK().BodyString(`{
	            "data": {
	                "name": "testgroup",
	                "comment": "updated comment",
	  }
	    }`)))

	// Enable initial state
	getGroup.Enable()
	createGroup.Enable()
	deleteGroup.Enable()
	updateGroup.Enable()

	// env + client configured by helper

	// Start the integration server with the mock setup
	server, err := integration.NewServer(
		t.Context(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProvider()),
	)
	require.NoError(t, err)

	updatedResource := property.NewMap(map[string]property.Value{
		"name":    property.New("testgroup"),
		"comment": property.New("updated comment"),
	})

	// Run the integration lifecycle tests
	integration.LifeCycleTest{
		Resource: "pve:group:Group",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New("testgroup"),
				"comment": property.New("comment"),
			}),
			Hook: func(inputs, output property.Map) {
				t.Logf("Outputs after Create: %v", output)
				assert.Equal(t, "testgroup", output.Get("name").AsString())
				assert.Equal(t, "comment", output.Get("comment").AsString())
			},
		},
		Updates: []integration.Operation{
			{
				Inputs: property.NewMap(map[string]property.Value{
					"name":    property.New("testgroup"),
					"comment": property.New("updated comment"),
				}),
				ExpectedOutput: &updatedResource,
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupCreateClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	group := &groupResource.Group{}
	request := infer.CreateRequest[groupResource.Inputs]{
		Inputs: groupResource.Inputs{Name: "testgroup", Comment: "comment"},
	}
	_, err := group.Create(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupCreateCreationError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/groups")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured by helper

	group := &groupResource.Group{}
	request := infer.CreateRequest[groupResource.Inputs]{
		Inputs: groupResource.Inputs{Name: "testgroup", Comment: "comment"},
	}
	_, err := group.Create(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to create group testgroup: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupCreateFetchError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/groups")).Reply(reply.Created().BodyString(`{
			"data": {
			"groupid": "testgroup",
			"comment": "comment"}
		}`)),
		mocha.Get(expect.URLPath("/access/groups/testgroup")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured by helper

	group := &groupResource.Group{}
	request := infer.CreateRequest[groupResource.Inputs]{
		Inputs: groupResource.Inputs{Name: "testgroup", Comment: "comment"},
	}
	_, err := group.Create(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to fetch group testgroup: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupDeleteClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	group := &groupResource.Group{}
	request := infer.DeleteRequest[groupResource.Outputs]{
		State: groupResource.Outputs{Inputs: groupResource.Inputs{Name: "testgroup"}},
	}
	_, err := group.Delete(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupDeleteDeletionError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/groups/testgroup")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured by helper

	group := &groupResource.Group{}
	request := infer.DeleteRequest[groupResource.Outputs]{
		State: groupResource.Outputs{Inputs: groupResource.Inputs{Name: "testgroup"}},
	}
	_, err := group.Delete(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete group")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupReadSuccess(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/groups/testgroup")).Reply(reply.OK().BodyString(`{
			"data": {"comment":"comment","members":[]}
		}`)),
	).Enable()

	// env + client configured

	group := &groupResource.Group{}
	request := infer.ReadRequest[groupResource.Inputs, groupResource.Outputs]{
		ID:     "testgroup",
		Inputs: groupResource.Inputs{Name: "testgroup"},
		State:  groupResource.Outputs{Inputs: groupResource.Inputs{Name: "testgroup", Comment: "old comment"}},
	}

	response, err := group.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "testgroup", response.State.Name)
	assert.Equal(t, "comment", response.State.Comment)
}

// Test does not set global environment variable, therefore can be parallelized!
func TestGroupReadIDNotFound(t *testing.T) {
	t.Parallel()
	group := &groupResource.Group{}
	request := infer.ReadRequest[groupResource.Inputs, groupResource.Outputs]{
		ID:     "",
		Inputs: groupResource.Inputs{Name: "testgroup"},
	}
	response, err := group.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, response.ID, "")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupReadClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	group := &groupResource.Group{}
	request := infer.ReadRequest[groupResource.Inputs, groupResource.Outputs]{
		ID:     "testgroup",
		Inputs: groupResource.Inputs{Name: "testgroup"},
	}
	_, err := group.Read(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupReadNotFound(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/groups/testgroup")).
			Reply(reply.BadRequest().BodyString(`{
			"data":null,
			"message":"group 'testgroup' does not exist\n"
		}`)),
	).Enable()
	// env + client configured

	group := &groupResource.Group{}
	request := infer.ReadRequest[groupResource.Inputs, groupResource.Outputs]{
		ID: "testgroup",
	}
	response, err := group.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Empty(t, response.ID)
	assert.Empty(t, response.State)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupReadGetResourceError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/groups/testgroup")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	group := &groupResource.Group{}
	request := infer.ReadRequest[groupResource.Inputs, groupResource.Outputs]{
		ID:     "testgroup",
		Inputs: groupResource.Inputs{Name: "testgroup"},
	}
	_, err := group.Read(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to get group testgroup: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupUpdateClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	group := &groupResource.Group{}
	request := infer.UpdateRequest[groupResource.Inputs, groupResource.Outputs]{
		State:  groupResource.Outputs{Inputs: groupResource.Inputs{Name: "testgroup", Comment: "old comment"}},
		Inputs: groupResource.Inputs{Name: "testgroup", Comment: "new comment"},
	}
	_, err := group.Update(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestGroupUpdateUpdateError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/groups/testgroup")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured by helper

	group := &groupResource.Group{}
	request := infer.UpdateRequest[groupResource.Inputs, groupResource.Outputs]{
		State:  groupResource.Outputs{Inputs: groupResource.Inputs{Name: "testgroup", Comment: "old comment"}},
		Inputs: groupResource.Inputs{Name: "testgroup", Comment: "new comment"},
	}

	_, err := group.Update(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to update group testgroup: 500 Internal Server Error")
}
