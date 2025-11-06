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
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	utils "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources"
	userResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/user"
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

// toggleMocksPostAction replicates helper used in other tests to enable GET mocks after create.
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

//nolint:paralleltest // mutates global env + seam
func TestUserHealthyLifeCycle(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// fetch created resource to confirm
	getUser := mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).
			Reply(reply.OK().BodyString(`{
				"data": {}
			}`)),
	)

	// perform create
	createUser := mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/users")).
			Reply(reply.Created().BodyString(`{"data":{"userid":"testuser"}}`)).
			PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{getUser}}),
	)

	// perform update
	updateUser := mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/users/testuser")).
			Reply(reply.OK().BodyString(`{"data":{
					"comment":"updated comment",
					"enable":1,
					"expire":42,
					"firstname":"Bob",
					"lastname":"Balancer",
					"email":"bob@example.com",
					"groups":["g1","g2"],
					"keys":"ssh-ed25519,ssh-rsa"}}`)),
	)

	// perform delete
	deleteUser := mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/users/testuser")).Reply(reply.OK()),
	)

	// Enable mocks
	getUser.Enable()
	createUser.Enable()
	updateUser.Enable()
	deleteUser.Enable()

	// Start provider integration server
	server, err := integration.NewServer(
		t.Context(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProvider()),
	)
	require.NoError(t, err)

	updatedResource := property.NewMap(map[string]property.Value{
		"userid":    property.New("testuser"),
		"comment":   property.New("updated comment"),
		"enable":    property.New(true),
		"expire":    property.New(42.0),
		"firstname": property.New("Bob"),
		"lastname":  property.New("Balancer"),
		"email":     property.New("bob@example.com"),
		"groups": property.New(property.NewArray([]property.Value{
			property.New("g1"),
			property.New("g2"),
		})),
		"keys": property.New(property.NewArray([]property.Value{
			property.New("ssh-ed25519"),
			property.New("ssh-rsa"),
		})),
		"password": property.New(""),
	})

	integration.LifeCycleTest{
		Resource: "pve:user:User",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"userid": property.New("testuser"),
			}),
			Hook: func(in, out property.Map) {
				assert.Equal(t, "testuser", out.Get("userid").AsString())
			},
		},
		Updates: []integration.Operation{
			{
				Inputs: property.NewMap(map[string]property.Value{
					"userid":    property.New("testuser"),
					"comment":   property.New("updated comment"),
					"enable":    property.New(true),
					"expire":    property.New(42.0),
					"firstname": property.New("Bob"),
					"lastname":  property.New("Balancer"),
					"email":     property.New("bob@example.com"),
					"groups": property.New(property.NewArray([]property.Value{
						property.New("g1"),
						property.New("g2"),
					})),
					"keys": property.New(property.NewArray([]property.Value{
						property.New("ssh-ed25519"),
						property.New("ssh-rsa"),
					})),
				}),
				ExpectedOutput: &updatedResource,
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserCreateClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	user := &userResource.User{}
	request := infer.CreateRequest[userResource.Inputs]{
		Name: "testuser",
		Inputs: userResource.Inputs{
			Name: "testuser",
		},
	}
	_, err := user.Create(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserCreateCreationError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/users")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	user := &userResource.User{}
	request := infer.CreateRequest[userResource.Inputs]{
		Name: "testuser",
		Inputs: userResource.Inputs{
			Name: "testuser",
		},
	}
	_, err := user.Create(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create user")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserCreateFetchError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/users")).
			Reply(reply.Created().BodyString(`{
			"data": {
				"userid": "testuser"	
		}`)),
		mocha.Get(expect.URLPath("/access/users/testuser")).
			Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	user := &userResource.User{}
	request := infer.CreateRequest[userResource.Inputs]{
		Name: "testuser",
		Inputs: userResource.Inputs{
			Name:  "testuser",
			Email: "testuser@example.com",
		},
	}
	_, err := user.Create(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch user")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserDeleteClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	user := &userResource.User{}
	request := infer.DeleteRequest[userResource.Outputs]{
		ID: "testuser",
	}
	_, err := user.Delete(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserDeleteDeletionError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/users/testuser")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	user := &userResource.User{}
	request := infer.DeleteRequest[userResource.Outputs]{
		State: userResource.Outputs{Inputs: userResource.Inputs{Name: "testuser"}},
	}
	_, err := user.Delete(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete user")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserReadSuccess(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).Reply(reply.OK().BodyString(`{
            "data":{
			"userid":"testuser",
			"comment":"c",
			"enable":1,
			"expire":10,
            "firstname":"F",
			"lastname":"L",
			"email":"f@example.com",
			"groups":["g1","g2"],
			"keys":"ssh-rsa AAA,ssh-ed25519 BBB"}}
        `)),
	).Enable()

	user := &userResource.User{}
	request := infer.ReadRequest[userResource.Inputs, userResource.Outputs]{
		ID:     "testuser",
		Inputs: userResource.Inputs{Name: "testuser"},
		State:  userResource.Outputs{Inputs: userResource.Inputs{Name: "testuser"}},
	}
	resp, err := user.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "testuser", resp.State.Name)
	assert.Equal(t, []string{"g1", "g2"}, resp.State.Groups)
	assert.Equal(t, []string{"ssh-ed25519 BBB", "ssh-rsa AAA"}, resp.State.Keys) // sorted
	assert.Equal(t, "F", resp.State.Firstname)
	assert.Equal(t, 10, resp.State.Expire)
}

// Test does not set global environment variable, therefore can be parallelized!
func TestUserReadIDNotFound(t *testing.T) {
	user := &userResource.User{}
	request := infer.ReadRequest[userResource.Inputs, userResource.Outputs]{
		ID:     "",
		Inputs: userResource.Inputs{Name: "testuser"},
		State:  userResource.Outputs{Inputs: userResource.Inputs{Name: "testuser"}},
	}
	response, err := user.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, response.ID, "")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserReadClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	user := &userResource.User{}
	request := infer.ReadRequest[userResource.Inputs, userResource.Outputs]{
		ID:     "testuser",
		Inputs: userResource.Inputs{Name: "testuser"},
		State:  userResource.Outputs{Inputs: userResource.Inputs{Name: "testuser"}},
	}
	_, err := user.Read(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserReadNotFound(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).
			Reply(reply.BadRequest().BodyString(`{
			"data":null,
			"message":"user 'testuser' does not exist\n"
		}`)),
	).Enable()
	// env + client configured

	user := &userResource.User{}
	request := infer.ReadRequest[userResource.Inputs, userResource.Outputs]{
		ID: "testuser",
	}
	response, err := user.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Empty(t, response.ID)
	assert.Empty(t, response.State)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserReadGetResourceError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).
			Reply(reply.InternalServerError()),
	).Enable()
	// env + client configured

	user := &userResource.User{}
	request := infer.ReadRequest[userResource.Inputs, userResource.Outputs]{
		ID:     "testuser",
		Inputs: userResource.Inputs{Name: "testuser"},
		State:  userResource.Outputs{Inputs: userResource.Inputs{Name: "testuser"}},
	}
	_, err := user.Read(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get user")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserUpdateClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	user := &userResource.User{}
	request := infer.UpdateRequest[userResource.Inputs, userResource.Outputs]{
		Inputs: userResource.Inputs{
			Name:    "testuser",
			Comment: "comment",
		},
		State: userResource.Outputs{
			Inputs: userResource.Inputs{
				Name:    "testuser",
				Comment: "comment",
			},
		},
	}
	_, err := user.Update(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestUserUpdateUpdateError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/users/testuser")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	user := &userResource.User{}
	request := infer.UpdateRequest[userResource.Inputs, userResource.Outputs]{
		Inputs: userResource.Inputs{
			Name:    "testuser",
			Comment: "comment",
		},
		State: userResource.Outputs{
			Inputs: userResource.Inputs{
				Name:    "testuser",
				Comment: "comment",
			},
		},
	}
	_, err := user.Update(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update user")
}
