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

package file_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	fileResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/file"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

type mockFileOperations struct {
	createFunc func(ctx context.Context, inputs proxmox.FileInputs) error
	getFunc    func(ctx context.Context, inputs proxmox.FileInputs) (*proxmox.FileOutputs, error)
	deleteFunc func(ctx context.Context, outputs proxmox.FileOutputs) error
}

func (m *mockFileOperations) Create(ctx context.Context, inputs proxmox.FileInputs) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, inputs)
	}
	return nil
}

func (m *mockFileOperations) Get(
	ctx context.Context,
	inputs proxmox.FileInputs,
) (*proxmox.FileOutputs, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, inputs)
	}
	return &proxmox.FileOutputs{FileInputs: inputs}, nil
}

func (m *mockFileOperations) Delete(ctx context.Context, outputs proxmox.FileOutputs) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, outputs)
	}
	return nil
}

func TestFileOperationsNotConfigured(t *testing.T) {
	t.Parallel()
	file := &fileResource.File{}
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		t.Parallel()
		_, err := file.Create(ctx, infer.CreateRequest[proxmox.FileInputs]{})
		assert.EqualError(t, err, "FileOperations not configured")
	})

	t.Run("Read", func(t *testing.T) {
		t.Parallel()
		_, err := file.Read(ctx, infer.ReadRequest[proxmox.FileInputs, proxmox.FileOutputs]{})
		assert.EqualError(t, err, "FileOperations not configured")
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		_, err := file.Delete(ctx, infer.DeleteRequest[proxmox.FileOutputs]{})
		assert.EqualError(t, err, "FileOperations not configured")
	})
}

func TestFileCreate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	inputs := proxmox.FileInputs{
		DataStoreID: "local",
		ContentType: "iso",
		SourceRaw: proxmox.FileSourceRaw{
			FileName: "test.iso",
			FileData: "test data",
		},
	}

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		called := false
		file := &fileResource.File{
			FileOps: &mockFileOperations{
				createFunc: func(ctx context.Context, i proxmox.FileInputs) error {
					called = true
					assert.Equal(t, inputs, i)
					return nil
				},
			},
		}

		resp, err := file.Create(ctx, infer.CreateRequest[proxmox.FileInputs]{Name: "test", Inputs: inputs})
		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, "test", resp.ID)
		assert.Equal(t, inputs, resp.Output.FileInputs)
	})

	t.Run("DryRun", func(t *testing.T) {
		t.Parallel()
		called := false
		file := &fileResource.File{
			FileOps: &mockFileOperations{
				createFunc: func(ctx context.Context, i proxmox.FileInputs) error {
					called = true
					return nil
				},
			},
		}

		resp, err := file.Create(
			ctx,
			infer.CreateRequest[proxmox.FileInputs]{Name: "test", Inputs: inputs, DryRun: true},
		)
		require.NoError(t, err)
		assert.False(t, called)
		assert.Equal(t, "test", resp.ID)
	})

	t.Run("AdapterError", func(t *testing.T) {
		t.Parallel()
		file := &fileResource.File{
			FileOps: &mockFileOperations{
				createFunc: func(ctx context.Context, i proxmox.FileInputs) error {
					return errors.New("adapter error")
				},
			},
		}

		_, err := file.Create(ctx, infer.CreateRequest[proxmox.FileInputs]{Name: "test", Inputs: inputs})
		require.Error(t, err)
		assert.EqualError(t, err, "adapter error")
	})
}

func TestFileRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	inputs := proxmox.FileInputs{
		DataStoreID: "local",
		ContentType: "iso",
		SourceRaw: proxmox.FileSourceRaw{
			FileName: "test.iso",
			FileData: "test data",
		},
	}
	outputs := proxmox.FileOutputs{FileInputs: inputs}

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		called := false
		file := &fileResource.File{
			FileOps: &mockFileOperations{
				getFunc: func(ctx context.Context, i proxmox.FileInputs) (*proxmox.FileOutputs, error) {
					called = true
					assert.Equal(t, inputs, i)
					return &outputs, nil
				},
			},
		}

		resp, err := file.Read(
			ctx,
			infer.ReadRequest[proxmox.FileInputs, proxmox.FileOutputs]{ID: "test", Inputs: inputs},
		)
		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, "test", resp.ID)
		assert.Equal(t, outputs, resp.State)
	})

	t.Run("IDNotFound", func(t *testing.T) {
		t.Parallel()
		called := false
		file := &fileResource.File{
			FileOps: &mockFileOperations{
				getFunc: func(ctx context.Context, i proxmox.FileInputs) (*proxmox.FileOutputs, error) {
					called = true
					return nil, nil
				},
			},
		}

		resp, err := file.Read(
			ctx,
			infer.ReadRequest[proxmox.FileInputs, proxmox.FileOutputs]{ID: "", Inputs: inputs},
		)
		require.NoError(t, err)
		assert.False(t, called)
		assert.Equal(t, "", resp.ID)
	})

	t.Run("AdapterError", func(t *testing.T) {
		t.Parallel()
		file := &fileResource.File{
			FileOps: &mockFileOperations{
				getFunc: func(ctx context.Context, i proxmox.FileInputs) (*proxmox.FileOutputs, error) {
					return nil, errors.New("adapter error")
				},
			},
		}

		_, err := file.Read(
			ctx,
			infer.ReadRequest[proxmox.FileInputs, proxmox.FileOutputs]{ID: "test", Inputs: inputs},
		)
		require.Error(t, err)
		assert.EqualError(t, err, "adapter error")
	})
}

func TestFileDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	outputs := proxmox.FileOutputs{
		FileInputs: proxmox.FileInputs{
			DataStoreID: "local",
			ContentType: "iso",
			SourceRaw: proxmox.FileSourceRaw{
				FileName: "test.iso",
				FileData: "test data",
			},
		},
	}

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		called := false
		file := &fileResource.File{
			FileOps: &mockFileOperations{
				deleteFunc: func(ctx context.Context, o proxmox.FileOutputs) error {
					called = true
					assert.Equal(t, outputs, o)
					return nil
				},
			},
		}

		_, err := file.Delete(ctx, infer.DeleteRequest[proxmox.FileOutputs]{State: outputs})
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("AdapterError", func(t *testing.T) {
		t.Parallel()
		file := &fileResource.File{
			FileOps: &mockFileOperations{
				deleteFunc: func(ctx context.Context, o proxmox.FileOutputs) error {
					return errors.New("adapter error")
				},
			},
		}

		_, err := file.Delete(ctx, infer.DeleteRequest[proxmox.FileOutputs]{State: outputs})
		require.Error(t, err)
		assert.EqualError(t, err, "adapter error")
	})
}

func TestFileDiff(t *testing.T) {
	t.Parallel()

	baseInputs := proxmox.FileInputs{
		DataStoreID: "local",
		ContentType: "snippets",
		SourceRaw: proxmox.FileSourceRaw{
			FileName: "test.yaml",
			FileData: "data",
		},
	}
	baseState := proxmox.FileOutputs{FileInputs: baseInputs}

	tests := []struct {
		name             string
		inputs           proxmox.FileInputs
		state            proxmox.FileOutputs
		wantChanges      bool
		wantKeys         []string
		wantDeleteBefore bool
	}{
		{
			name:             "no changes",
			inputs:           baseInputs,
			state:            baseState,
			wantChanges:      false,
			wantDeleteBefore: true,
		},
		{
			name: "datastoreId changed triggers replace",
			inputs: proxmox.FileInputs{
				DataStoreID: "ceph",
				ContentType: baseInputs.ContentType,
				SourceRaw:   baseInputs.SourceRaw,
			},
			state:            baseState,
			wantChanges:      true,
			wantKeys:         []string{"datastoreId"},
			wantDeleteBefore: true,
		},
		{
			name: "contentType changed triggers replace",
			inputs: proxmox.FileInputs{
				DataStoreID: baseInputs.DataStoreID,
				ContentType: "iso",
				SourceRaw:   baseInputs.SourceRaw,
			},
			state:            baseState,
			wantChanges:      true,
			wantKeys:         []string{"contentType"},
			wantDeleteBefore: true,
		},
		{
			name: "sourceRaw fileData changed triggers replace",
			inputs: proxmox.FileInputs{
				DataStoreID: baseInputs.DataStoreID,
				ContentType: baseInputs.ContentType,
				SourceRaw: proxmox.FileSourceRaw{
					FileName: baseInputs.SourceRaw.FileName,
					FileData: "new-data",
				},
			},
			state:            baseState,
			wantChanges:      true,
			wantKeys:         []string{"sourceRaw"},
			wantDeleteBefore: true,
		},
		{
			name: "sourceRaw fileName changed triggers replace",
			inputs: proxmox.FileInputs{
				DataStoreID: baseInputs.DataStoreID,
				ContentType: baseInputs.ContentType,
				SourceRaw: proxmox.FileSourceRaw{
					FileName: "renamed.yaml",
					FileData: baseInputs.SourceRaw.FileData,
				},
			},
			state:            baseState,
			wantChanges:      true,
			wantKeys:         []string{"sourceRaw"},
			wantDeleteBefore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := &fileResource.File{}
			resp, err := f.Diff(context.Background(), infer.DiffRequest[proxmox.FileInputs, proxmox.FileOutputs]{
				ID:     "test-file",
				Inputs: tt.inputs,
				State:  tt.state,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.wantChanges, resp.HasChanges)
			assert.Equal(t, tt.wantDeleteBefore, resp.DeleteBeforeReplace)
			for _, key := range tt.wantKeys {
				assert.Contains(t, resp.DetailedDiff, key)
				assert.Equal(t, p.UpdateReplace, resp.DetailedDiff[key].Kind)
			}
		})
	}
}
