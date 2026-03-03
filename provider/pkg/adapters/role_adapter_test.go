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

package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	roleName       = "testrole"
	rolePrivilege1 = "VM.Allocate"
	rolePrivilege2 = "Datastore.Allocate"
)

func TestRoleAdapterCreate(t *testing.T) {
	t.Parallel()

	t.Run("create success", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.RoleInputs{
			Name:       roleName,
			Privileges: []string{rolePrivilege1, rolePrivilege2},
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/access/roles")

			var body map[string]any
			if err := json.Unmarshal([]byte(req.Body), &body); err == nil {
				assert.Equal(t, roleName, body["roleid"])
				privs, ok := body["privs"].(string)
				assert.True(t, ok)
				assert.Contains(t, privs, rolePrivilege1)
				assert.Contains(t, privs, rolePrivilege2)
			} else {
				vals, err2 := url.ParseQuery(req.Body)
				require.NoError(t, err2)
				assert.Equal(t, roleName, vals.Get("roleid"))
				privs := vals.Get("privs")
				assert.Contains(t, privs, rolePrivilege1)
				assert.Contains(t, privs, rolePrivilege2)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		role := NewRoleAdapter(pxa)

		err := role.Create(context.Background(), inputs)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, captured.Method)
		assert.Contains(t, captured.Path, "/access/roles")
		assert.Contains(t, captured.Body, roleName)
		assert.Contains(t, captured.Body, rolePrivilege1)
		assert.Contains(t, captured.Body, rolePrivilege2)
	})

	t.Run("create handles API error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.RoleInputs{
			Name:       roleName,
			Privileges: []string{rolePrivilege1},
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/access/roles")
			assert.Contains(t, req.Body, roleName)

			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		role := NewRoleAdapter(pxa)

		err := role.Create(context.Background(), inputs)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to create role: 500 Internal Server Error")
		assert.Contains(t, captured.Path, "/access/roles")
	})
}

func TestRoleAdapterGet(t *testing.T) {
	t.Parallel()

	t.Run("get success", func(t *testing.T) {
		t.Parallel()

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/access/roles/")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					rolePrivilege1: true,
					rolePrivilege2: true,
				},
			})
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		role := NewRoleAdapter(pxa)

		out, err := role.Get(context.Background(), roleName)
		require.NoError(t, err)
		require.NotNil(t, out)

		assert.Equal(t, roleName, out.Name)
		assert.ElementsMatch(t, []string{rolePrivilege1, rolePrivilege2}, out.Privileges)

		assert.Equal(t, http.MethodGet, captured.Method)
		assert.Contains(t, captured.Path, "/access/roles/")
	})

	t.Run("get returns nil privileges when empty", func(t *testing.T) {
		t.Parallel()

		server, _ := createMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *mockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{},
			})
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		role := NewRoleAdapter(pxa)

		out, err := role.Get(context.Background(), roleName)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Nil(t, out.Privileges)
	})

	t.Run("get handles API error", func(t *testing.T) {
		t.Parallel()

		server, _ := createMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *mockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		role := NewRoleAdapter(pxa)

		out, err := role.Get(context.Background(), roleName)
		require.Error(t, err)
		assert.Nil(t, out)
		assert.EqualError(t, err, "failed to get Role resource: 500 Internal Server Error")
	})
}

func TestRoleAdapterUpdate(t *testing.T) {
	t.Parallel()

	t.Run("update success", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.RoleInputs{
			Privileges: []string{rolePrivilege2, rolePrivilege1},
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/access/roles/"+url.PathEscape(roleName))

			var body map[string]any
			if err := json.Unmarshal([]byte(req.Body), &body); err == nil {
				assert.Equal(t, roleName, body["roleid"])
				privs, ok := body["privs"].(string)
				assert.True(t, ok)
				assert.Contains(t, privs, rolePrivilege1)
				assert.Contains(t, privs, rolePrivilege2)
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		role := NewRoleAdapter(pxa)

		err := role.Update(context.Background(), roleName, inputs)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Contains(t, captured.Path, "/access/roles/")
		assert.Contains(t, captured.Body, rolePrivilege1)
		assert.Contains(t, captured.Body, rolePrivilege2)
	})

	t.Run("update handles API error", func(t *testing.T) {
		t.Parallel()

		server, _ := createMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *mockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		role := NewRoleAdapter(pxa)

		err := role.Update(context.Background(), roleName, proxmox.RoleInputs{Privileges: []string{rolePrivilege1}})
		require.Error(t, err)
		assert.EqualError(t, err, "failed to update role: 500 Internal Server Error")
	})
}

func TestRoleAdapterDelete(t *testing.T) {
	t.Parallel()

	t.Run("delete success", func(t *testing.T) {
		t.Parallel()

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Contains(t, r.URL.Path, "/access/roles/")
			assert.Empty(t, req.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		role := NewRoleAdapter(pxa)

		err := role.Delete(context.Background(), roleName)
		require.NoError(t, err)
		assert.Equal(t, http.MethodDelete, captured.Method)
		assert.Contains(t, captured.Path, "/access/roles/")
	})

	t.Run("delete handles API error", func(t *testing.T) {
		t.Parallel()

		server, _ := createMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *mockRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		role := NewRoleAdapter(pxa)

		err := role.Delete(context.Background(), roleName)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to delete role testrole: 500 Internal Server Error")
	})
}

func TestNewRoleAdapter(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{PveURL: "https://test.proxmox.com:8006", PveUser: "test@pam", PveToken: "test-token"}
	pxa := NewProxmoxAdapter(cfg)
	require.NoError(t, pxa.Connect(context.Background()))

	role := NewRoleAdapter(pxa)
	require.NotNil(t, role)
	assert.NotNil(t, role.proxmoxAdapter)
}
