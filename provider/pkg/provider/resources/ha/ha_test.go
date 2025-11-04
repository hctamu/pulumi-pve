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

package ha_test

import (
	"context"
	"net/http"
	"strings"
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

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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
		mocha.Get(expect.URLPath("/cluster/ha/resources/100")).
			Repeat(4). // create read + update read + refresh etc
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				getCount++
				state := "started"
				group := "grp1"
				if getCount >= 2 { // after update show new state/group
					state = "stopped"
					group = "grp2"
				}
				return &reply.Response{
					Status: http.StatusOK,
					Body:   strings.NewReader(`{"data":{"sid":"100","state":"` + state + `","group":"` + group + `"}}`),
				}, nil
			}),
	)

	createHA := mock.AddMocks(
		mocha.Post(expect.URLPath("/cluster/ha/resources/")).
			Reply(reply.Created().BodyString(`{"data":{"sid":"100","state":"started","group":"grp1"}}`)).
			PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{getHA}}),
	)

	updateHA := mock.AddMocks(
		mocha.Put(expect.URLPath("/cluster/ha/resources/100")).
			Reply(reply.OK().BodyString(`{"data":{"sid":"100","state":"stopped","group":"grp2"}}`)),
	)

	deleteHA := mock.AddMocks(
		mocha.Delete(expect.URLPath("/cluster/ha/resources/100")).Reply(reply.OK()),
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
		"group":      property.New("grp2"),
		"state":      property.New("stopped"),
		"resourceId": property.New(100.0),
	})

	integration.LifeCycleTest{
		Resource: "pve:ha:Ha",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New("grp1"),
				"state":      property.New("started"), // explicit though default
				"resourceId": property.New(100.0),
			}),
			Hook: func(in, out property.Map) { assert.True(t, out.Get("resourceId").IsNumber()) },
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"group":      property.New("grp2"),
				"state":      property.New("stopped"),
				"resourceId": property.New(100.0),
			}),
			ExpectedOutput: &expected,
		}},
	}.Run(t, server)
}

//nolint:paralleltest // uses env + seam
func TestHaReadSuccess(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/ha/resources/42")).
			Reply(reply.OK().BodyString(`{"data":{"sid":"42","state":"started","group":"g"}}`)),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.ReadRequest[haResource.Inputs, haResource.Outputs]{
		ID:     "ha-42",
		Inputs: haResource.Inputs{ResourceID: 42},
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: 42}},
	}
	resp, err := ha.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "g", resp.State.Group)
	assert.Equal(t, haResource.State("started"), resp.State.State)
}

//nolint:paralleltest // uses env + seam
func TestHaReadFailure(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/ha/resources/55")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError}, nil
			}),
	).Enable()

	ha := &haResource.Ha{}
	req := infer.ReadRequest[haResource.Inputs, haResource.Outputs]{
		ID:     "ha-55",
		Inputs: haResource.Inputs{ResourceID: 55},
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: 55}},
	}
	_, err := ha.Read(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get HA resource")
}

//nolint:paralleltest // seam override
func TestHaUpdateClientAcquisitionFailure(t *testing.T) {
	orig := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = orig }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, assert.AnError }

	h := &haResource.Ha{}
	req := infer.UpdateRequest[haResource.Inputs, haResource.Outputs]{
		ID:     "ha-1",
		Inputs: haResource.Inputs{ResourceID: 1, State: haResource.StateEnabled},
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: 1, State: haResource.StateDisabled}},
	}
	_, err := h.Update(context.Background(), req)
	require.Error(t, err)
}

//nolint:paralleltest // env + seam
func TestHaUpdateBackendFailure(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Put(expect.URLPath("/cluster/ha/resources/77")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError}, nil
			}),
	).Enable()

	h := &haResource.Ha{}
	req := infer.UpdateRequest[haResource.Inputs, haResource.Outputs]{
		ID:     "ha-77",
		Inputs: haResource.Inputs{ResourceID: 77, State: haResource.StateDisabled, Group: "g2"},
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: 77, State: haResource.StateEnabled, Group: "g1"}},
	}
	_, err := h.Update(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update HA resource")
}

//nolint:paralleltest // env + seam
func TestHaUpdateRemoveGroupSuccess(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Put(expect.URLPath("/cluster/ha/resources/5")).Reply(reply.OK()),
	).Enable()

	h := &haResource.Ha{}
	req := infer.UpdateRequest[haResource.Inputs, haResource.Outputs]{
		ID:     "ha-5",
		Inputs: haResource.Inputs{ResourceID: 5, State: haResource.StateDisabled, Group: ""}, // clearing group
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: 5, State: haResource.StateDisabled, Group: "old"}},
	}
	resp, err := h.Update(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Output.Group)
}

//nolint:paralleltest // env + seam
func TestHaDeleteSuccess(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Delete(expect.URLPath("/cluster/ha/resources/9")).Reply(reply.OK()),
	).Enable()

	h := &haResource.Ha{}
	req := infer.DeleteRequest[haResource.Outputs]{
		ID:    "ha-9",
		State: haResource.Outputs{Inputs: haResource.Inputs{ResourceID: 9}},
	}
	_, err := h.Delete(context.Background(), req)
	require.NoError(t, err)
}

//nolint:paralleltest // env + seam
func TestHaDeleteBackendFailure(t *testing.T) {
	mock, cleanup := utils.NewAPIMock(t)
	defer cleanup()
	mock.AddMocks(
		mocha.Delete(expect.URLPath("/cluster/ha/resources/11")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusInternalServerError}, nil
			}),
	).Enable()

	h := &haResource.Ha{}
	req := infer.DeleteRequest[haResource.Outputs]{
		ID:    "ha-11",
		State: haResource.Outputs{Inputs: haResource.Inputs{ResourceID: 11}},
	}
	_, err := h.Delete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete HA resource")
}

// Unit tests for Diff logic (no server needed)
func TestHaDiff(t *testing.T) {
	t.Parallel()
	h := &haResource.Ha{}
	resp, err := h.Diff(context.Background(), infer.DiffRequest[haResource.Inputs, haResource.Outputs]{
		ID:     "ha-1",
		Inputs: haResource.Inputs{ResourceID: 2, Group: "g2", State: haResource.StateDisabled},
		State:  haResource.Outputs{Inputs: haResource.Inputs{ResourceID: 1, Group: "g1", State: haResource.StateEnabled}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	// resourceId replace, group update, state update
	assert.Equal(t, 3, len(resp.DetailedDiff))
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["resourceId"].Kind)
}
