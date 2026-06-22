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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

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

// MockProxmoxClient is a test double for proxmox.Client.
// ResolveNode returns the provided node name or the DefaultNode fallback.
// NextVMID returns DefaultVMID.
type MockProxmoxClient struct {
	DefaultNode string
	DefaultVMID int
}

var _ proxmox.Client = (*MockProxmoxClient)(nil)

// Get implements proxmox.Client.
func (m *MockProxmoxClient) Get(_ context.Context, _ string, _ any) error { return nil }

// Post implements proxmox.Client.
func (m *MockProxmoxClient) Post(_ context.Context, _ string, _, _ any) error { return nil }

// Put implements proxmox.Client.
func (m *MockProxmoxClient) Put(_ context.Context, _ string, _, _ any) error { return nil }

// Delete implements proxmox.Client.
func (m *MockProxmoxClient) Delete(_ context.Context, _ string, _ any) error { return nil }

// ResolveNode implements proxmox.Client.
func (m *MockProxmoxClient) ResolveNode(_ context.Context, node *string) (string, error) {
	if node != nil {
		return *node, nil
	}
	return m.DefaultNode, nil
}

// NextVMID implements proxmox.Client.
func (m *MockProxmoxClient) NextVMID(_ context.Context) (int, error) {
	return m.DefaultVMID, nil
}

// WaitForTask implements proxmox.Client.
func (m *MockProxmoxClient) WaitForTask(_ context.Context, _ string, _, _ time.Duration) error {
	return nil
}
