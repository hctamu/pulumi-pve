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

package adapters_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

func TestSDNAdapterLock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		response   string
		expectErr  bool
		errMsg     string
		expected   string
		timeout    time.Duration
	}{
		{
			name:       "successful lock returns token",
			statusCode: http.StatusOK,
			response:   `{"data":"d5fcc91e-5f0b-4fe6-9f4f-f2b4f0d8f4f9"}`,
			expectErr:  false,
			expected:   "d5fcc91e-5f0b-4fe6-9f4f-f2b4f0d8f4f9",
			timeout:    60 * time.Second,
		},
		{
			name:       "API error on lock",
			statusCode: http.StatusInternalServerError,
			response:   `{"errors":"could not acquire lock"}`,
			expectErr:  true,
			errMsg:     "failed to acquire SDN lock",
			timeout:    time.Nanosecond,
		},
		{
			name:       "empty token is rejected",
			statusCode: http.StatusOK,
			response:   `{"data":""}`,
			expectErr:  true,
			errMsg:     "empty lock token returned",
			timeout:    time.Nanosecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, captured := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")

					if r.Method == http.MethodPost && r.URL.Path == "/cluster/sdn/lock" {
						w.WriteHeader(tt.statusCode)
						_, _ = w.Write([]byte(tt.response))
						return
					}

					w.WriteHeader(http.StatusNotFound)
				},
			)
			defer server.Close()

			cfg := &config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			}

			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			sdnAdapter := adapters.NewSDNAdapter(proxmoxAdapter)
			token, err := sdnAdapter.Lock(context.Background(), tt.timeout, false)

			assert.Equal(t, http.MethodPost, captured.Method)
			assert.Equal(t, "/cluster/sdn/lock", captured.Path)
			assert.Equal(t, "", captured.Body)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, token)
		})
	}
}

func TestSDNAdapterApply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		response   string
		expectErr  bool
		errMsg     string
	}{
		{
			name:       "successful apply returns UPID and waits for task",
			statusCode: http.StatusOK,
			response:   `{"data":"UPID:node1:00000000:00000000:00000001:sdn:undefined:root@pam:"}`,
			expectErr:  false,
		},
		{
			name:       "API error on apply",
			statusCode: http.StatusInternalServerError,
			response:   `{"errors": "apply failed"}`,
			expectErr:  true,
			errMsg:     "failed to apply SDN changes",
		},
		{
			name:       "bad request error",
			statusCode: http.StatusBadRequest,
			response:   `{"errors": "invalid parameters"}`,
			expectErr:  true,
			errMsg:     "failed to apply SDN changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")

					// Handle the initial SDN apply request
					if r.Method == http.MethodPut && r.URL.Path == "/cluster/sdn" {
						w.WriteHeader(tt.statusCode)
						_, _ = w.Write([]byte(tt.response))
						return
					}

					// Handle task status polling for successful applies
					if r.Method == http.MethodGet && tt.statusCode == http.StatusOK {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(
							[]byte(
								`{"data":{"upid":"UPID:node1:00000000:00000000:00000001:sdn:undefined:root@pam:",` +
									`"status":"stopped","exitstatus":"OK"}}`,
							),
						)
						return
					}

					// Unexpected request
					w.WriteHeader(http.StatusNotFound)
				},
			)
			defer server.Close()

			cfg := &config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			}

			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			sdnAdapter := adapters.NewSDNAdapter(proxmoxAdapter)
			err = sdnAdapter.Apply(context.Background(), "", 0)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSDNAdapterLockContextCancellation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		errMsg string
	}{
		{
			name:   "context cancelled during retry sleep",
			errMsg: "failed to acquire SDN lock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodPost && r.URL.Path == "/cluster/sdn/lock" {
						w.WriteHeader(http.StatusServiceUnavailable)
						_, _ = w.Write([]byte(`{"errors":"lock unavailable"}`))
						return
					}
					w.WriteHeader(http.StatusNotFound)
				},
			)
			defer server.Close()

			cfg := &config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			}

			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			// Pre-cancel the context so sleepWithContext's ctx.Done() branch is hit immediately.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			sdnAdapter := adapters.NewSDNAdapter(proxmoxAdapter)
			_, err = sdnAdapter.Lock(ctx, 60*time.Second, false)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestSDNAdapterApplyTaskFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		errMsg string
	}{
		{
			name:   "task fails after apply starts",
			errMsg: "failed to wait for SDN apply task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, _ := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					w.Header().Set("Content-Type", "application/json")

					if r.Method == http.MethodPut && r.URL.Path == "/cluster/sdn" {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(
							[]byte(`{"data":"UPID:node1:00000000:00000000:00000001:sdn:undefined:root@pam:"}`),
						)
						return
					}

					// Task status polling: task stopped with a failure exit status.
					if r.Method == http.MethodGet {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(
							[]byte(
								`{"data":{"upid":"UPID:node1:00000000:00000000:00000001:sdn:undefined:root@pam:",` +
									`"status":"stopped","exitstatus":"ERROR"}}`,
							),
						)
						return
					}

					w.WriteHeader(http.StatusNotFound)
				},
			)
			defer server.Close()

			cfg := &config.Config{
				PveURL:   server.URL,
				PveUser:  "test@pam",
				PveToken: "test-token",
			}

			proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
			err := proxmoxAdapter.Connect(context.Background())
			require.NoError(t, err)

			sdnAdapter := adapters.NewSDNAdapter(proxmoxAdapter)
			err = sdnAdapter.Apply(context.Background(), "", 0)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestNewSDNAdapter(t *testing.T) {
	t.Parallel()

	t.Run("creates adapter with valid client", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			PveURL:   "https://test.proxmox.com:8006",
			PveUser:  "test@pam",
			PveToken: "test-token",
		}

		proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
		err := proxmoxAdapter.Connect(context.Background())
		require.NoError(t, err)

		sdnAdapter := adapters.NewSDNAdapter(proxmoxAdapter)
		require.NotNil(t, sdnAdapter)
	})
}
