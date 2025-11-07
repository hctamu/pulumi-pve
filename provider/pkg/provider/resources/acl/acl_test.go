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
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	utils "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources"
	aclResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/acl"
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

// aclHealthyLifeCycleHelper is used to test healthy lifecycle of ACL resource for different types.
func aclLHealthyLifeCycleHelper(t *testing.T, typ, bodystring, ugid string) {
	mockServer, cleanup := utils.NewAPIMock(t)
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

	server, err := integration.NewServer(
		context.Background(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProvider()),
	)
	if err != nil {
		t.Fatalf("server init: %v", err)
	}

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
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	acl := &aclResource.ACL{}
	request := infer.CreateRequest[aclResource.Inputs]{
		Name: "/|PVEAdmin|user|testuser",
		Inputs: aclResource.Inputs{
			Path:      "/",
			RoleID:    "PVEAdmin",
			Type:      "user",
			UGID:      "testuser",
			Propagate: true,
		},
	}
	_, err := acl.Create(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client error")
}

// aclCreateTypeNotFoundHelper is used to test Create failure when the specified type entity is not found.
func aclCreateTypeNotFoundHelper(t *testing.T, typ string) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/" + typ + "s/test" + typ)).Reply(reply.NotFound()),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	_, err := res.Create(context.Background(), infer.CreateRequest[aclResource.Inputs]{
		Name: "test" + typ,
		Inputs: aclResource.Inputs{
			Path:      "/",
			RoleID:    "PVEAdmin",
			Type:      typ,
			UGID:      "test" + typ,
			Propagate: true,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to find "+typ) {
		t.Fatalf("expected "+typ+" not found error, got %v", err)
	}
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
	_, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	acl := &aclResource.ACL{}
	request := infer.CreateRequest[aclResource.Inputs]{
		Name: "/|PVEAdmin|invalid|testinvalid",
		Inputs: aclResource.Inputs{
			Path:      "/",
			RoleID:    "PVEAdmin",
			Type:      "invalid",
			UGID:      "testinvalid",
			Propagate: true,
		},
	}
	_, err := acl.Create(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateCreationError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).
			Reply(reply.OK().BodyString(`{"data":{"userid":"testuser"}}`)),
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	acl := &aclResource.ACL{}
	request := infer.CreateRequest[aclResource.Inputs]{
		Name: "/|PVEAdmin|user|testuser",
		Inputs: aclResource.Inputs{
			Path:   "/",
			RoleID: "PVEAdmin",
			Type:   "user",
			UGID:   "testuser",
		},
	}
	_, err := acl.Create(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create ACL")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLCreateFetchError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).
			Reply(reply.OK().BodyString(`{"data":{"userid":"testuser"}}`)),
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.OK()),
		mocha.Get(expect.URLPath("/access/acl")).
			Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	acl := &aclResource.ACL{}
	request := infer.CreateRequest[aclResource.Inputs]{
		Name: "/|PVEAdmin|user|testuser",
		Inputs: aclResource.Inputs{
			Path:      "/",
			RoleID:    "PVEAdmin",
			Type:      "user",
			UGID:      "testuser",
			Propagate: true,
		},
	}
	_, err := acl.Create(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch ACL")
}

// Test does not set global environment variable, therefore can be parallelized!
func TestACLDeleteInvalidType(t *testing.T) {
	acl := &aclResource.ACL{}
	request := infer.DeleteRequest[aclResource.Outputs]{
		ID: "/|PVEAdmin|invalid|testinvalid",
		State: aclResource.Outputs{
			Inputs: aclResource.Inputs{
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
	assert.Contains(t, err.Error(), "invalid type")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLDeleteClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	acl := &aclResource.ACL{}
	request := infer.DeleteRequest[aclResource.Outputs]{
		ID: "/|PVEAdmin|user|testuser",
		State: aclResource.Outputs{
			Inputs: aclResource.Inputs{
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
	assert.Contains(t, err.Error(), "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLDeleteDeletionError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).
			Reply(reply.OK().BodyString(`{"data":[{"path":"/","roleid":"PVEAdmin","type":"user","ugid":"testuser","propagate":1}]}`)),
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	acl := &aclResource.ACL{}
	request := infer.DeleteRequest[aclResource.Outputs]{
		ID: "/|PVEAdmin|user|testuser",
		State: aclResource.Outputs{
			Inputs: aclResource.Inputs{
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
	assert.Contains(t, err.Error(), "failed to delete ACL")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadSuccess(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
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

	// env + client already configured

	acl := &aclResource.ACL{}
	id := "/|PVEAdmin|group|testgroup"
	response, err := acl.Read(context.Background(), infer.ReadRequest[aclResource.Inputs, aclResource.Outputs]{
		ID: id,
		Inputs: aclResource.Inputs{
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
	acl := &aclResource.ACL{}
	request := infer.ReadRequest[aclResource.Inputs, aclResource.Outputs]{
		ID:     "",
		Inputs: aclResource.Inputs{Path: "/", RoleID: "PVEAdmin", Type: "group", UGID: "testgroup", Propagate: false},
		State: aclResource.Outputs{
			Inputs: aclResource.Inputs{Path: "/", RoleID: "PVEAdmin", Type: "group", UGID: "testgroup", Propagate: false},
		},
	}
	response, err := acl.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, response.ID, "")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	acl := &aclResource.ACL{}
	_, err := acl.Read(context.Background(), infer.ReadRequest[aclResource.Inputs, aclResource.Outputs]{
		ID: "/|PVEAdmin|group|testgroup",
		Inputs: aclResource.Inputs{ // simulate previous inputs
			Path: "/", RoleID: "PVEAdmin", Type: "group", UGID: "testgroup", Propagate: false,
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadACLIDError(t *testing.T) {
	_, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	acl := &aclResource.ACL{}
	_, err := acl.Read(context.Background(), infer.ReadRequest[aclResource.Inputs, aclResource.Outputs]{
		ID: "invalid-acl-id-format",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ACL ID")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestACLReadNotFound(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
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

	// env + client configured

	acl := &aclResource.ACL{}
	request := infer.ReadRequest[aclResource.Inputs, aclResource.Outputs]{
		ID: "/|PVEAdmin|user|testuser",
	}
	_, err := acl.Read(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ACL")
}
