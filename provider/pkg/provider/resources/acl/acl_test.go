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

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	aclResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/acl"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
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

// mockACLOperations provides a simple mock for proxmox.ACLOperations
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

// aclHealthyLifeCycleHelper is used to test healthy lifecycle of ACL resource for different types.
func aclLHealthyLifeCycleHelper(t *testing.T, typ, bodystring, ugid string) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	// get the type entity
	getACLTypeResource := mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/" + typ + "s/" + ugid)).
			Reply(reply.OK().BodyString(bodystring)),
	)

	// list ACLs endpoint returns an object with a data field (array) matching go-proxmox expectations.
	// Include unrelated ACLs to ensure we exercise scanning logic instead of trivially matching the first element.
	getACL := mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).Reply(reply.OK().BodyJSON(
			map[string]interface{}{
				"data": []map[string]interface{}{
					{ // unrelated different path
						"path":      "/nodes",
						"roleid":    "PVEAdmin",
						"type":      "group", // ensure mix of types
						"ugid":      "othergroup",
						"propagate": 1,
					},
					{ // unrelated different role
						"path":      "/",
						"roleid":    "PVEAuditor",
						"type":      "user",
						"ugid":      "someoneelse",
						"propagate": 0,
					},
					{ // target ACL we expect to find
						"path":      "/",
						"roleid":    "PVEAdmin",
						"type":      typ,
						"ugid":      ugid,
						"propagate": 1, // API returns IntOrBool, keep numeric for consistency
					},
				},
			},
		)),
	)

	// perform create
	createACL := mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/acl")).
			Reply(reply.OK()).
			PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{getACL}}),
	)

	// perform delete
	deleteACL := mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.OK()),
	)

	// enable mocks
	getACLTypeResource.Enable()
	getACL.Enable()
	createACL.Enable()
	deleteACL.Enable()

	// env + client already configured

	testConfig := &config.Config{
		PveURL:   mockServer.URL(),
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}

	server, err := integration.NewServer(
		context.Background(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProviderWithConfig(testConfig)),
	)
	require.NoError(t, err)

	integration.LifeCycleTest{
		Resource: "pve:acl:ACL",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"path":      property.New("/"),
				"roleid":    property.New("PVEAdmin"),
				"type":      property.New(typ),
				"ugid":      property.New(ugid),
				"propagate": property.New(true),
			}),
			Hook: func(_, outputs property.Map) {
				if outputs.Get("type").AsString() != typ || outputs.Get("ugid").AsString() != ugid {
					t.Fatalf("expected user ugid=%s got %s", ugid, outputs.Get("ugid").AsString())
				}
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLHealthyLifeCycleGroup(t *testing.T) {
	aclLHealthyLifeCycleHelper(
		t,
		"group",
		`{"data":{"groupid":"testgroup","comment":"c"}}`,
		"testgroup",
	)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLHealthyLifeCycleUser(t *testing.T) {
	aclLHealthyLifeCycleHelper(
		t,
		"user",
		`{"data":{"userid":"testuser","comment":"c"}}`,
		"testuser",
	)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLHealthyLifeCycleToken(t *testing.T) {
	aclLHealthyLifeCycleHelper(
		t,
		"token",
		`{"data":{"userid":"svc!apitoken","comment":"c"}}`,
		"svc!apitoken",
	)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateClientError(t *testing.T) {
	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{createFunc: func(ctx context.Context, inputs proxmox.ACLInputs) error {
			return errors.New("client error")
		}},
	}
	request := infer.CreateRequest[proxmox.ACLInputs]{
		Name: "/|PVEAdmin|user|testuser",
		Inputs: proxmox.ACLInputs{
			Path:      "/",
			RoleID:    "PVEAdmin",
			Type:      "user",
			UGID:      "testuser",
			Propagate: true,
		},
	}
	_, err := acl.Create(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

// aclCreateTypeNotFoundHelper is used to test Create failure when the specified type entity is not found.
func aclCreateTypeNotFoundHelper(t *testing.T, typ string) {
	mock, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/" + typ + "s/test" + typ)).Reply(reply.InternalServerError()),
	).Enable()

	// Resource with real adapter wired to mock server
	res := &aclResource.ACL{ACLOps: adapters.NewACLAdapter(adapters.NewProxmoxAdapter(&config.Config{
		PveURL:   mock.URL(),
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}))}
	response, err := res.Create(context.Background(), infer.CreateRequest[proxmox.ACLInputs]{
		Name: "test" + typ,
		Inputs: proxmox.ACLInputs{
			Path:      "/",
			RoleID:    "PVEAdmin",
			Type:      typ,
			UGID:      "test" + typ,
			Propagate: true,
		},
	})
	assert.Error(t, err)
	assert.EqualError(t, err, "failed to find "+typ+" "+response.Output.UGID+" for ACL: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateGroupNotFound(t *testing.T) {
	aclCreateTypeNotFoundHelper(t, "group")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateUserNotFound(t *testing.T) {
	aclCreateTypeNotFoundHelper(t, "user")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateInvalidType(t *testing.T) {
	acl := &aclResource.ACL{
		ACLOps: &mockACLOperations{createFunc: func(ctx context.Context, inputs proxmox.ACLInputs) error {
			return errors.New("invalid type (must be 'user', 'group', or 'token')")
		}},
	}
	request := infer.CreateRequest[proxmox.ACLInputs]{
		Name: "/|PVEAdmin|invalid|testinvalid",
		Inputs: proxmox.ACLInputs{
			Path:      "/",
			RoleID:    "PVEAdmin",
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
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).
			Reply(reply.OK().BodyString(`{"data":{"userid":"testuser"}}`)),
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.InternalServerError()),
	).Enable()

	acl := &aclResource.ACL{ACLOps: adapters.NewACLAdapter(adapters.NewProxmoxAdapter(&config.Config{
		PveURL:   mockServer.URL(),
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}))}
	request := infer.CreateRequest[proxmox.ACLInputs]{
		Name: "/|PVEAdmin|user|testuser",
		Inputs: proxmox.ACLInputs{
			Path:   "/",
			RoleID: "PVEAdmin",
			Type:   "user",
			UGID:   "testuser",
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
		ID: "/|PVEAdmin|invalid|testinvalid",
		State: proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{
				Path:      "/",
				RoleID:    "PVEAdmin",
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
		ID: "/|PVEAdmin|user|testuser",
		State: proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{
				Path:      "/",
				RoleID:    "PVEAdmin",
				Type:      "user",
				UGID:      "testuser",
				Propagate: true,
			},
		},
	}
	_, err := acl.Delete(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLDeleteDeletionError(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).
			Reply(reply.OK().BodyString(`{"data":[{
				"path":"/",
				"roleid":"PVEAdmin",
				"type":"user","ugid":
				"testuser","propagate":1}]
			}`)),
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.InternalServerError()),
	).Enable()

	acl := &aclResource.ACL{ACLOps: adapters.NewACLAdapter(adapters.NewProxmoxAdapter(&config.Config{
		PveURL:   mockServer.URL(),
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}))}
	request := infer.DeleteRequest[proxmox.ACLOutputs]{
		ID: "/|PVEAdmin|user|testuser",
		State: proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{
				Path:      "/",
				RoleID:    "PVEAdmin",
				Type:      "user",
				UGID:      "testuser",
				Propagate: true,
			},
		},
	}
	_, err := acl.Delete(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "failed to delete ACL resource: 500 Internal Server Error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadSuccess(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).Reply(reply.OK().BodyString(`{
			"data":[{
			"path":"/",
			"roleid":"PVEAdmin",
			"type":"group",
			"ugid":"testgroup",
			"propagate":0}]
		}`)),
	).Enable()

	acl := &aclResource.ACL{ACLOps: adapters.NewACLAdapter(adapters.NewProxmoxAdapter(&config.Config{
		PveURL:   mockServer.URL(),
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}))}
	id := "/|PVEAdmin|group|testgroup"
	response, err := acl.Read(context.Background(), infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID: id,
		Inputs: proxmox.ACLInputs{
			Path: "/", RoleID: "PVEAdmin", Type: "group", UGID: "testgroup", Propagate: false,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, response.ID, id)
	assert.Equal(t, response.State.Path, "/")
	assert.Equal(t, response.State.RoleID, "PVEAdmin")
	assert.Equal(t, response.State.UGID, "testgroup")
	assert.Equal(t, response.State.Propagate, false)
}

// Test does not set global environment variable, therefore can be parallelized!
func TestACLReadIDNotFound(t *testing.T) {
	t.Parallel()

	acl := &aclResource.ACL{ACLOps: &mockACLOperations{}}
	request := infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID:     "",
		Inputs: proxmox.ACLInputs{Path: "/", RoleID: "PVEAdmin", Type: "group", UGID: "testgroup", Propagate: false},
		State: proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{Path: "/", RoleID: "PVEAdmin", Type: "group", UGID: "testgroup", Propagate: false},
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
		ID: "/|PVEAdmin|group|testgroup",
		Inputs: proxmox.ACLInputs{ // simulate previous inputs
			Path: "/", RoleID: "PVEAdmin", Type: "group", UGID: "testgroup", Propagate: false,
		},
	})
	assert.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadACLIDError(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	acl := &aclResource.ACL{ACLOps: adapters.NewACLAdapter(adapters.NewProxmoxAdapter(&config.Config{
		PveURL:   mockServer.URL(),
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}))}
	_, err := acl.Read(context.Background(), infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID: "invalid-acl-id-format",
	})
	assert.Error(t, err)
	assert.EqualError(t, err, "invalid ACL ID: invalid-acl-id-format")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadNotFound(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).Reply(reply.OK()),
		mocha.Get(expect.URLPath("/access/acl")).Reply(reply.OK().BodyString(`{
			"data":[{
			"path":"/",
			"roleid":"PVEAdmin",
			"type":"group",
			"ugid":"testgroup",
			"propagate":0}]
		}`)),
	).Enable()

	acl := &aclResource.ACL{ACLOps: adapters.NewACLAdapter(adapters.NewProxmoxAdapter(&config.Config{
		PveURL:   mockServer.URL(),
		PveUser:  "user@pve!token",
		PveToken: "TOKEN",
	}))}
	request := infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs]{
		ID: "/|PVEAdmin|user|testuser",
	}
	_, err := acl.Read(context.Background(), request)
	require.Error(t, err)
	assert.EqualError(t, err, "acl not found")
}
