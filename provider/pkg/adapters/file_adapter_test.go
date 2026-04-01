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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	dataStoreID = "local"
	contentType = "iso"
	fileName    = "test.iso"
	fileData    = "test data"
	pveURL      = "https://localhost:8006"
	pveUser     = "root@pam"
	pveToken    = "token"
)

// mockSSHClient is a mock implementation of the SSHClient interface.
type mockSSHClient struct {
	RunFunc func(operation proxmox.SSHOperation, path string, data ...string) (string, error)
}

// Run executes the mock SSH command.
func (m *mockSSHClient) Run(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(operation, path, data...)
	}
	return "", nil
}

func newTestFileAdapter(t *testing.T, sshClient proxmox.SSHClient) *FileAdapter {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{PveURL: server.URL, PveUser: pveUser, PveToken: pveToken}
	pxa := NewProxmoxAdapter(cfg)
	err := pxa.Connect(context.Background())
	require.NoError(t, err)

	return NewFileAdapter(
		pxa,
		func(ctx context.Context) (proxmox.SSHClient, error) {
			return sshClient, nil
		},
	)
}

func TestFileAdapterCreate(t *testing.T) {
	t.Run("create success", func(t *testing.T) {
		t.Parallel()
		sshClient := &mockSSHClient{
			RunFunc: func(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
				assert.Equal(t, proxmox.SSHOperationWrite, operation)
				expectedPath := fmt.Sprintf("/mnt/pve/%s/%s/%s", dataStoreID, contentType, fileName)
				assert.Equal(t, expectedPath, path)
				assert.Equal(t, fileData, data[0])
				return "", nil
			},
		}

		adapter := newTestFileAdapter(t, sshClient)

		inputs := proxmox.FileInputs{
			DataStoreID: dataStoreID,
			ContentType: contentType,
			SourceRaw: proxmox.FileSourceRaw{
				FileName: fileName,
				FileData: fileData,
			},
		}

		err := adapter.Create(context.Background(), inputs)
		require.NoError(t, err)
	})

	t.Run("create handles ssh client error", func(t *testing.T) {
		t.Parallel()
		adapter := NewFileAdapter(
			NewProxmoxAdapter(&config.Config{PveURL: "http://localhost", PveUser: pveUser, PveToken: pveToken}),
			func(ctx context.Context) (proxmox.SSHClient, error) {
				return nil, errors.New("ssh connection error")
			},
		)

		inputs := proxmox.FileInputs{}
		err := adapter.Create(context.Background(), inputs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error getting ssh client")
	})

	t.Run("create handles ssh write error", func(t *testing.T) {
		t.Parallel()
		sshClient := &mockSSHClient{
			RunFunc: func(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
				return "", errors.New("ssh write error")
			},
		}
		adapter := newTestFileAdapter(t, sshClient)

		inputs := proxmox.FileInputs{}
		err := adapter.Create(context.Background(), inputs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error sending data via SSH")
	})
}

func TestFileAdapterGet(t *testing.T) {
	t.Run("get success", func(t *testing.T) {
		t.Parallel()
		sshClient := &mockSSHClient{
			RunFunc: func(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
				assert.Equal(t, proxmox.SSHOperationRead, operation)
				expectedPath := fmt.Sprintf("/mnt/pve/%s/%s/%s", dataStoreID, contentType, fileName)
				assert.Equal(t, expectedPath, path)
				return fileData, nil
			},
		}

		adapter := newTestFileAdapter(t, sshClient)

		inputs := proxmox.FileInputs{
			DataStoreID: dataStoreID,
			ContentType: contentType,
			SourceRaw: proxmox.FileSourceRaw{
				FileName: fileName,
			},
		}

		file, err := adapter.Get(context.Background(), inputs)
		require.NoError(t, err)
		assert.Equal(t, dataStoreID, file.DataStoreID)
		assert.Equal(t, contentType, file.ContentType)
		assert.Equal(t, fileName, file.SourceRaw.FileName)
		assert.Equal(t, fileData, file.SourceRaw.FileData)
	})

	t.Run("get handles ssh client error", func(t *testing.T) {
		t.Parallel()
		adapter := NewFileAdapter(
			NewProxmoxAdapter(&config.Config{PveURL: "http://localhost", PveUser: pveUser, PveToken: pveToken}),
			func(ctx context.Context) (proxmox.SSHClient, error) {
				return nil, errors.New("ssh connection error")
			},
		)

		inputs := proxmox.FileInputs{}
		_, err := adapter.Get(context.Background(), inputs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error getting ssh client")
	})

	t.Run("get handles ssh read error", func(t *testing.T) {
		t.Parallel()
		sshClient := &mockSSHClient{
			RunFunc: func(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
				return "", errors.New("ssh read error")
			},
		}
		adapter := newTestFileAdapter(t, sshClient)

		inputs := proxmox.FileInputs{}
		_, err := adapter.Get(context.Background(), inputs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error reading file via SSH")
	})
}

func TestFileAdapterUpdate(t *testing.T) {
	t.Run("update success", func(t *testing.T) {
		t.Parallel()
		sshClient := &mockSSHClient{
			RunFunc: func(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
				return "", nil
			},
		}

		adapter := newTestFileAdapter(t, sshClient)

		state := proxmox.FileInputs{
			DataStoreID: dataStoreID,
			ContentType: contentType,
			SourceRaw: proxmox.FileSourceRaw{
				FileName: fileName,
				FileData: fileData,
			},
		}

		inputs := proxmox.FileInputs{
			DataStoreID: dataStoreID,
			ContentType: contentType,
			SourceRaw: proxmox.FileSourceRaw{
				FileName: "new-" + fileName,
				FileData: "new-" + fileData,
			},
		}

		err := adapter.Update(context.Background(), state, inputs)
		require.NoError(t, err)
	})

	t.Run("update handles ssh client error", func(t *testing.T) {
		t.Parallel()
		adapter := NewFileAdapter(
			NewProxmoxAdapter(&config.Config{PveURL: "http://localhost", PveUser: pveUser, PveToken: pveToken}),
			func(ctx context.Context) (proxmox.SSHClient, error) {
				return nil, errors.New("ssh connection error")
			},
		)

		err := adapter.Update(context.Background(), proxmox.FileInputs{}, proxmox.FileInputs{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error getting ssh client")
	})

	t.Run("update handles ssh delete error", func(t *testing.T) {
		t.Parallel()
		sshClient := &mockSSHClient{
			RunFunc: func(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
				if operation == proxmox.SSHOperationDelete {
					return "", errors.New("ssh delete error")
				}
				return "", nil
			},
		}
		adapter := newTestFileAdapter(t, sshClient)

		err := adapter.Update(context.Background(), proxmox.FileInputs{}, proxmox.FileInputs{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error removing file via SSH")
	})

	t.Run("update handles ssh write error", func(t *testing.T) {
		t.Parallel()
		sshClient := &mockSSHClient{
			RunFunc: func(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
				if operation == proxmox.SSHOperationWrite {
					return "", errors.New("ssh write error")
				}
				return "", nil
			},
		}
		adapter := newTestFileAdapter(t, sshClient)

		err := adapter.Update(context.Background(), proxmox.FileInputs{}, proxmox.FileInputs{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error creating file via SSH")
	})
}

func TestFileAdapter_Delete(t *testing.T) {
	t.Run("delete success", func(t *testing.T) {
		t.Parallel()
		sshClient := &mockSSHClient{
			RunFunc: func(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
				assert.Equal(t, proxmox.SSHOperationDelete, operation)
				expectedPath := fmt.Sprintf("/mnt/pve/%s/%s/%s", dataStoreID, contentType, fileName)
				assert.Equal(t, expectedPath, path)
				return "", nil
			},
		}

		adapter := newTestFileAdapter(t, sshClient)

		outputs := proxmox.FileOutputs{
			FileInputs: proxmox.FileInputs{
				DataStoreID: dataStoreID,
				ContentType: contentType,
				SourceRaw: proxmox.FileSourceRaw{
					FileName: fileName,
				},
			},
		}

		err := adapter.Delete(context.Background(), outputs)
		require.NoError(t, err)
	})

	t.Run("delete handles ssh client error", func(t *testing.T) {
		t.Parallel()
		adapter := NewFileAdapter(
			NewProxmoxAdapter(&config.Config{PveURL: "http://localhost", PveUser: pveUser, PveToken: pveToken}),
			func(ctx context.Context) (proxmox.SSHClient, error) {
				return nil, errors.New("ssh connection error")
			},
		)

		err := adapter.Delete(context.Background(), proxmox.FileOutputs{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error getting ssh client")
	})

	t.Run("delete handles ssh delete error", func(t *testing.T) {
		t.Parallel()
		sshClient := &mockSSHClient{
			RunFunc: func(operation proxmox.SSHOperation, path string, data ...string) (string, error) {
				return "", errors.New("ssh delete error")
			},
		}
		adapter := newTestFileAdapter(t, sshClient)

		err := adapter.Delete(context.Background(), proxmox.FileOutputs{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error removing file via SSH")
	})
}

func TestNewFileAdapter(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{PveURL: pveURL, PveUser: pveUser, PveToken: pveToken}
	pxa := NewProxmoxAdapter(cfg)
	err := pxa.Connect(context.Background())
	require.NoError(t, err)

	sshClientFunc := func(ctx context.Context) (proxmox.SSHClient, error) {
		return &mockSSHClient{}, nil
	}

	fileAdapter := NewFileAdapter(pxa, sshClientFunc)
	require.NotNil(t, fileAdapter)
	assert.Equal(t, pxa, fileAdapter.proxmoxAdapter)
	assert.NotNil(t, fileAdapter.sshClient)
}
