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

package resources

import (
	"context"
	"os"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"
	"github.com/vitorsalgado/mocha/v3"
)

// NewAPIMock starts a mocha mock server, sets PVE_API_URL, and overrides the global
// Proxmox client factory. It returns the mock instance and a cleanup function.
// Not safe for parallel tests due to global/env mutation.
func NewAPIMock(
	t *testing.T,
) (mock *mocha.Mocha, cleanup func()) {
	// helper defined outside *_test.go for cross-package reuse; needs t.Helper for clearer failures
	t.Helper()
	mock = mocha.New(t)
	mock.Start()

	_ = os.Setenv("PVE_API_URL", mock.URL())

	orig := client.GetProxmoxClientFn
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) {
		c := api.NewClient(os.Getenv("PVE_API_URL"), api.WithAPIToken("user@pve!token", "TOKEN"))
		return &px.Client{Client: c}, nil
	}

	cleanup = func() {
		client.GetProxmoxClientFn = orig
		_ = os.Unsetenv("PVE_API_URL")
		_ = mock.Close()
	}
	return
}

// OverrideClient overrides the global client factory without touching environment variables.
// Useful for tests that exercise early validation paths and won't hit the network.
func OverrideClient(
	t *testing.T,
	apiURL string,
) (cleanup func()) {
	// central shared test helper; uses t.Helper for better stack trace
	t.Helper()
	orig := client.GetProxmoxClientFn
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) {
		c := api.NewClient(apiURL, api.WithAPIToken("u", "t"))
		return &px.Client{Client: c}, nil
	}
	cleanup = func() { client.GetProxmoxClientFn = orig }
	return
}
