package group_test

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/params"
	"github.com/vitorsalgado/mocha/v3/reply"

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
func TestGroupCreate(t *testing.T) {
	// Start the mock server
	mockServer := mocha.New(t)
	mockServer.Start()
	defer func() {
		if err := mockServer.Close(); err != nil {
			t.Errorf("failed to close mock server: %v", err)
		}
	}()

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

// //nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
// func TestGroupCreateNewGroupError(t *testing.T) {
// 	// Start the mock server
// 	mockServer := mocha.New(t)
// 	mockServer.Start()
// 	defer func() {
// 		if err := mockServer.Close(); err != nil {
// 			t.Errorf("failed to close mock server: %v", err)
// 		}
// 	}()

// 	// // Mock the authentication endpoint to succeed (needed for client setup)
// 	// authMock := mockServer.AddMocks(
// 	// 	mocha.Post(expect.URLPath("/access/ticket")).Reply(
// 	// 		reply.OK().BodyString(`{
// 	//             "data": {
// 	//                 "ticket": "mock-ticket",
// 	//                 "CSRFPreventionToken": "mock-csrf-token"
// 	//             }
// 	//         }`)),
// 	// )

// 	// Mock the POST /access/groups endpoint to return a 400 Bad Request
// 	// This should cause pxc.NewGroup() to return an error
// 	createGroupError := mockServer.AddMocks(
// 		mocha.Post(expect.URLPath("/access/groups")).Reply(
// 			reply.BadRequest().BodyString(`{
//                 "errors": {
//                     "groupid": "parameter verification failed - group 'testgroup' already exists"
//                 }
//             }`)),
// 	)

// 	// Enable both mocks
// 	createGroupError.Enable()

// 	// Set environment variable to direct Proxmox API requests to the mock server
// 	_ = os.Setenv("PVE_API_URL", mockServer.URL())
// 	defer func() {
// 		if err := os.Unsetenv("PVE_API_URL"); err != nil {
// 			t.Errorf("failed to unset PVE_API_URL: %v", err)
// 		}
// 	}()

// 	// Start the integration server
// 	server, err := integration.NewServer(
// 		t.Context(),
// 		provider.Name,
// 		semver.Version{Minor: 1},
// 		integration.WithProvider(provider.NewProvider()),
// 	)
// 	require.NoError(t, err)

// 	// Test create operation that should fail
// 	integration.LifeCycleTest{
// 		Resource: "pve:group:Group",
// 		Create: integration.Operation{
// 			Inputs: property.NewMap(map[string]property.Value{
// 				"name":    property.New("testgroup"),
// 				"comment": property.New("test group comment"),
// 			}),
// 			ExpectFailure: true,
// 		},
// 	}.Run(t, server)
// }
