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
	"net/http"
	"strings"
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
	"github.com/vitorsalgado/mocha/v3/params"
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

	getGroup := mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/groups/testgroup")).
			Repeat(2).
			ReplyFunction(
				func(request *http.Request, m reply.M, p params.P) (*reply.Response, error) {
					r := strings.NewReader(`{
			"data": {
				"groupid": "testgroup",
				"comment": "updated comment" ,
				"guests": []
			  }
		  }`)
					return &reply.Response{
						Status: http.StatusOK,
						Body:   r,
					}, nil
				},
			),
	)

	createGroup := mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/groups")).Reply(reply.Created().BodyString(`{
            "data": {
                "groupid": "testgroup",
                "name": "testgroup",
                "comment": "comment",
                "guests": []
            }
        }`)).PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{getGroup}}),
	)

	deleteGroup := mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/groups/testgroup")).Reply(
			reply.OK()))

	updateGroup := mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/groups/testgroup")).Reply(reply.OK().BodyString(`{
	            "data": {
	                "name": "testgroup",
	                "comment": "updated comment",
	                "guests": []
	  }
	    }`)))

	// Enable initial state
	createGroup.Enable()
	deleteGroup.Enable()
	getGroup.Enable()
	updateGroup.Enable()

	// Set environment variable to direct Proxmox API requests to the mock server
	// env + client configured by helper

	// Start the integration server with the mock setup
	server, err := integration.NewServer(
		t.Context(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProvider()),
	)
	require.NoError(t, err)

	outputMap := property.NewMap(map[string]property.Value{
		"name":    property.New("testgroup"),
		"comment": property.New("updated comment"),
	})

	// Run the integration lifecycle tests
	integration.LifeCycleTest{
		Resource: "pve:group:Group",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name":    property.New("testgroup"),
				"comment": property.New("test group comment"),
			}),
			Hook: func(inputs, output property.Map) {
				t.Logf("Outputs after Create: %v", output)
				assert.Equal(t, "testgroup", output.Get("name").AsString())
			},
		},
		Updates: []integration.Operation{
			{
				Inputs: property.NewMap(map[string]property.Value{
					"name":    property.New("testgroup"),
					"comment": property.New("updated comment"),
				}),
				ExpectedOutput: &outputMap,
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // shares env mutation
func TestGroupReadSuccessWithSeam(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/groups/readgroup")).Reply(reply.OK().BodyString(`{
			"data": {"groupid":"readgroup","comment":"read comment","guests":[]}
		}`)),
	).Enable()

	// env + client configured

	group := &groupResource.Group{}
	request := infer.ReadRequest[groupResource.Inputs, groupResource.Outputs]{
		ID:     "readgroup",
		Inputs: groupResource.Inputs{Name: "readgroup"},
		State:  groupResource.Outputs{Inputs: groupResource.Inputs{Name: "readgroup", Comment: "stale"}},
	}

	resp, err := group.Read(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, "readgroup", resp.State.Name)
	assert.Equal(t, "read comment", resp.State.Comment)
}

//nolint:paralleltest // shares env mutation
func TestGroupReadMissingIDWithSeam(t *testing.T) {
	cleanup := utils.OverrideClient(t, "http://invalid")
	defer cleanup()

	group := &groupResource.Group{}
	request := infer.ReadRequest[groupResource.Inputs, groupResource.Outputs]{
		ID:     "", // triggers missing ID branch
		Inputs: groupResource.Inputs{Name: "whatever"},
	}
	_, err := group.Read(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing group ID")
}

//nolint:paralleltest // shares env mutation
func TestGroupReadBackendErrorWithSeam(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	// Malformed JSON to force error in underlying client lib
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/groups/badgroup")).Reply(reply.OK().BodyString(`{"data": {`)),
	).Enable()
	// env + client configured

	group := &groupResource.Group{}
	request := infer.ReadRequest[groupResource.Inputs, groupResource.Outputs]{
		ID:     "badgroup",
		Inputs: groupResource.Inputs{Name: "badgroup"},
	}
	_, err := group.Read(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get group")
}

//nolint:paralleltest // mutates seam
func TestGroupUpdateClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client boom") }

	group := &groupResource.Group{}
	request := infer.UpdateRequest[groupResource.Inputs, groupResource.Outputs]{
		State:  groupResource.Outputs{Inputs: groupResource.Inputs{Name: "g1", Comment: "old"}},
		Inputs: groupResource.Inputs{Name: "g1", Comment: "new"},
	}
	_, err := group.Update(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client boom")
}

//nolint:paralleltest // shares env + seam
func TestGroupUpdateFetchError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	// Return 500 so pxc.Group errors
	mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/groups/g1")).Reply(reply.InternalServerError()),
	).Enable()

	// env + client configured

	group := &groupResource.Group{}
	request := infer.UpdateRequest[groupResource.Inputs, groupResource.Outputs]{
		State:  groupResource.Outputs{Inputs: groupResource.Inputs{Name: "g1", Comment: "old"}},
		Inputs: groupResource.Inputs{Name: "g1", Comment: "new"},
	}
	_, err := group.Update(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update group")
}
