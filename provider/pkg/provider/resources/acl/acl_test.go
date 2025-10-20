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

// aclHealthyLifeCycleHelper is used to test healthy lifecycle of ACL resource for different types.
func aclLHealthyLifeCycleHelper(t *testing.T, typ, bodystring, ugid string) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Validate 'type' existence during Create
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/" + typ + "s/" + ugid)).
			Reply(reply.OK().BodyString(bodystring)),
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
						"path":"/",
						"roleid":"PVEAdmin",
						"type": "` + typ + `",
						"ugid":"` + ugid + `",
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

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLHealthyLifeCycleGroup(t *testing.T) {
	aclLHealthyLifeCycleHelper(
		t,
		"group",
		`{"data":{"groupid":"testgroup","comment":"c"}}`,
		"testgroup",
	)
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLHealthyLifeCycleUser(t *testing.T) {
	aclLHealthyLifeCycleHelper(
		t,
		"user",
		`{"data":{"userid":"testuser","comment":"c"}}`,
		"testuser",
	)
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLHealthyLifeCycleToken(t *testing.T) {
	aclLHealthyLifeCycleHelper(
		t,
		"token",
		`{"data":{"userid":"svc!apitoken","comment":"c"}}`,
		"svc!apitoken",
	)
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

// aclCreateTypeNotFoundHelper is used to test Create failure when the specified type entity is not found.
func aclCreateTypeNotFoundHelper(t *testing.T, typ string) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Group lookup returns 404 -> causes early error
	mock.AddMocks(
		mocha.Get(expect.URLPath("/access/" + typ + "s/missing")),
	).Enable()

	// env + client already configured

	res := &aclResource.ACL{}
	_, err := res.Create(context.Background(), infer.CreateRequest[aclResource.Inputs]{
		Name: typ + "Missing",
		Inputs: aclResource.Inputs{
			Path:      "/",
			RoleID:    "PVEAdmin",
			Type:      typ,
			UGID:      "missing",
			Propagate: true,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to find "+typ) {
		t.Fatalf("expected "+typ+" not found error, got %v", err)
	}
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLCreateGroupNotFound(t *testing.T) {
	aclCreateTypeNotFoundHelper(t, "group")
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLCreateUserNotFound(t *testing.T) {
	aclCreateTypeNotFoundHelper(t, "user")
}

//nolint:paralleltest // sets PVE_API_URL & overrides GetProxmoxClientFn (global state)
func TestACLCreateUpdateACLFailure(t *testing.T) {
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
