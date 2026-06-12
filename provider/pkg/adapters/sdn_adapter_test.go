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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

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
			name:       "successful apply returns UPID",
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

			server, captured := testutils.CreateMockServer(
				t,
				func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
					assert.Equal(t, http.MethodPut, r.Method)
					assert.Equal(t, "/cluster/sdn", r.URL.Path)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.statusCode)
					_, _ = w.Write([]byte(tt.response))
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
			err = sdnAdapter.Apply(context.Background())

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, http.MethodPut, captured.Method)
			assert.Equal(t, "/cluster/sdn", captured.Path)
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
