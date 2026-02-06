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
	"strings"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userID        = "testuser@pve"
	userEmail     = "user@example.com"
	userPassword  = "super-secret"
	userFirstname = "Test"
	userLastname  = "User"

	userGroup1 = "group-a"
	userGroup2 = "group-b"

	userKey1 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKey1"
	userKey2 = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQKey2"
)

func TestUserAdapterCreate(t *testing.T) {
	t.Parallel()

	t.Run("create success", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.UserInputs{
			Name:      userID,
			Comment:   "hello",
			Email:     userEmail,
			Enable:    true,
			Expire:    0,
			Firstname: userFirstname,
			Lastname:  userLastname,
			Groups:    []string{userGroup1, userGroup2},
			Keys:      []string{userKey1, userKey2},
			Password:  userPassword,
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/access/users")

			var body map[string]any
			if err := json.Unmarshal([]byte(req.Body), &body); err == nil {
				assert.Equal(t, userID, body["userid"])
				assert.Equal(t, "hello", body["comment"])
				assert.Equal(t, userEmail, body["email"])
				assert.Equal(t, userFirstname, body["firstname"])
				assert.Equal(t, userLastname, body["lastname"])
				assert.Equal(t, userPassword, body["password"])

				groups, ok := body["groups"].([]any)
				require.True(t, ok, "expected groups to be an array")
				assert.ElementsMatch(t, []any{userGroup1, userGroup2}, groups)

				keys, ok := body["keys"].([]any)
				require.True(t, ok, "expected keys to be an array")
				assert.ElementsMatch(t, []any{userKey1, userKey2}, keys)
			} else {
				vals, err2 := url.ParseQuery(req.Body)
				require.NoError(t, err2)
				assert.Equal(t, userID, vals.Get("userid"))
				assert.Equal(t, "hello", vals.Get("comment"))
				assert.Equal(t, userEmail, vals.Get("email"))
				assert.Equal(t, userFirstname, vals.Get("firstname"))
				assert.Equal(t, userLastname, vals.Get("lastname"))
				assert.Equal(t, userPassword, vals.Get("password"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		user := NewUserAdapter(pxa)

		err := user.Create(context.Background(), inputs)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, captured.Method)
		assert.Contains(t, captured.Path, "/access/users")
		assert.Contains(t, captured.Body, userID)
		assert.Contains(t, captured.Body, userEmail)
	})

	t.Run("create omits keys when empty", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.UserInputs{
			Name:     userID,
			Enable:   true,
			Keys:     []string{},
			Password: userPassword,
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/access/users")

			// With keys == nil, json omitempty should omit the field.
			if strings.Contains(req.Body, "\"keys\"") {
				t.Fatalf("expected request body to omit keys, got: %s", req.Body)
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		user := NewUserAdapter(pxa)

		err := user.Create(context.Background(), inputs)
		require.NoError(t, err)
		assert.Contains(t, captured.Path, "/access/users")
	})

	t.Run("create handles API error", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.UserInputs{Name: userID, Enable: true, Password: userPassword}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/access/users")
			assert.Contains(t, req.Body, userID)

			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		user := NewUserAdapter(pxa)

		err := user.Create(context.Background(), inputs)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to create user "+userID+": 500 Internal Server Error")
		assert.Contains(t, captured.Path, "/access/users")
	})
}

func TestUserAdapterGet(t *testing.T) {
	t.Parallel()

	t.Run("get success", func(t *testing.T) {
		t.Parallel()

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, _ *mockRequest) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Contains(t, r.URL.Path, "/access/users/")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"userid":    userID,
					"comment":   "hello",
					"email":     userEmail,
					"enable":    1,
					"expire":    0,
					"firstname": userFirstname,
					"lastname":  userLastname,
					"groups":    []string{userGroup1, userGroup2},
					"keys":      userKey2 + "," + userKey1,
				},
			})
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		user := NewUserAdapter(pxa)

		out, err := user.Get(context.Background(), userID)
		require.NoError(t, err)
		require.NotNil(t, out)

		assert.Equal(t, userID, out.Name)
		assert.Equal(t, "hello", out.Comment)
		assert.Equal(t, userEmail, out.Email)
		assert.True(t, out.Enable)
		assert.Equal(t, userFirstname, out.Firstname)
		assert.Equal(t, userLastname, out.Lastname)
		assert.ElementsMatch(t, []string{userGroup1, userGroup2}, out.Groups)
		// StringToSlice sorts
		assert.Equal(t, []string{userKey1, userKey2}, out.Keys)

		assert.Equal(t, http.MethodGet, captured.Method)
		assert.Contains(t, captured.Path, "/access/users/")
	})

	t.Run("get returns nil keys when empty", func(t *testing.T) {
		t.Parallel()

		server, _ := createMockServer(t, func(w http.ResponseWriter, _ *http.Request, _ *mockRequest) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"userid": userID,
					"enable": 1,
					"keys":   "",
				},
			})
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		user := NewUserAdapter(pxa)

		out, err := user.Get(context.Background(), userID)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Nil(t, out.Keys)
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
		user := NewUserAdapter(pxa)

		out, err := user.Get(context.Background(), userID)
		require.Error(t, err)
		assert.Nil(t, out)
		assert.EqualError(t, err, "failed to get User resource: 500 Internal Server Error")
	})
}

func TestUserAdapterUpdate(t *testing.T) {
	t.Parallel()

	t.Run("update success", func(t *testing.T) {
		t.Parallel()

		inputs := proxmox.UserInputs{
			Comment:   "updated",
			Email:     userEmail,
			Enable:    false,
			Expire:    123,
			Firstname: userFirstname,
			Lastname:  userLastname,
			Groups:    []string{userGroup1},
			Keys:      []string{userKey2, userKey1},
		}

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/access/users/"+url.PathEscape(userID))

			var body map[string]any
			if err := json.Unmarshal([]byte(req.Body), &body); err == nil {
				assert.Equal(t, userID, body["userid"])
				assert.Equal(t, "updated", body["comment"])
				assert.Equal(t, userEmail, body["email"])
				assert.Equal(t, float64(123), body["expire"])
				assert.Equal(t, userFirstname, body["firstname"])
				assert.Equal(t, userLastname, body["lastname"])

				// keys is a string (comma-separated) in api.User
				if keys, ok := body["keys"].(string); ok {
					assert.Equal(t, userKey1+","+userKey2, keys)
				}

				// enable could encode as bool or number depending on IntOrBool
				switch v := body["enable"].(type) {
				case bool:
					assert.False(t, v)
				case float64:
					assert.Equal(t, float64(0), v)
				case string:
					assert.Equal(t, "0", v)
				}
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		user := NewUserAdapter(pxa)

		err := user.Update(context.Background(), userID, inputs)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPut, captured.Method)
		assert.Contains(t, captured.Path, "/access/users/")
		assert.Contains(t, captured.Body, "updated")
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
		user := NewUserAdapter(pxa)

		err := user.Update(context.Background(), userID, proxmox.UserInputs{Enable: true})
		require.Error(t, err)
		assert.EqualError(t, err, "failed to update user "+userID+": 500 Internal Server Error")
	})
}

func TestUserAdapterDelete(t *testing.T) {
	t.Parallel()

	t.Run("delete success", func(t *testing.T) {
		t.Parallel()

		server, captured := createMockServer(t, func(w http.ResponseWriter, r *http.Request, req *mockRequest) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Contains(t, r.URL.Path, "/access/users/")
			assert.Empty(t, req.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": null}`))
		})
		defer server.Close()

		cfg := &config.Config{PveURL: server.URL, PveUser: "test@pam", PveToken: "token"}
		pxa := NewProxmoxAdapter(cfg)
		require.NoError(t, pxa.Connect(context.Background()))
		user := NewUserAdapter(pxa)

		err := user.Delete(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, http.MethodDelete, captured.Method)
		assert.Contains(t, captured.Path, "/access/users/")
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
		user := NewUserAdapter(pxa)

		err := user.Delete(context.Background(), userID)
		require.Error(t, err)
		assert.EqualError(t, err, "failed to delete user "+userID+": 500 Internal Server Error")
	})
}

func TestNewUserAdapter(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{PveURL: "https://test.proxmox.com:8006", PveUser: "test@pam", PveToken: "test-token"}
	pxa := NewProxmoxAdapter(cfg)
	require.NoError(t, pxa.Connect(context.Background()))

	user := NewUserAdapter(pxa)
	require.NotNil(t, user)
	assert.NotNil(t, user.proxmoxAdapter)
}
