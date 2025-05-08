package pool_test

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/params"
	"github.com/vitorsalgado/mocha/v3/reply"
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

func TestPoolCreate(t *testing.T) {
	// Start the mock server
	mockServer := mocha.New(t)
	mockServer.Start()
	defer mockServer.Close()

	getPool := mockServer.AddMocks(
		mocha.Get(expect.URLPath("/pools/testpool")).Repeat(2).ReplyFunction(func(request *http.Request, m reply.M, p params.P) (*reply.Response, error) {
			r := strings.NewReader(`{
            "data": {
                "poolid": "testpool",
                "comment": "updated comment" ,
                "guests": []
              }
          }`)
			return &reply.Response{
				Status: http.StatusOK,
				Body:   r,
			}, nil
		}))

	createPool := mockServer.AddMocks(
		mocha.Post(expect.URLPath("/pools")).Reply(reply.Created().BodyString(`{
            "data": {
                "poolid": "testpool",
                "name": "testpool",
                "comment": "comment",
                "guests": []
            }
        }`)).PostAction(&toggleMocksPostAction{toEnable: []*mocha.Scoped{getPool}}),
	)

	deletePool := mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/pools/testpool")).Reply(
			reply.OK()))

	updatePool := mockServer.AddMocks(
		mocha.Put(expect.URLPath("/pools/testpool")).Reply(reply.OK().BodyString(`{
            "data": {
                "name": "testpool",
                "comment": "updated comment",
                "guests": []
  }
    }`)))

	// Enable initial state
	createPool.Enable()
	deletePool.Enable()
	getPool.Enable()
	updatePool.Enable()

	// Set environment variable to direct Proxmox API requests to the mock server
	_ = os.Setenv("PVE_API_URL", mockServer.URL())
	defer os.Unsetenv("PVE_API_URL")

	// Start the integration server with the mock setup
	server := integration.NewServer("pve", semver.Version{Minor: 1}, provider.Provider())

	// Run the integration lifecycle tests
	integration.LifeCycleTest{
		Resource: "pve:pool:Pool",
		Create: integration.Operation{
			Inputs: presource.NewPropertyMapFromMap(map[string]interface{}{
				"name":    "testpool",
				"comment": "test pool comment",
			}),
			Hook: func(inputs, output presource.PropertyMap) {
				t.Logf("Outputs after Create: %v", output)
				assert.Equal(t, "testpool", output["name"].StringValue())
			},
		},
		Updates: []integration.Operation{
			{
				Inputs: presource.NewPropertyMapFromMap(map[string]interface{}{
					"name":    "testpool",
					"comment": "updated comment",
				}),
				ExpectedOutput: presource.NewPropertyMapFromMap(map[string]interface{}{
					"name":    "testpool",
					"comment": "updated comment",
				}),
			},
		},
	}.Run(t, server)
}
