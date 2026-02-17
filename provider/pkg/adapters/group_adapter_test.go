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
	groupID          = "testgroup"
	groupDescription = "This is a test group"
)

func TestGroupAdapterCreate(t *testing.T) {
	t.Run("create success", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.GroupInputs{
			Name:    groupID,
			Comment: groupDescription,
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/access/groups")

			var body map[string]any
			if err := json.Unmarshal([]byte(req.Body), &body); err == nil {
				assert.Equal(t, groupID, body["groupid"])
				assert.Equal(t, groupDescription, body["comment"])
			} else {
				formValues, parseErr := url.ParseQuery(req.Body)
				require.NoError(t, parseErr, "body is not valid JSON or form data")
				assert.Equal(t, groupID, formValues.Get("groupid"))
				assert.Equal(t, groupDescription, formValues.Get("description"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		group := NewGroupAdapter(pxa)

		err := group.Create(context.Background(), inputs)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, captured.Method)
		assert.Contains(t, captured.Path, "/access/groups")
	})

	t.Run("create handles API error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.GroupInputs{
			Name:    groupID,
			Comment: groupDescription,
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/access/groups")
			assert.Contains(t, req.Body, groupID)

			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		group := NewGroupAdapter(pxa)

		err := group.Create(context.Background(), inputs)
		require.Error(t, err)
		assert.Contains(t, captured.Path, "/access/groups")
	})
}

func TestGroupAdapterGet(t *testing.T) {
	t.Parallel()

	t.Run("get success", func(t *testing.T) {
		t.Parallel()

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/access/groups/")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"groupid": groupID,
					"comment": groupDescription,
				},
			})
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		group := NewGroupAdapter(pxa)

		outputs, err := group.Get(context.Background(), groupID)
		require.NoError(t, err)
		require.NotNil(t, outputs)

		assert.Equal(t, groupID, outputs.Name)
		assert.Equal(t, groupDescription, outputs.Comment)

		assert.Equal(t, http.MethodGet, captured.Method)
		assert.Contains(t, captured.Path, "/access/groups/")
	})

	t.Run("get handles API error", func(t *testing.T) {
		t.Parallel()

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		group := NewGroupAdapter(pxa)

		outputs, err := group.Get(context.Background(), groupID)
		require.Error(t, err)
		assert.Nil(t, outputs)
	})
}

func TestGroupAdapterUpdate(t *testing.T) {
	t.Parallel()

	t.Run("update success", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.GroupInputs{
			Name:    groupID,
			Comment: groupDescription,
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/access/groups/"+url.PathEscape(groupID))

			var body map[string]any
			if err := json.Unmarshal([]byte(req.Body), &body); err == nil {
				assert.Equal(t, groupID, body["groupid"])
				assert.Equal(t, groupDescription, body["comment"])
			} else {
				formValues, parseErr := url.ParseQuery(req.Body)
				require.NoError(t, parseErr, "body is not valid JSON or form data")
				assert.Equal(t, groupDescription, formValues.Get("description"))
				assert.Equal(t, groupID, formValues.Get("groupid"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		group := NewGroupAdapter(pxa)

		err := group.Update(context.Background(), groupID, inputs)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Contains(t, captured.Path, "/access/groups/"+url.PathEscape(groupID))
	})

	t.Run("update handles API error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.GroupInputs{
			Name:    groupID,
			Comment: groupDescription,
		}

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/access/groups/"+url.PathEscape(groupID))

			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		group := NewGroupAdapter(pxa)

		err := group.Update(context.Background(), groupID, inputs)
		require.Error(t, err)
	})
}

func TestGroupAdapterDelete(t *testing.T) {
	t.Parallel()

	t.Run("delete success", func(t *testing.T) {
		t.Parallel()

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Contains(t, r.URL.Path, "/access/groups/"+url.PathEscape(groupID))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		group := NewGroupAdapter(pxa)

		err := group.Delete(context.Background(), groupID)
		require.NoError(t, err)

		assert.Equal(t, http.MethodDelete, captured.Method)
		assert.Contains(t, captured.Path, "/access/groups/")
	})

	t.Run("delete handles API error", func(t *testing.T) {
		t.Parallel()

		server, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Contains(t, r.URL.Path, "/access/groups/")

			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		group := NewGroupAdapter(pxa)

		err := group.Delete(context.Background(), groupID)
		require.Error(t, err)
	})
}

func TestNewGroupAdapter(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{PveURL: "http://example.com", PveUser: "test@pam", PveToken: "token"}
	pxa := NewProxmoxAdapter(cfg)
	require.NoError(t, pxa.Connect(context.Background()))

	groupAdapter := NewGroupAdapter(pxa)
	require.NotNil(t, groupAdapter)
	assert.NotNil(t, groupAdapter)
}
