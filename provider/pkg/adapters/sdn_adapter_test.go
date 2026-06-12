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

	t.Run("successful apply", func(t *testing.T) {
		t.Parallel()

		server, captured := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Equal(t, "/cluster/sdn", r.URL.Path)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data": null}`))
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
		require.NoError(t, err)

		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Equal(t, "/cluster/sdn", captured.Path)
	})

	t.Run("apply handles API error", func(t *testing.T) {
		t.Parallel()

		server, _ := testutils.CreateMockServer(
			t,
			func(w http.ResponseWriter, r *http.Request, _ *testutils.MockRequest) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors": "apply failed"}`))
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
		require.Error(t, err)
		assert.EqualError(t, err, "failed to apply SDN changes: 500 Internal Server Error")
	})
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
