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
	"strings"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACLAdapterCreate(t *testing.T) {
	t.Parallel()

	t.Run("create user ACL success", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.ACLInputs{Path: "/", RoleID: "PVEAdmin", Type: "user", UGID: "testuser", Propagate: true}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			switch {
			case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/access/users/"):
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":{"userid":"testuser"}}`))
			case r.Method == http.MethodPut && r.URL.Path == "/access/acl":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":null}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		acl := NewACLAdapter(pxa)

		err := acl.Create(context.Background(), inputs)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Equal(t, "/access/acl", captured.Path)
	})

	t.Run("create group ACL success", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.ACLInputs{Path: "/", RoleID: "PVEAdmin", Type: "group", UGID: "testgroup", Propagate: false}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			switch {
			case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/access/groups/"):
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":{"groupid":"testgroup"}}`))
			case r.Method == http.MethodPut && r.URL.Path == "/access/acl":
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":null}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		acl := NewACLAdapter(pxa)

		err := acl.Create(context.Background(), inputs)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Equal(t, "/access/acl", captured.Path)
	})

	t.Run("create token ACL success", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.ACLInputs{
			Path:      "/pool/abc",
			RoleID:    "PVEAuditor",
			Type:      "token",
			UGID:      "user@pam!tok",
			Propagate: true,
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			if r.Method == http.MethodPut && r.URL.Path == "/access/acl" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":null}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		acl := NewACLAdapter(pxa)

		err := acl.Create(context.Background(), inputs)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Equal(t, "/access/acl", captured.Path)
	})

	t.Run("create invalid type", func(t *testing.T) {
		t.Parallel()
		inputs := proxmox.ACLInputs{Path: "/", RoleID: "PVEAdmin", Type: "invalid", UGID: "x"}
		cfg := &config.Config{PveURL: "http://example", PveUser: "u", PveToken: "t"}
		pxa := NewProxmoxAdapter(cfg)
		acl := NewACLAdapter(pxa)
		err := acl.Create(context.Background(), inputs)
		require.Error(t, err)
		assert.EqualError(t, err, ErrInvalidACLType.Error())
	})

	t.Run("create handles API error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.ACLInputs{Path: "/", RoleID: "PVEAdmin", Type: "user", UGID: "testuser"}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			switch {
			case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/access/users/"):
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":{"userid":"testuser"}}`))
			case r.Method == http.MethodPut && r.URL.Path == "/access/acl":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"data":null}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		acl := NewACLAdapter(pxa)

		err := acl.Create(context.Background(), inputs)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to create ACL resource: 500 Internal Server Error")
	})
}

func TestACLAdapterGet(t *testing.T) {
	t.Parallel()

	t.Run("get success", func(t *testing.T) {
		t.Parallel()

		aclID := "/|PVEAdmin|group|testgroup"

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			if r.Method == http.MethodGet && r.URL.Path == "/access/acl" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				payload := map[string]interface{}{
					"data": []map[string]interface{}{
						{
							"path":      "/",
							"roleid":    "PVEAdmin",
							"type":      "group",
							"ugid":      "testgroup",
							"propagate": 0,
						},
					},
				}
				_ = json.NewEncoder(w).Encode(payload)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		acl := NewACLAdapter(pxa)

		out, err := acl.Get(context.Background(), aclID)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "/", out.Path)
		assert.Equal(t, "PVEAdmin", out.RoleID)
		assert.Equal(t, "group", out.Type)
		assert.Equal(t, "testgroup", out.UGID)
		assert.False(t, out.Propagate)

		assert.Equal(t, http.MethodGet, captured.Method)
		assert.Equal(t, "/access/acl", captured.Path)
	})

	t.Run("get not found", func(t *testing.T) {
		t.Parallel()
		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			if r.Method == http.MethodGet && r.URL.Path == "/access/acl" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"data": []map[string]interface{}{},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		acl := NewACLAdapter(pxa)

		_, err := acl.Get(context.Background(), "/|PVEAdmin|user|nouser")
		require.Error(t, err)
		assert.EqualError(t, err, ErrACLNotFound.Error())
	})

	t.Run("get handles API error", func(t *testing.T) {
		t.Parallel()
		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			if r.Method == http.MethodGet && r.URL.Path == "/access/acl" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"data":null}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		acl := NewACLAdapter(pxa)

		_, err := acl.Get(context.Background(), "/|PVEAdmin|group|testgroup")
		require.Error(t, err)
		assert.EqualError(t, err, "failed to get ACLs: 500 Internal Server Error")
	})
}

func TestACLAdapterUpdate(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{PveURL: "http://example", PveUser: "u", PveToken: "t"}
	pxa := NewProxmoxAdapter(cfg)
	acl := NewACLAdapter(pxa)
	err := acl.Update(context.Background(), "/|PVEAdmin|group|testgroup", proxmox.ACLInputs{})
	require.Error(t, err)
	assert.EqualError(
		t,
		err,
		"ACL resource update is not supported, because ACLs are uniquely identified by their properties",
	)
}

func TestACLAdapterDelete(t *testing.T) {
	t.Parallel()

	t.Run("delete success", func(t *testing.T) {
		t.Parallel()
		outputs := proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{Path: "/", RoleID: "PVEAdmin", Type: "group", UGID: "testgroup", Propagate: false},
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			if r.Method == http.MethodPut && r.URL.Path == "/access/acl" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":null}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		acl := NewACLAdapter(pxa)

		err := acl.Delete(context.Background(), outputs)
		require.NoError(t, err)
		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Equal(t, "/access/acl", captured.Path)
	})

	t.Run("delete invalid type", func(t *testing.T) {
		t.Parallel()
		outputs := proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{Path: "/", RoleID: "PVEAdmin", Type: "invalid", UGID: "x"},
		}
		cfg := &config.Config{PveURL: "http://example", PveUser: "u", PveToken: "t"}
		pxa := NewProxmoxAdapter(cfg)
		acl := NewACLAdapter(pxa)
		err := acl.Delete(context.Background(), outputs)
		require.Error(t, err)
		assert.EqualError(t, err, ErrInvalidACLType.Error())
	})

	t.Run("delete handles API error", func(t *testing.T) {
		t.Parallel()
		outputs := proxmox.ACLOutputs{
			ACLInputs: proxmox.ACLInputs{Path: "/", RoleID: "PVEAdmin", Type: "user", UGID: "testuser"},
		}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			if r.Method == http.MethodPut && r.URL.Path == "/access/acl" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"data":null}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		acl := NewACLAdapter(pxa)

		err := acl.Delete(context.Background(), outputs)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to delete ACL resource: 500 Internal Server Error")
	})
}
