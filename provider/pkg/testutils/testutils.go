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

// Package testutils provides common test utilities and helpers for the provider tests.
package testutils

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/require"
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

// Ptr creates a pointer to any value.
func Ptr[T any](v T) *T {
	return &v
}

// CreateMockServer creates a test HTTP server that captures requests and returns mock responses
func CreateMockServer(
	t *testing.T,
	handler func(w http.ResponseWriter, r *http.Request, captured *MockRequest),
) (*httptest.Server, *MockRequest) {
	t.Helper()
	var captured MockRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture request details
		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.Headers = r.Header.Clone()
		captured.QueryParams = r.URL.Query()

		// Read body
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			captured.Body = string(bodyBytes)
		}

		// Call handler
		handler(w, r, &captured)
	}))

	return server, &captured
}

// MockRequest captures information about HTTP requests for verification
type MockRequest struct {
	Method      string
	Path        string
	Body        string
	Headers     http.Header
	QueryParams map[string][]string
}
