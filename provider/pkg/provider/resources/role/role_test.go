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
	"errors" // Still needed for mock server body
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	roleResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/role"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
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
func TestRoleHealthyLifeCycle(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	// fetch created resource to confirm
	getRole := mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles/testrole")).
			Reply(reply.OK().BodyString(`{
				"data": {"VM.PowerMgmt":1}
			}`)),
	)

	// perform create
	createRole := mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/roles")).
			Reply(reply.Created().BodyString(`{
				"data": {
				"roleid": "testrole",
				"privs": "VM.PowerMgmt"}	
			}`)).PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{getRole}}),
	)

	// perform delete
	deleteRole := mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/roles/testrole")).Reply(
			reply.OK()))

	// perform update
	updateRole := mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/roles/testrole")).
			// The API will receive "VM.PowerMgmt,Datastore.Allocate" from our provider
			// and return it in the response.
			Reply(reply.OK().BodyString(`{
	            "data": {"roleid":"testrole","name": "testrole",
				"privs": "VM.PowerMgmt,Datastore.Allocate", "special":0}}
	    `)))

	// Enable initial state
	getRole.Enable()
	createRole.Enable()
	deleteRole.Enable()
	updateRole.Enable()

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
		"name": property.New("testrole"),
		"privileges": property.New(property.NewArray([]property.Value{
			property.New("Datastore.Allocate"),
			property.New("VM.PowerMgmt"),
		})),
	})

	// Run the integration lifecycle tests
	integration.LifeCycleTest{
		Resource: "pve:role:Role",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name": property.New("testrole"),
				"privileges": property.New(property.NewArray([]property.Value{
					property.New("VM.PowerMgmt"),
				})),
			}),
			Hook: func(inputs, output property.Map) {
				assert.Equal(t, "testrole", output.Get("name").AsString())
				outputPrivileges := output.Get("privileges").AsArray().AsSlice()
				require.Len(t, outputPrivileges, 1)
				assert.Equal(t, "VM.PowerMgmt", outputPrivileges[0].AsString())
			},
		},
		Updates: []integration.Operation{
			{
				Inputs: property.NewMap(map[string]property.Value{
					"name": property.New("testrole"),
					"privileges": property.New(property.NewArray([]property.Value{
						property.New("Datastore.Allocate"),
						property.New("VM.PowerMgmt"),
					})),
				}),
				ExpectedOutput: &updatedResource,
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleCreateClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	role := &roleResource.Role{}
	request := infer.CreateRequest[roleResource.Inputs]{
		Name: "testrole",
		Inputs: roleResource.Inputs{
			Name:       "testrole",
			Privileges: []string{"VM.PowerMgmt"},
		},
	}
	_, err := role.Create(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleCreateCreationError(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/roles")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	role := &roleResource.Role{}
	request := infer.CreateRequest[roleResource.Inputs]{
		Name: "testrole",
		Inputs: roleResource.Inputs{
			Name:       "testrole",
			Privileges: []string{"VM.PowerMgmt"},
		},
	}
	_, err := role.Create(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to create role testrole: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleCreateFetchError(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/roles")).
			Reply(reply.Created().BodyString(`{
			"data": {
				"roleid": "testrole",
				"privs": "VM.PowerMgmt"}	
		}`)),
		mocha.Get(expect.URLPath("/access/roles/testrole")).
			Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	role := &roleResource.Role{}
	request := infer.CreateRequest[roleResource.Inputs]{
		Name: "testrole",
		Inputs: roleResource.Inputs{
			Name:       "testrole",
			Privileges: []string{"VM.PowerMgmt"},
		},
	}
	_, err := role.Create(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to fetch role testrole: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleDeleteClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	role := &roleResource.Role{}
	request := infer.DeleteRequest[roleResource.Outputs]{
		ID: "testrole",
	}
	_, err := role.Delete(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleDeleteDeletionError(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/roles/testrole")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	role := &roleResource.Role{}
	request := infer.DeleteRequest[roleResource.Outputs]{
		State: roleResource.Outputs{Inputs: roleResource.Inputs{Name: "testrole"}},
	}
	_, err := role.Delete(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to delete role testrole: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleReadSuccess(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	// List endpoint returns our target role with privileges unsorted to test normalization
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles/testrole")).Reply(reply.OK().BodyString(`{
			"data":{
				"Datastore.Allocate":1,"VM.PowerMgmt":1}
		}`)),
	).Enable()

	// env + client already configured; no additional override needed

	role := &roleResource.Role{}
	req := infer.ReadRequest[roleResource.Inputs, roleResource.Outputs]{
		ID:     "testrole",
		Inputs: roleResource.Inputs{Name: "testrole"},
		State: roleResource.Outputs{
			Inputs: roleResource.Inputs{Name: "testrole", Privileges: []string{"Datastore.Allocate"}},
		},
	}
	resp, err := role.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "testrole", resp.Inputs.Name)
	assert.Equal(t, []string{"Datastore.Allocate", "VM.PowerMgmt"}, resp.State.Privileges)
}

// Test does not set global environment variable, therefore can be parallelized!
func TestRoleReadIDNotFound(t *testing.T) {
	t.Parallel()

	role := &roleResource.Role{}
	request := infer.ReadRequest[roleResource.Inputs, roleResource.Outputs]{
		ID:     "",
		Inputs: roleResource.Inputs{Name: "testrole"},
		State:  roleResource.Outputs{Inputs: roleResource.Inputs{Name: "testrole"}},
	}
	response, err := role.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, response.ID, "")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleReadClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	role := &roleResource.Role{}
	request := infer.ReadRequest[roleResource.Inputs, roleResource.Outputs]{
		ID:     "testrole",
		Inputs: roleResource.Inputs{Name: "testrole"},
		State:  roleResource.Outputs{Inputs: roleResource.Inputs{Name: "testrole"}},
	}
	_, err := role.Read(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleReadNotFound(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles/testrole")).
			Reply(reply.BadRequest().BodyString(`{
			"data":null,
			"message":"role 'testrole' does not exist\n"
		}`)),
	).Enable()
	// env + client configured

	role := &roleResource.Role{}
	request := infer.ReadRequest[roleResource.Inputs, roleResource.Outputs]{
		ID: "testrole",
	}
	response, err := role.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Empty(t, response.ID)
	assert.Empty(t, response.State)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleReadGetResourceError(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles/testrole")).
			Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	role := &roleResource.Role{}
	request := infer.ReadRequest[roleResource.Inputs, roleResource.Outputs]{
		ID:     "testrole",
		Inputs: roleResource.Inputs{Name: "testrole"},
		State:  roleResource.Outputs{Inputs: roleResource.Inputs{Name: "testrole"}},
	}
	_, err := role.Read(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to get role testrole: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleUpdateClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	role := &roleResource.Role{}
	request := infer.UpdateRequest[roleResource.Inputs, roleResource.Outputs]{
		Inputs: roleResource.Inputs{
			Name:       "testrole",
			Privileges: []string{"Datastore.Allocate", "VM.PowerMgmt"},
		},
		State: roleResource.Outputs{
			Inputs: roleResource.Inputs{
				Name:       "testrole",
				Privileges: []string{"VM.PowerMgmt"},
			},
		},
	}
	_, err := role.Update(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestRoleUpdateUpdateError(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/roles/testrole")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	role := &roleResource.Role{}
	request := infer.UpdateRequest[roleResource.Inputs, roleResource.Outputs]{
		Inputs: roleResource.Inputs{
			Name:       "testrole",
			Privileges: []string{"Datastore.Allocate", "VM.PowerMgmt"},
		},
		State: roleResource.Outputs{
			Inputs: roleResource.Inputs{
				Name:       "testrole",
				Privileges: []string{"VM.PowerMgmt"},
			},
		},
	}
	_, err := role.Update(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to update role testrole: 500 Internal Server Error")
}
