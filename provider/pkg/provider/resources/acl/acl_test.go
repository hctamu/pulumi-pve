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
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	utils "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources"
	aclResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/acl"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLHealthyLifeCycle(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Validate group existence during Create
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/groups/testgroup")).
			Reply(reply.OK().BodyString(`{"data":{"groupid":"testgroup","comment":"c","guests":[]}}`)),
	).Enable()

	// Create (and later Delete) use PUT /access/acl
	mock.AddMocks(
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.OK()),
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.OK()), // delete call
	).Enable()

	// Read + Delete re-fetch list ACLs. Repeat enough times.
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).Repeat(10).Reply( // increased for multi-pass reads
			reply.OK().BodyString(`{
				"data":[
					{
						"path":"/vms",
						"roleid":"PVEVMAdmin",
						"type":"group",
						"ugid":"testgroup",
						"propagate":1
					}
				]
			}`)),
	).Enable()

	// env + client already configured by helper

	server, err := integration.NewServer(
		context.Background(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProvider()),
	)
	if err != nil {
		t.Fatalf("server init: %v", err)
	}

	createInputs := property.NewMap(map[string]property.Value{
		"path":      property.New("/vms"),
		"roleid":    property.New("PVEVMAdmin"),
		"type":      property.New("group"),
		"ugid":      property.New("testgroup"),
		"propagate": property.New(true),
	})

	integration.LifeCycleTest{
		Resource: "pve:acl:ACL",
		Create: integration.Operation{
			Inputs: createInputs,
			Hook: func(inputs, outputs property.Map) {
				if got := outputs.Get("ugid").AsString(); got != "testgroup" {
					t.Fatalf("unexpected ugid %s", got)
				}
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLHealthyLifeCycleUser(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Validate user existence during Create (use same ugid string everywhere)
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).
			Reply(reply.OK().BodyString(`{"data":{"userid":"testuser","comment":"c"}}`)),
	).Enable()

	// Create + Delete
	mock.AddMocks(
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.OK()),
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.OK()),
	).Enable()

	// Reads (create, refresh, delete...) provide extra repeats for safety
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).Repeat(10).Reply(
			reply.OK().BodyString(`{
				"data":[
					{
						"path":"/nodes",
						"roleid":"PVEAdmin",
						"type":"user",
						"ugid":"testuser",
						"propagate":1
					}
				]
			}`)),
	).Enable()

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
				"path":      property.New("/nodes"),
				"roleid":    property.New("PVEAdmin"),
				"type":      property.New("user"),
				"ugid":      property.New("testuser"),
				"propagate": property.New(true),
			}),
			Hook: func(_, outputs property.Map) {
				if outputs.Get("type").AsString() != "user" || outputs.Get("ugid").AsString() != "testuser" {
					t.Fatalf("expected user ugid=testuser got %s", outputs.Get("ugid").AsString())
				}
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLHealthyLifeCycleToken(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// No validation call for token type
	mock.AddMocks(
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.OK()),
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.OK()),
	).Enable()

	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).Repeat(10).Reply(
			reply.OK().BodyString(`{
				"data":[
					{
						"path":"/pool/dev",
						"roleid":"PVEAuditor",
						"type":"token",
						"ugid":"svc!apitoken",
						"propagate":0
					}
				]
			}`)),
	).Enable()

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
				"path":      property.New("/pool/dev"),
				"roleid":    property.New("PVEAuditor"),
				"type":      property.New("token"),
				"ugid":      property.New("svc!apitoken"),
				"propagate": property.New(false),
			}),
			Hook: func(_, outputs property.Map) {
				if outputs.Get("type").AsString() != "token" || outputs.Get("ugid").AsString() != "svc!apitoken" {
					t.Fatalf("expected token ugid=svc!apitoken")
				}
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLReadSuccess(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Matching ACL returned
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).Reply(reply.OK().BodyString(`{
			"data":[{"path":"/read","roleid":"PVEAdmin","type":"group","ugid":"grp1","propagate":1}]
		}`)),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	id := "/read|PVEAdmin|group|grp1"
	out, err := res.Read(context.Background(), infer.ReadRequest[aclResource.Inputs, aclResource.Outputs]{
		ID: id,
		Inputs: aclResource.Inputs{ // simulate previous inputs
			Path: "/read", RoleID: "PVEAdmin", Type: "group", UGID: "grp1", Propagate: true,
		},
	})
	if err != nil {
		t.Fatalf("read success branch errored: %v", err)
	}
	if out.ID != id {
		t.Fatalf("expected id %s got %s", id, out.ID)
	}
	if out.State.Path != "/read" || out.State.RoleID != "PVEAdmin" || !out.State.Propagate {
		t.Fatalf("unexpected state returned: %+v", out.State)
	}
}

//nolint:paralleltest // overrides GetProxmoxClientFn (global state)
func TestACLReadVanished(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Empty list -> resource considered gone
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).Reply(reply.OK().BodyString(`{"data":[]}`)),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	out, err := res.Read(context.Background(), infer.ReadRequest[aclResource.Inputs, aclResource.Outputs]{
		ID:     "/gone|PVEAdmin|group|ghost",
		Inputs: aclResource.Inputs{Path: "/gone", RoleID: "PVEAdmin", Type: "group", UGID: "ghost", Propagate: true},
	})
	if err != nil {
		t.Fatalf("read vanished branch errored: %v", err)
	}
	if out.ID != "" { // should be cleared to signal recreate
		t.Fatalf("expected empty id for vanished resource, got %s", out.ID)
	}
}

//nolint:paralleltest // overrides GetProxmoxClientFn (global state)
func TestACLReadInvalidID(t *testing.T) {
	cleanup := utils.OverrideClient(t, "http://invalid")
	defer cleanup()

	res := &aclResource.ACL{}
	_, err := res.Read(context.Background(), infer.ReadRequest[aclResource.Inputs, aclResource.Outputs]{ID: "bad-id"})
	if err == nil {
		t.Fatalf("expected error for invalid id")
	}
}

//nolint:paralleltest // overrides GetProxmoxClientFn (global state)
func TestACLReadMissingID(t *testing.T) {
	cleanup := utils.OverrideClient(t, "http://invalid")
	defer cleanup()

	res := &aclResource.ACL{}
	_, err := res.Read(context.Background(), infer.ReadRequest[aclResource.Inputs, aclResource.Outputs]{ID: ""})
	if err == nil {
		t.Fatalf("expected error for missing id")
	}
}

//nolint:paralleltest // overrides GetProxmoxClientFn (global state)
func TestACLCreateValidationErrors(t *testing.T) {
	cleanup := utils.OverrideClient(t, "http://invalid")
	defer cleanup()

	cases := []struct {
		name    string
		inputs  aclResource.Inputs
		wantErr string
	}{
		{"missing path", aclResource.Inputs{RoleID: "r", Type: "token", UGID: "x"}, "path must be specified"},
		{"missing role", aclResource.Inputs{Path: "/p", Type: "token", UGID: "x"}, "roleid must be specified"},
		{"invalid type", aclResource.Inputs{Path: "/p", RoleID: "r", Type: "bad", UGID: "x"}, "invalid type"},
		{"missing ugid", aclResource.Inputs{Path: "/p", RoleID: "r", Type: "token"}, "ugid must be specified"},
	}
	res := &aclResource.ACL{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := res.Create(context.Background(), infer.CreateRequest[aclResource.Inputs]{
				Name:   tc.name,
				Inputs: tc.inputs,
			})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q got %v", tc.wantErr, err)
			}
		})
	}
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLCreateGroupNotFound(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Group lookup returns 404 -> causes early error
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/groups/missing")),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	_, err := res.Create(context.Background(), infer.CreateRequest[aclResource.Inputs]{
		Name:   "groupMissing",
		Inputs: aclResource.Inputs{Path: "/vms", RoleID: "PVEVMAdmin", Type: "group", UGID: "missing", Propagate: true},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to find group") {
		t.Fatalf("expected group not found error, got %v", err)
	}
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLCreateUserNotFound(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// User lookup returns 404 -> early error
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/users/missing")),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	_, err := res.Create(context.Background(), infer.CreateRequest[aclResource.Inputs]{
		Name:   "userMissing",
		Inputs: aclResource.Inputs{Path: "/nodes", RoleID: "PVEAdmin", Type: "user", UGID: "missing", Propagate: true},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to find user") {
		t.Fatalf("expected user not found error, got %v", err)
	}
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLCreateUpdateACLFailureToken(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// PUT /access/acl returns 500 to force failure
	mock.AddMocks(
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	_, err := res.Create(context.Background(), infer.CreateRequest[aclResource.Inputs]{
		Name: "tokenFail",
		Inputs: aclResource.Inputs{
			Path:      "/pool/dev",
			RoleID:    "PVEAuditor",
			Type:      "token",
			UGID:      "svc!fail",
			Propagate: false,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to create ACL") {
		t.Fatalf("expected create failure error, got %v", err)
	}
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLDeleteAlreadyGone(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Empty ACL list so GetACL returns ErrACLNotFound -> Delete should be no-op success.
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).Reply(reply.OK().BodyString(`{"data":[]}`)),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	// Provide existing state (will be re-fetched and not found)
	_, err := res.Delete(context.Background(), infer.DeleteRequest[aclResource.Outputs]{
		ID: "/gone|PVEAdmin|group|ghost",
		State: aclResource.Outputs{
			Inputs: aclResource.Inputs{Path: "/gone", RoleID: "PVEAdmin", Type: "group", UGID: "ghost", Propagate: true},
		},
	})
	if err != nil {
		t.Fatalf("expected no error for already gone delete, got %v", err)
	}
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLDeleteUpdateFailure(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// List returns matching user ACL; PUT fails with 500 to trigger error path.
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).
			Reply(reply.OK().BodyString(`{"data":[{"path":"/nodes",
			"roleid":"PVEAdmin","type":"user","ugid":"bob","propagate":1}]}`)),
		mocha.Put(expect.URLPath("/access/acl")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	_, err := res.Delete(context.Background(), infer.DeleteRequest[aclResource.Outputs]{
		ID: "/nodes|PVEAdmin|user|bob",
		State: aclResource.Outputs{
			Inputs: aclResource.Inputs{Path: "/nodes", RoleID: "PVEAdmin", Type: "user", UGID: "bob", Propagate: true},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to delete ACL") {
		t.Fatalf("expected delete failure error, got %v", err)
	}
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLDeleteInvalidTypeBranch(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// List returns ACL with invalid type string so we reach switch default.
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/acl")).
			Reply(reply.OK().BodyString(`{"data":[{"path":"/weird",
			"roleid":"PVEAdmin","type":"invalid","ugid":"x","propagate":0}]}`)),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	_, err := res.Delete(context.Background(), infer.DeleteRequest[aclResource.Outputs]{
		ID: "/weird|PVEAdmin|invalid|x",
		State: aclResource.Outputs{
			Inputs: aclResource.Inputs{Path: "/weird", RoleID: "PVEAdmin", Type: "invalid", UGID: "x", Propagate: false},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid type") {
		t.Fatalf("expected invalid type error, got %v", err)
	}
}
