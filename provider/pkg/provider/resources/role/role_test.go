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
	"net/http"
	"os"
	"strings" // Still needed for mock server body
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	roleResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/role"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"
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
func TestRoleHealthyLifeCycle(t *testing.T) {
	// Start the mock server
	mockServer := mocha.New(t)
	mockServer.Start()
	defer func() {
		if err := mockServer.Close(); err != nil {
			t.Errorf("failed to close mock server: %v", err)
		}
	}()

	// List roles endpoint used by GetRole via pxc.Roles(ctx).
	listCall := 0
	listRoles := mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles")).
			Repeat(4).
			ReplyFunction(func(request *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				listCall++
				apiPrivs := "VM.PowerMgmt" // Initial privileges as returned by API (comma-separated string)
				if listCall >= 2 {         // After update, we expect two privileges
					apiPrivs = "VM.PowerMgmt,Datastore.Allocate" // API returns comma-separated string
				}
				body := strings.NewReader(`{"data":[{"roleid":
				"testrole","privs":"` + apiPrivs + `","special":0}]}`)
				return &reply.Response{Status: http.StatusOK, Body: body}, nil
			}),
	)

	createRole := mockServer.AddMocks(
		mocha.Post(expect.URLPath("/access/roles")).
			// The API will receive a comma-separated string from our provider
			// and return it in the response.
			Reply(reply.Created().BodyString(`{
				"data": {"roleid": "testrole", "name": "testrole", 
				"privs": "VM.PowerMgmt", "special": 0}}
			`)).PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{listRoles}}),
	)

	deleteRole := mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/roles/testrole")).Reply(
			reply.OK()))

	updateRole := mockServer.AddMocks(
		mocha.Put(expect.URLPath("/access/roles/testrole")).
			// The API will receive "VM.PowerMgmt,Datastore.Allocate" from our provider
			// and return it in the response.
			Reply(reply.OK().BodyString(`{
	            "data": {"roleid":"testrole","name": "testrole",
				"privs": "VM.PowerMgmt,Datastore.Allocate", "special":0}}
	    `)))

	// Enable initial state
	createRole.Enable()
	deleteRole.Enable()
	listRoles.Enable()
	updateRole.Enable()

	// Set environment variable to direct Proxmox API requests to the mock server
	_ = os.Setenv("PVE_API_URL", mockServer.URL())
	defer func() {
		if err := os.Unsetenv("PVE_API_URL"); err != nil {
			t.Errorf("failed to unset PVE_API_URL: %v", err)
		}
	}()

	// Start the integration server with the mock setup
	server, err := integration.NewServer(
		t.Context(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProvider()),
	)
	require.NoError(t, err)

	outputMap := property.NewMap(map[string]property.Value{
		"name": property.New("testrole"),
		"privileges": property.New(property.NewArray([]property.Value{
			property.New("Datastore.Allocate"), // Sorted alphabetically
			property.New("VM.PowerMgmt"),       // Sorted alphabetically
		})),
	})

	// Run the integration lifecycle tests
	integration.LifeCycleTest{
		Resource: "pve:role:Role",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"name": property.New("testrole"),
				"privileges": property.New(property.NewArray([]property.Value{
					property.New("VM.PowerMgmt"), // Input order doesn't strictly matter, provider sorts for API
				})),
			}),
			Hook: func(inputs, output property.Map) {
				assert.Equal(t, "testrole", output.Get("name").AsString())

				// Using AsArray().AsSlice() as a workaround if AsArray() alone is still problematic
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
				ExpectedOutput: &outputMap,
			},
		},
	}.Run(t, server)
}

//nolint:paralleltest // shares env mutation
func TestRoleReadSuccessWithSeam(t *testing.T) {
	mockServer := mocha.New(t)
	mockServer.Start()
	defer func() { _ = mockServer.Close() }()

	// List endpoint returns our target role with privileges unsorted to test normalization
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles")).Reply(reply.OK().BodyString(`{
			"data": [
				{"roleid":"readrole","privs":"VM.PowerMgmt,Datastore.Allocate","special":0}
			]
		}`)),
	).Enable()

	_ = os.Setenv("PVE_API_URL", mockServer.URL())
	defer func() {
		if err := os.Unsetenv("PVE_API_URL"); err != nil {
			t.Errorf("failed to unset PVE_API_URL: %v", err)
		}
	}()

	// Override client seam to avoid Pulumi config plumbing
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) {
		apiClient := api.NewClient(mockServer.URL(), api.WithAPIToken("user@pve!token", "TOKEN"))
		return &px.Client{Client: apiClient}, nil
	}

	role := &roleResource.Role{}
	req := infer.ReadRequest[roleResource.Inputs, roleResource.Outputs]{
		ID:     "readrole",
		Inputs: roleResource.Inputs{Name: "readrole"},
		State:  roleResource.Outputs{Inputs: roleResource.Inputs{Name: "readrole", Privileges: []string{"VM.PowerMgmt"}}},
	}

	resp, err := role.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "readrole", resp.State.Name)
	// Privileges should be sorted alphabetically by implementation
	assert.Equal(t, []string{"Datastore.Allocate", "VM.PowerMgmt"}, resp.State.Privileges)
}

//nolint:paralleltest // shares env mutation
func TestRoleReadMissingIDWithSeam(t *testing.T) {
	// Override seam to avoid provider config dependency
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return &px.Client{}, nil }

	role := &roleResource.Role{}
	req := infer.ReadRequest[roleResource.Inputs, roleResource.Outputs]{
		ID:    "", // triggers missing ID branch
		State: roleResource.Outputs{Inputs: roleResource.Inputs{Name: "anything"}},
	}
	_, err := role.Read(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing role ID")
}

//nolint:paralleltest // shares env mutation
func TestRoleReadNotFoundMarksDeleted(t *testing.T) {
	mockServer := mocha.New(t)
	mockServer.Start()
	defer func() { _ = mockServer.Close() }()

	// List endpoint returns a different role so target is not found
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles")).
			Reply(reply.OK().BodyString(`{"data": [{"roleid":"other",
			"privs":"VM.PowerMgmt","special":0}]}`)),
	).Enable()

	_ = os.Setenv("PVE_API_URL", mockServer.URL())
	defer func() {
		if err := os.Unsetenv("PVE_API_URL"); err != nil {
			t.Errorf("failed to unset PVE_API_URL: %v", err)
		}
	}()

	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) {
		apiClient := api.NewClient(mockServer.URL(), api.WithAPIToken("user@pve!token", "TOKEN"))
		return &px.Client{Client: apiClient}, nil
	}

	role := &roleResource.Role{}
	req := infer.ReadRequest[roleResource.Inputs, roleResource.Outputs]{
		ID:     "missingrole",
		Inputs: roleResource.Inputs{Name: "missingrole"},
		State:  roleResource.Outputs{Inputs: roleResource.Inputs{Name: "missingrole"}},
	}
	resp, err := role.Read(context.Background(), req)
	require.NoError(t, err)
	// When not found we expect empty response (ID unset) signalling deletion
	assert.Empty(t, resp.ID)
	assert.Equal(t, roleResource.Outputs{}, resp.State)
}

//nolint:paralleltest // env and global seam mutations
func TestRoleUpdateNoChangeNormalRole(t *testing.T) {
	mockServer := mocha.New(t)
	mockServer.Start()
	defer func() { _ = mockServer.Close() }()

	// Non-special role privileges identical
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles")).
			Reply(reply.OK().BodyString(`{"data":[{"roleid":"normal",
			"privs":"Datastore.Allocate,VM.PowerMgmt","special":0}]}`)),
	).Enable()

	_ = os.Setenv("PVE_API_URL", mockServer.URL())
	defer func() { _ = os.Unsetenv("PVE_API_URL") }()

	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) {
		apiClient := api.NewClient(mockServer.URL(), api.WithAPIToken("user@pve!token", "TOKEN"))
		return &px.Client{Client: apiClient}, nil
	}

	r := &roleResource.Role{}
	req := infer.UpdateRequest[roleResource.Inputs, roleResource.Outputs]{
		Inputs: roleResource.Inputs{Name: "normal", Privileges: []string{"Datastore.Allocate", "VM.PowerMgmt"}},
		State: roleResource.Outputs{
			Inputs: roleResource.Inputs{Name: "normal", Privileges: []string{"Datastore.Allocate", "VM.PowerMgmt"}},
		},
	}
	resp, err := r.Update(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, []string{"Datastore.Allocate", "VM.PowerMgmt"}, resp.Output.Privileges)
}

//nolint:paralleltest // env and global seam mutations
func TestRoleUpdateChangeFailure(t *testing.T) {
	mockServer := mocha.New(t)
	mockServer.Start()
	defer func() { _ = mockServer.Close() }()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles")).
			Reply(reply.OK().BodyString(`{"data":[{"roleid":"normal",
			"privs":"VM.PowerMgmt","special":0}]}`)),
		// Simulate backend failure on update
		mocha.Put(expect.URLPath("/access/roles/normal")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError, Body: strings.NewReader(`{"data":null}`)}, nil
			}),
	).Enable()

	_ = os.Setenv("PVE_API_URL", mockServer.URL())
	defer func() { _ = os.Unsetenv("PVE_API_URL") }()

	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) {
		apiClient := api.NewClient(mockServer.URL(), api.WithAPIToken("user@pve!token", "TOKEN"))
		return &px.Client{Client: apiClient}, nil
	}

	r := &roleResource.Role{}
	req := infer.UpdateRequest[roleResource.Inputs, roleResource.Outputs]{
		Inputs: roleResource.Inputs{Name: "normal", Privileges: []string{"Datastore.Allocate", "VM.PowerMgmt"}},
		State:  roleResource.Outputs{Inputs: roleResource.Inputs{Name: "normal", Privileges: []string{"VM.PowerMgmt"}}},
	}
	_, err := r.Update(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update role")
}

//nolint:paralleltest // env and global seam mutations
func TestRoleUpdateGetRoleFailure(t *testing.T) {
	mockServer := mocha.New(t)
	mockServer.Start()
	defer func() { _ = mockServer.Close() }()

	// Simulate list roles endpoint failure -> GetRole should error
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError, Body: strings.NewReader(`{"data":null}`)}, nil
			}),
	).Enable()

	_ = os.Setenv("PVE_API_URL", mockServer.URL())
	defer func() { _ = os.Unsetenv("PVE_API_URL") }()

	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) {
		apiClient := api.NewClient(mockServer.URL(), api.WithAPIToken("user@pve!token", "TOKEN"))
		return &px.Client{Client: apiClient}, nil
	}

	r := &roleResource.Role{}
	req := infer.UpdateRequest[roleResource.Inputs, roleResource.Outputs]{
		Inputs: roleResource.Inputs{Name: "normal", Privileges: []string{"VM.PowerMgmt"}},
		State:  roleResource.Outputs{Inputs: roleResource.Inputs{Name: "normal", Privileges: []string{"VM.PowerMgmt"}}},
	}
	_, err := r.Update(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get role")
}

//nolint:paralleltest // env and global seam mutations
func TestRoleUpdateClientAcquisitionFailure(t *testing.T) {
	// Override seam to return error
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, assert.AnError }

	r := &roleResource.Role{}
	req := infer.UpdateRequest[roleResource.Inputs, roleResource.Outputs]{
		Inputs: roleResource.Inputs{Name: "normal", Privileges: []string{"VM.PowerMgmt"}},
		State:  roleResource.Outputs{Inputs: roleResource.Inputs{Name: "normal", Privileges: []string{"VM.PowerMgmt"}}},
	}
	_, err := r.Update(context.Background(), req)
	require.Error(t, err)
}

// //nolint:paralleltest // env and seam mutation
func TestRoleDeleteClientAcquisitionFailure(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, assert.AnError }

	r := &roleResource.Role{}
	req := infer.DeleteRequest[roleResource.Outputs]{
		State: roleResource.Outputs{Inputs: roleResource.Inputs{Name: "deleterole"}},
	}
	_, err := r.Delete(context.Background(), req)
	require.Error(t, err)
}

//nolint:paralleltest // env and seam mutation
func TestRoleDeleteGetRoleListError(t *testing.T) {
	mockServer := mocha.New(t)
	mockServer.Start()
	defer func() { _ = mockServer.Close() }()

	// List returns 500
	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError, Body: strings.NewReader(`{"data":null}`)}, nil
			}),
	).Enable()

	_ = os.Setenv("PVE_API_URL", mockServer.URL())
	defer func() { _ = os.Unsetenv("PVE_API_URL") }()

	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) {
		apiClient := api.NewClient(mockServer.URL(), api.WithAPIToken("user@pve!token", "TOKEN"))
		return &px.Client{Client: apiClient}, nil
	}

	r := &roleResource.Role{}
	req := infer.DeleteRequest[roleResource.Outputs]{
		State: roleResource.Outputs{Inputs: roleResource.Inputs{Name: "any"}},
	}
	_, err := r.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get role")
}

//nolint:paralleltest // env and seam mutation
func TestRoleDeleteBackendFailure(t *testing.T) {
	mockServer := mocha.New(t)
	mockServer.Start()
	defer func() { _ = mockServer.Close() }()

	mockServer.AddMocks(
		mocha.Get(expect.URLPath("/access/roles")).
			Reply(reply.OK().BodyString(`{"data":[{"roleid":"deleterole",
			"privs":"VM.PowerMgmt","special":0}]}`)),
		mocha.Delete(expect.URLPath("/access/roles/deleterole")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError, Body: strings.NewReader(`{"data":null}`)}, nil
			}),
	).Enable()

	_ = os.Setenv("PVE_API_URL", mockServer.URL())
	defer func() { _ = os.Unsetenv("PVE_API_URL") }()

	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) {
		apiClient := api.NewClient(mockServer.URL(), api.WithAPIToken("user@pve!token", "TOKEN"))
		return &px.Client{Client: apiClient}, nil
	}

	r := &roleResource.Role{}
	req := infer.DeleteRequest[roleResource.Outputs]{
		State: roleResource.Outputs{Inputs: roleResource.Inputs{Name: "deleterole"}},
	}
	_, err := r.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete role")
}
