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
	"net/http"
	"strings"
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
	"github.com/vitorsalgado/mocha/v3/params"
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

	getUser := mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/testuser")).
			ReplyFunction(
				func(request *http.Request, m reply.M, p params.P) (*reply.Response, error) {
					r := strings.NewReader(`{"data":{"userid":"testuser"}}`)
					return &reply.Response{
						Status: http.StatusOK,
						Body:   r,
					}, nil
				},
			),
	)

	// Create user
	createUser := mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/users")).
			Reply(reply.Created().BodyString(`{"data":{"userid":"testuser"}}`)).
			PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{getUser}}),
	)

	// Update user
	updateUser := mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/users/testuser")).
			Reply(reply.OK().BodyString(`{"data":{
					"userid":"testuser",
					"comment":"updated comment",
					"enable":1,
					"expire":42,
					"firstname":"Bob",
					"lastname":"Balancer",
					"email":"bob@example.com",
					"groups":["g1","g2"],
					"keys":"ssh-ed25519,ssh-rsa"}}`)),
	)

	// Delete user
	deleteUser := mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/users/testuser")).Reply(reply.OK()),
	)

	// Enable mocks
	createUser.Enable()
	getUser.Enable()
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

	expectedAfterUpdate := property.NewMap(map[string]property.Value{
		"userid":    property.New("testuser"),
		"comment":   property.New("updated comment"),
		"enable":    property.New(true), // enable toggled to true
		"expire":    property.New(42.0), // updated expire value
		"firstname": property.New("Bob"),
		"lastname":  property.New("Balancer"),
		"email":     property.New("bob@example.com"),
		"groups": property.New(property.NewArray([]property.Value{ // output preserves input order (unsorted)
			property.New("g1"),
			property.New("g2"),
		})),
		"keys": property.New(property.NewArray([]property.Value{ // update output preserves input order here
			property.New("ssh-ed25519"),
			property.New("ssh-rsa"),
		})),
		"password": property.New(""), // provider clears password on Read
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
					"groups": property.New(property.NewArray([]property.Value{ // intentionally unsorted input
						property.New("g2"),
						property.New("g1"),
					})),
					"keys": property.New(property.NewArray([]property.Value{ // intentionally unsorted input
						property.New("ssh-rsa"),
						property.New("ssh-ed25519"),
					})),
				}),
				ExpectedOutput: &expectedAfterUpdate,
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // global seam
func TestUserReadSuccess(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/readuser")).Reply(reply.OK().BodyString(`{
            "data":{
			"userid":"readuser",
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

	u := &userResource.User{}
	req := infer.ReadRequest[userResource.Inputs, userResource.Outputs]{
		ID:     "readuser",
		Inputs: userResource.Inputs{Name: "readuser"},
		State:  userResource.Outputs{Inputs: userResource.Inputs{Name: "readuser"}},
	}
	resp, err := u.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "readuser", resp.State.Name)
	assert.Equal(t, []string{"g1", "g2"}, resp.State.Groups)
	assert.Equal(t, []string{"ssh-ed25519 BBB", "ssh-rsa AAA"}, resp.State.Keys) // sorted
	assert.Equal(t, "F", resp.State.Firstname)
	assert.Equal(t, 10, resp.State.Expire)
}

//nolint:paralleltest // global env
func TestUserUpdateChangeError(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mockServer.AddMocks(
		// mocha.Get(expect.URLPath("/access/users/userid")).
		// 	Reply(reply.OK().BodyString(`{"data":{"userid":"userid"}}`)),
		mocha.Put(expect.URLPath("/access/users/userid")).Reply(reply.InternalServerError()),
	).Enable()

	u := &userResource.User{}
	req := infer.UpdateRequest[userResource.Inputs, userResource.Outputs]{
		State:  userResource.Outputs{Inputs: userResource.Inputs{Name: "userid"}},
		Inputs: userResource.Inputs{Name: "userid"},
	}
	_, err := u.Update(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update user")
}

//nolint:paralleltest // seam override
func TestUserDeleteClientAcquisitionFailure(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("acquire fail") }

	u := &userResource.User{}
	req := infer.DeleteRequest[userResource.Outputs]{
		State: userResource.Outputs{Inputs: userResource.Inputs{Name: "deluser"}},
	}
	_, err := u.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire fail")
}

//nolint:paralleltest // env
func TestUserDeleteBackendFailure(t *testing.T) {
	mockServer, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/users/deluser")).
			Reply(reply.OK().BodyString(`{"data":{
			"userid":"deluser",
			"comment":"c",
			"enable":1,
			"expire":0,
			"firstname":"A",
			"lastname":"B",
			"email":"e",
			"groups":[],
			"keys":""}}`)),
		mocha.Delete(expect.URLPath("/access/users/deluser")).Reply(reply.InternalServerError()),
	).Enable()

	u := &userResource.User{}
	req := infer.DeleteRequest[userResource.Outputs]{
		State: userResource.Outputs{Inputs: userResource.Inputs{Name: "deluser"}},
	}
	_, err := u.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete user")
}
