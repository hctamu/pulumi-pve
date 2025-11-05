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

// Package ha_test contains comprehensive tests for HA resource operations.
//
// Test Organization:
// - Lifecycle tests: Full create/update/delete cycles using integration.LifeCycleTest
// - Unit tests: Direct method calls with mocked backends (Create, Read, Update, Delete, Diff)
// - Error scenarios: API failures, client acquisition errors, edge cases
//
// Note: Most tests disable paralleltest due to shared environment setup and client seam overrides.
// Only pure unit tests (like Diff) can safely run in parallel.
package ha_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	utils "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources"
	haResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/ha"
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

// Common test constants
const (
	apiPathHAResources = "/cluster/ha/resources/"

	stateStarted = "started"
	stateStopped = "stopped"

	group1 = "grp1"
	group2 = "grp2"

	resourceName = "test-ha"

	id100      = "100"
	id100Float = 100.0
	id200      = "200"
	id200Float = 200.0

	clientError = "failed to get Proxmox client: assert.AnError general error for testing"
)

// toggleMocksPostAction mirrors helper pattern from other resource tests.
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

//nolint:paralleltest // uses global env + client seam
func TestHaHealthyLifeCycle(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// GET resource used by Read operations (after create + update + refreshes)
	// We'll return different state once update applied.
	getCount := 0
	getHA := mock.AddMocks(
		mocha.Get(expect.URLPath(apiPathHAResources + id100)).
			Repeat(4). // create read + update read + refresh etc
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				getCount++
				state := stateStarted
				group := group1
				if getCount >= 2 { // after update show new state/group
					state = stateStopped
					group = group2
				}
				return &reply.Response{
					Status: http.StatusOK,
					Body:   strings.NewReader(`{"data":{"sid":"` + id100 + `","state":"` + state + `","group":"` + group + `"}}`),
				}, nil
			}),
	)

	createHA := mock.AddMocks(
		mocha.Post(expect.URLPath(apiPathHAResources)).
			Reply(reply.Created().BodyString(
				`{"data":{"sid":"` + id100 + `","state":"` + stateStarted + `","group":"` + group1 + `"}}`,
			)).
			PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{getHA}}),
	)

	updateHA := mock.AddMocks(
		mocha.Put(expect.URLPath(apiPathHAResources + id100)).
			Reply(reply.OK().BodyString(
				`{"data":{"sid":"` + id100 + `","state":"` + stateStopped + `","group":"` + group2 + `"}}`,
			)),
	)

	deleteHA := mock.AddMocks(
		mocha.Delete(expect.URLPath(apiPathHAResources + id100)).Reply(reply.OK()),
	)

	// enable initial mocks
	createHA.Enable()
	getHA.Enable()
	updateHA.Enable()
	deleteHA.Enable()

	server, err := integration.NewServer(
		t.Context(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProvider()),
	)
	require.NoError(t, err)

	// expected output after update
	expected := property.NewMap(map[string]property.Value{
		"group":      property.New(group2),
		"state":      property.New(stateStopped),
		"resourceId": property.New(id100Float),
	})

	integration.LifeCycleTest{
		Resource: "pve:ha:Ha",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New(group1),
				"state":      property.New(stateStarted),
				"resourceId": property.New(id100Float),
			}),
			Hook: func(in, out property.Map) {
				assert.Equal(t, id100Float, out.Get("resourceId").AsNumber())
				assert.Equal(t, group1, out.Get("group").AsString())
				assert.Equal(t, stateStarted, out.Get("state").AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New(group2),
				"state":      property.New(stateStopped),
				"resourceId": property.New(id100Float),
			}),
			ExpectedOutput: &expected,
		}},
	}.Run(t, server)
}

//nolint:paralleltest // env + seam
func TestHaCreateSuccess(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Mock successful POST to create HA resource
	mock.AddMocks(
		mocha.Post(expect.URLPath(apiPathHAResources)).
			Reply(reply.Created().BodyString(
				`{"data":{"sid":"` + id100 + `","state":"` + stateStarted + `","group":"` + group1 + `"}}`,
			)),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.CreateRequest[haResource.Inputs]{
		Name:   resourceName,
		Inputs: haResource.Inputs{ResourceID: id100Float, Group: group1, State: haResource.StateStarted},
	}
	resp, err := ha.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, resourceName, resp.ID)
	assert.Equal(t, int(id100Float), resp.Output.ResourceID)
	assert.Equal(t, group1, resp.Output.Group)
	assert.Equal(t, haResource.StateStarted, resp.Output.State)
}

//nolint:paralleltest // env + seam
func TestHaCreateBackendFailure(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Mock API returning 500 error to test error handling
	mock.AddMocks(
		mocha.Post(expect.URLPath(apiPathHAResources)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError}, nil
			}),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.CreateRequest[haResource.Inputs]{
		Name:   resourceName,
		Inputs: haResource.Inputs{ResourceID: id100Float, Group: group1, State: haResource.StateStarted},
	}
	_, err := ha.Create(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "failed to create HA resource 500 Internal Server Error ", err.Error())
}

//nolint:paralleltest // uses seam override
func TestHaCreateClientAcquisitionFailure(t *testing.T) {
	// Override the client acquisition function to simulate failure
	orig := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = orig }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, assert.AnError }

	ha := &haResource.Ha{}
	req := infer.CreateRequest[haResource.Inputs]{
		Name:   resourceName,
		Inputs: haResource.Inputs{ResourceID: id100Float, Group: group1, State: haResource.StateStarted},
	}
	_, err := ha.Create(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, clientError, err.Error())
}

//nolint:paralleltest // uses env + seam
func TestHaReadSuccess(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Get(expect.URLPath(apiPathHAResources + id100)).
			Reply(reply.OK().BodyString(
				`{"data":{"sid":"` + id100 + `","state":"` + stateStarted + `","group":"` + group1 + `"}}`,
			)),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.ReadRequest[haResource.Inputs, haResource.Outputs]{
		ID:     id100,
		Inputs: haResource.Inputs{ResourceID: id100Float},
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: id100Float}},
	}
	resp, err := ha.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int(id100Float), resp.State.ResourceID)
	assert.Equal(t, group1, resp.State.Group)
	assert.Equal(t, haResource.State(stateStarted), resp.State.State)
}

//nolint:paralleltest // uses env + seam
func TestHaReadFailure(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Get(expect.URLPath(apiPathHAResources + id100)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError}, nil
			}),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.ReadRequest[haResource.Inputs, haResource.Outputs]{
		ID:     id100,
		Inputs: haResource.Inputs{ResourceID: id100Float},
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: id100Float}},
	}
	_, err := ha.Read(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "failed to get HA resource 500 Internal Server Error ", err.Error())
}

//nolint:paralleltest // uses seam override
func TestHaReadClientAcquisitionFailure(t *testing.T) {
	orig := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = orig }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, assert.AnError }

	ha := &haResource.Ha{}
	req := infer.ReadRequest[haResource.Inputs, haResource.Outputs]{
		ID:     id100,
		Inputs: haResource.Inputs{ResourceID: id100Float},
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: id100Float}},
	}
	_, err := ha.Read(context.Background(), req)
	assert.Equal(t, clientError, err.Error())
}

//nolint:paralleltest // uses env + seam
func TestHaUpdateClientAcquisitionFailure(t *testing.T) {
	orig := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = orig }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, assert.AnError }

	ha := &haResource.Ha{}
	req := infer.UpdateRequest[haResource.Inputs, haResource.Outputs]{
		ID:     id100,
		Inputs: haResource.Inputs{ResourceID: id100Float, State: haResource.StateStarted},
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: id100Float, State: haResource.StateStopped}},
	}
	_, err := ha.Update(context.Background(), req)
	assert.Equal(t, clientError, err.Error())
}

//nolint:paralleltest // env + seam
func TestHaUpdateBackendFailure(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Mock PUT request returning 500 error to test update error handling
	mock.AddMocks(
		mocha.Put(expect.URLPath(apiPathHAResources + id100)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError}, nil
			}),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.UpdateRequest[haResource.Inputs, haResource.Outputs]{
		ID:     id100,
		Inputs: haResource.Inputs{ResourceID: id100Float, State: haResource.StateStopped, Group: group2},
		State: haResource.Outputs{
			Inputs: haResource.Inputs{ResourceID: id100Float, State: haResource.StateStarted, Group: group1},
		},
	}
	_, err := ha.Update(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "failed to update HA resource 500 Internal Server Error ", err.Error())
}

//nolint:paralleltest // env + seam
func TestHaUpdateRemoveGroupSuccess(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Mock successful PUT request when removing group field
	// When group is cleared (empty string), the API should receive a "delete" field
	mock.AddMocks(
		mocha.Put(expect.URLPath(apiPathHAResources + id100)).Reply(reply.OK()),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.UpdateRequest[haResource.Inputs, haResource.Outputs]{
		ID:     id100,
		Inputs: haResource.Inputs{ResourceID: id100Float, State: haResource.StateStarted, Group: ""}, // clearing group
		State: haResource.Outputs{
			Inputs: haResource.Inputs{ResourceID: id100Float, State: haResource.StateStarted, Group: group1},
		},
	}
	resp, err := ha.Update(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Output.Group)
}

//nolint:paralleltest // env + seam
func TestHaDeleteSuccess(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Delete(expect.URLPath(apiPathHAResources + id100)).Reply(reply.OK()),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.DeleteRequest[haResource.Outputs]{
		ID:    id100,
		State: haResource.Outputs{Inputs: haResource.Inputs{ResourceID: id100Float}},
	}
	_, err := ha.Delete(context.Background(), req)
	require.NoError(t, err)
}

//nolint:paralleltest // env + seam
func TestHaDeleteBackendFailure(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Delete(expect.URLPath(apiPathHAResources + id100)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError}, nil
			}),
	).Enable()

	h := &haResource.Ha{}
	req := infer.DeleteRequest[haResource.Outputs]{
		ID:    id100,
		State: haResource.Outputs{Inputs: haResource.Inputs{ResourceID: id100Float}},
	}
	_, err := h.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, "failed to delete HA resource 500 Internal Server Error ", err.Error())
}

// Edge case tests

//nolint:paralleltest // env + seam
func TestHaCreateWithoutGroup(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Test creating HA resource without optional group field
	mock.AddMocks(
		mocha.Post(expect.URLPath(apiPathHAResources)).
			Reply(reply.Created().BodyString(`{"data":{"sid":"` + id100 + `","state":"` + stateStarted + `"}}`)),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.CreateRequest[haResource.Inputs]{
		Name:   resourceName,
		Inputs: haResource.Inputs{ResourceID: id100Float, State: haResource.StateStarted}, // No group
	}
	resp, err := ha.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int(id100Float), resp.Output.ResourceID)
	assert.Empty(t, resp.Output.Group)
	assert.Equal(t, haResource.StateStarted, resp.Output.State)
}

//nolint:paralleltest // uses global env + client seam
func TestHaResourceIDChangeTriggersReplace(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()

	// Track the order of operations to verify replace behavior
	// When ResourceID changes, Pulumi should replace the resource (delete old, create new)
	var operations []string
	var mu sync.Mutex

	recordOp := func(op string) {
		mu.Lock()
		defer mu.Unlock()
		operations = append(operations, op)
	}

	// Initial GET for resource 100 (after create)
	getHA100 := mock.AddMocks(
		mocha.Get(expect.URLPath(apiPathHAResources + id100)).
			Repeat(2). // After initial create
			Reply(reply.OK().BodyString(
				`{"data":{"sid":"` + id100 + `","state":"` + stateStarted + `","group":"` + group1 + `"}}`,
			)),
	)

	// GET for new resource 200 (after replace create)
	getHA200 := mock.AddMocks(
		mocha.Get(expect.URLPath(apiPathHAResources + id200)).
			Repeat(2).
			Reply(reply.OK().BodyString(
				`{"data":{"sid":"` + id200 + `","state":"` + stateStarted + `","group":"` + group1 + `"}}`,
			)),
	)

	// Count POST calls to distinguish between create-100 and create-200
	postCallCount := 0

	// Handle both POST requests (create-100 and create-200)
	// First POST creates resource 100, second POST creates resource 200 (the replacement)
	createHA := mock.AddMocks(
		mocha.Post(expect.URLPath(apiPathHAResources)).
			Repeat(2). // Will be called twice: once for 100, once for 200
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				postCallCount++
				if postCallCount == 1 {
					recordOp("create-100")
					// Enable GET for resource 100
					getHA100.Enable()
					return &reply.Response{
						Status: http.StatusCreated,
						Body: strings.NewReader(
							`{"data":{"sid":"` + id100 + `","state":"` + stateStarted + `","group":"` + group1 + `"}}`,
						),
					}, nil
				}
				// Second POST is for resource 200
				recordOp("create-200")
				// Enable GET for resource 200
				getHA200.Enable()
				return &reply.Response{
					Status: http.StatusCreated,
					Body: strings.NewReader(
						`{"data":{"sid":"` + id200 + `","state":"` + stateStarted + `","group":"` + group1 + `"}}`,
					),
				}, nil
			}),
	)

	// Delete old resource 100 (happens after creating 200 in current implementation)
	deleteHA100 := mock.AddMocks(
		mocha.Delete(expect.URLPath(apiPathHAResources + id100)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				recordOp("delete-100")
				getHA100.Disable()
				return &reply.Response{Status: http.StatusOK}, nil
			}),
	)

	// Final delete of resource 200 (test cleanup)
	deleteHA200 := mock.AddMocks(
		mocha.Delete(expect.URLPath(apiPathHAResources + id200)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				recordOp("delete-200")
				return &reply.Response{Status: http.StatusOK}, nil
			}),
	)

	// Enable mocks
	createHA.Enable()
	deleteHA100.Enable()
	deleteHA200.Enable()

	server, err := integration.NewServer(
		t.Context(),
		provider.Name,
		semver.Version{Minor: 1},
		integration.WithProvider(provider.NewProvider()),
	)
	require.NoError(t, err)

	expectedAfterReplace := property.NewMap(map[string]property.Value{
		"group":      property.New(group1),
		"state":      property.New(stateStarted),
		"resourceId": property.New(id200Float),
	})

	integration.LifeCycleTest{
		Resource: "pve:ha:Ha",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New(group1),
				"state":      property.New(stateStarted),
				"resourceId": property.New(id100Float),
			}),
			Hook: func(in, out property.Map) {
				assert.Equal(t, id100Float, out.Get("resourceId").AsNumber())
			},
		},
		Updates: []integration.Operation{{
			// Change ResourceID - should trigger replace
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New(group1),
				"state":      property.New(stateStarted),
				"resourceId": property.New(id200Float), // Changed from 100 to 200
			}),
			ExpectedOutput: &expectedAfterReplace,
		}},
	}.Run(t, server)

	// Verify operations happened and resource was replaced
	mu.Lock()
	defer mu.Unlock()

	// Should have: create-100, create-200 (replacement), delete-100, delete-200 (cleanup)
	// Note: Currently using create-before-delete strategy (not delete-before-replace)
	// This is because the `provider:"replaceOnChanges"` tag requires a Diff method
	// to properly signal DeleteBeforeReplace behavior to Pulumi.
	require.GreaterOrEqual(t, len(operations), 4, "Should have all 4 operations")

	// Find indices of key operations
	create100Idx, delete100Idx, create200Idx := -1, -1, -1
	for i, op := range operations {
		switch op {
		case "create-100":
			if create100Idx == -1 {
				create100Idx = i
			}
		case "delete-100":
			delete100Idx = i
		case "create-200":
			create200Idx = i
		}
	}

	// Verify replacement happened (both creates and deletes occurred)
	assert.NotEqual(t, -1, create100Idx, "Initial create-100 should have happened")
	assert.NotEqual(t, -1, create200Idx, "Replacement create-200 should have happened")
	assert.NotEqual(t, -1, delete100Idx, "Delete-100 should have happened after replacement")

	// Verify that ResourceID change triggered a replace (new resource was created)
	assert.Less(t, create100Idx, create200Idx, "Original resource should be created before replacement")
	assert.Less(t, create200Idx, delete100Idx, "New resource created before old one deleted (create-before-delete)")
}

func TestHaStateValidation(t *testing.T) {
	t.Parallel()

	// Test state validation for all valid and invalid state values
	tests := []struct {
		name      string
		state     haResource.State
		wantError bool
	}{
		{"valid ignored", haResource.StateIgnored, false},
		{"valid started", haResource.StateStarted, false},
		{"valid stopped", haResource.StateStopped, false},
		{"invalid state", haResource.State("invalid"), true},
		{"empty state", haResource.State(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.state.ValidateState(context.Background())
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
